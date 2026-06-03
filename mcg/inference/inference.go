// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package inference

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"sdv.googlesource.com/mcg/mcg/expressions"
	"sdv.googlesource.com/mcg/mcg/type_resolvers"
	"sdv.googlesource.com/mcg/mcg/validators"

	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

const AdhocPackage string = "aaos.sdv.telemetry.adhoc"

func Infer(
	config *pb.MetricsConfig,
	typeResolver type_resolvers.EnrichedTypeResolver,
	dataSourceMessageTypes map[string]string,
) []error {
	sourcesByName := map[string]*pb.Source{}
	for _, source := range config.GetSources() {
		sourcesByName[source.GetName()] = source
	}

	var adhocDescriptor *descriptorpb.FileDescriptorProto
	for _, fileDescriptor := range config.GetDescriptorProtos() {
		if fileDescriptor.GetPackage() == AdhocPackage {
			adhocDescriptor = fileDescriptor
			break
		}
	}
	if adhocDescriptor == nil {
		adhocDescriptor = new(descriptorpb.FileDescriptorProto)
		adhocDescriptor.Syntax = proto.String("proto2") // Do not use proto3 here: b/380905512
		adhocDescriptor.Name = proto.String("adhoc.proto")
		adhocDescriptor.Package = proto.String(AdhocPackage)
	}

	ti := &typeInference{
		config:                 config,
		typeResolver:           typeResolver,
		dataSourceMessageTypes: dataSourceMessageTypes,
		sourcesByName:          sourcesByName,
		adhocDescriptor:        adhocDescriptor,
		referencedEnums:        make(map[protoreflect.FullName]protoreflect.EnumDescriptor),
		referencedMessages:     make(map[protoreflect.FullName]protoreflect.MessageDescriptor),
	}
	ti.expressionResolver = NewExpressionResolver(
		config.GetExpressionNodes(),
		ti.getSourceMessageName,
		ti.getDescriptorProto,
	)
	return ti.infer()
}

type typeInference struct {
	config                 *pb.MetricsConfig
	typeResolver           type_resolvers.EnrichedTypeResolver
	dataSourceMessageTypes map[string]string
	sourcesByName          map[string]*pb.Source
	expressionResolver     *ExpressionResolver
	// The adhoc descriptor contains generated message descriptors describing
	// aggregators and report generators.
	adhocDescriptor    *descriptorpb.FileDescriptorProto
	referencedEnums    map[protoreflect.FullName]protoreflect.EnumDescriptor
	referencedMessages map[protoreflect.FullName]protoreflect.MessageDescriptor
}

func (ti *typeInference) infer() []error {
	var errs []error

	// Topologically sorting sources ensures that we infer the message types of an aggregator's
	// dependencies before we try to infer the aggregator itself.
	sortedSources, err := ti.topologicallySortedSources()
	if err != nil {
		// It does not make sense to continue if this fails, since all following errors may be
		// caused by us not being able to topologically sort sources.
		return []error{fmt.Errorf("Cyclic dependency between sources and/or triggers detected: %w", err)}
	}

	for _, source := range sortedSources {
		if agg := source.GetAggregator(); agg != nil {
			if err := ti.processMessageBuilder(agg.GetMessageBuilder(), source.GetName(), "aggregator"); err != nil {
				errs = append(errs, err)
			}
		}
	}

	for _, reportConfig := range ti.config.GetMetricsReportConfigs() {
		if err := ti.processMessageBuilder(reportConfig.GetMessageBuilder(), reportConfig.GetName(), "metrics report"); err != nil {
			errs = append(errs, err)
		}
	}

	if validationErrs := ti.validateTriggerTypes(); len(validationErrs) > 0 {
		errs = append(errs, validationErrs...)
	}

	fds, err := TreeShake(ti.referencedMessages, ti.referencedEnums)
	if err != nil {
		errs = append(errs, err)
	}

	// Append the adhoc descriptor if it is non-empty.
	if len(ti.adhocDescriptor.GetMessageType()) > 0 || len(ti.adhocDescriptor.GetEnumType()) > 0 || len(ti.adhocDescriptor.GetDependency()) > 0 {
		// Remove any duplicate dependencies in the adhoc descriptor.
		slices.Sort(ti.adhocDescriptor.Dependency)
		ti.adhocDescriptor.Dependency = slices.Compact(ti.adhocDescriptor.Dependency)

		fds = append(fds, ti.adhocDescriptor)
	}

	ti.config.SetDescriptorProtos(fds)

	return errs
}

func (ti *typeInference) getSourceMessageName(sourceName string) (protoreflect.FullName, error) {
	source, ok := ti.sourcesByName[sourceName]
	if !ok {
		return "", fmt.Errorf("Source %q does not exist.", sourceName)
	}

	if dataSource := source.GetDataSource(); dataSource != nil {
		if msgName, ok := ti.dataSourceMessageTypes[dataSource.GetSourceIdentifier()]; ok {
			return protoreflect.FullName(strings.TrimPrefix(msgName, ".")), nil
		}
		return protoreflect.FullName(strings.TrimPrefix(dataSource.GetSourceIdentifier(), ".")), nil
	} else if aggregator := source.GetAggregator(); aggregator != nil {
		msgTypeName := aggregator.GetMessageBuilder().GetMessageType()
		if msgTypeName == "" {
			// This should never happen, since the topological sort we do on
			// sources should ensure that we only resolve expressions referring
			// to already resolved aggregators.
			return "", fmt.Errorf("Aggregator schema for %q not inferred yet. This is a bug, please report it.", sourceName)
		}
		return protoreflect.FullName(strings.TrimPrefix(msgTypeName, ".")), nil
	}
	return "", fmt.Errorf("Source %q has unknown source type", sourceName)
}

func (ti *typeInference) topologicallySortedSources() ([]*pb.Source, error) {
	g := validators.NewGraphForInferenceCycleChecks(ti.config)
	orderedSourceNames, err := g.StableReverseTopologicalOrdering()
	if err != nil {
		return nil, err
	}
	// nameToOrderIdx will contain the index in the topological sort order of each source.
	nameToOrderIdx := make(map[string]int)
	for idx, sourceName := range orderedSourceNames {
		nameToOrderIdx[sourceName] = idx
	}

	sources := slices.Clone(ti.config.GetSources())
	slices.SortFunc(sources, func(a, b *pb.Source) int {
		return nameToOrderIdx[a.GetName()] - nameToOrderIdx[b.GetName()]
	})
	return sources, nil
}

func (ti *typeInference) processMessageBuilder(msgBuilder *pb.ProtoMessageBuilder, name string, entityType string) error {
	if msgBuilder.GetMessageType() == "" {
		if err := ti.generateAndRegisterAdhocSchema(msgBuilder, name); err != nil {
			return fmt.Errorf("Failed to generate ad-hoc schema for %s %q: %w", entityType, name, err)
		}
	} else {
		if err := ti.validatePredefinedMessageType(msgBuilder); err != nil {
			return fmt.Errorf("Invalid predefined message type for %s %q: %w", entityType, name, err)
		}
	}
	return nil
}

// generateAndRegisterAdhocSchema creates a synthetic DescriptorProto for an inline MessageBuilder.
func (ti *typeInference) generateAndRegisterAdhocSchema(msgBuilder *pb.ProtoMessageBuilder, parentName string) error {
	name := fmt.Sprintf(".%s.%s", AdhocPackage, parentName)

	// Check if a schema with this name was already generated or provided (to avoid redundant generation)
	if ti.IsTypeInOutputDescriptors(protoreflect.FullName(strings.TrimPrefix(name, "."))) {
		return nil
	}

	// Set the message name to avoid infinite recursion.
	msgBuilder.SetMessageType(name)

	descriptor := &descriptorpb.DescriptorProto{}
	descriptor.Name = &parentName

	var errs []error
	for i, field_assignment := range msgBuilder.GetFieldAssignments() {
		// Create Field
		fieldDesc, err := ti.getFieldDescriptorFromAssignment(field_assignment, int32(i+1))
		if err != nil {
			errs = append(errs, err)
		}
		descriptor.Field = append(descriptor.Field, fieldDesc)
	}
	err := errors.Join(errs...)
	if err != nil {
		// Clean up placeholder on failure
		msgBuilder.SetMessageType("")
		return err
	}

	// Deduplicate: reuse existing identical adhoc descriptor if found, otherwise register the new schema.
	finalName := ti.AddOrDeduplicateAdhocMessage(descriptor)
	msgBuilder.SetMessageType(finalName)

	return nil
}

func (ti *typeInference) validatePredefinedMessageType(msgBuilder *pb.ProtoMessageBuilder) error {
	msgTypeName := msgBuilder.GetMessageType()
	fullName := protoreflect.FullName(strings.TrimPrefix(msgTypeName, "."))

	if !fullName.IsValid() {
		return fmt.Errorf("predefined message type %q is not a qualified protobuf name", msgTypeName)
	}

	msgType, err := ti.typeResolver.FindMessageByName(fullName)
	if err != nil {
		return fmt.Errorf("No definition found for message type %q.", msgTypeName)
	}

	if !ti.IsTypeInOutputDescriptors(msgType.Descriptor().FullName()) {
		ti.referencedMessages[msgType.Descriptor().FullName()] = msgType.Descriptor()
	}
	return nil
}

// IsTypeInOutputDescriptors checks if a message type is present in the synthetic adhoc file
// or is already marked as referenced.
func (ti *typeInference) IsTypeInOutputDescriptors(messageName protoreflect.FullName) bool {
	if strings.HasPrefix(string(messageName), "google.protobuf.") {
		return true
	}
	if ti.findAdhocMessage(messageName) != nil {
		return true
	}
	if _, ok := ti.referencedMessages[messageName]; ok {
		return true
	}
	return false
}

func (ti *typeInference) getDescriptorProto(name protoreflect.FullName) *descriptorpb.DescriptorProto {
	if msg := ti.findAdhocMessage(name); msg != nil {
		return msg
	}
	if msg, err := ti.typeResolver.FindMessageByName(name); err == nil {
		return protodesc.ToDescriptorProto(msg.Descriptor())
	}
	return nil
}

// AddOrDeduplicateAdhocMessage checks if an identical adhoc message descriptor already exists.
// If so, it returns the duplicate's qualified name. Otherwise, it commits and registers
// the new descriptor inside the synthetic adhoc.proto and returns its qualified name.
func (ti *typeInference) AddOrDeduplicateAdhocMessage(desc *descriptorpb.DescriptorProto) string {
	if identical := ti.findIdenticalAdhocMessageIgnoringName(desc); identical != nil {
		return fmt.Sprintf(".%s.%s", AdhocPackage, identical.GetName())
	}
	ti.adhocDescriptor.MessageType = append(ti.adhocDescriptor.MessageType, desc)
	return fmt.Sprintf(".%s.%s", AdhocPackage, desc.GetName())
}

// findAdhocMessage retrieves an adhoc message descriptor by name.
func (ti *typeInference) findAdhocMessage(name protoreflect.FullName) *descriptorpb.DescriptorProto {
	if pkg := string(name.Parent()); pkg != AdhocPackage {
		return nil
	}

	typeStr := string(name.Name())
	for _, msgDescriptor := range ti.adhocDescriptor.GetMessageType() {
		if msgDescriptor.GetName() == typeStr {
			return msgDescriptor
		}
	}
	return nil
}

// findIdenticalAdhocMessageIgnoringName searches for an existing identical
// adhoc message descriptor, but ignores the name of the message.
func (ti *typeInference) findIdenticalAdhocMessageIgnoringName(desc *descriptorpb.DescriptorProto) *descriptorpb.DescriptorProto {
	for _, msg := range ti.adhocDescriptor.GetMessageType() {
		if func() bool {
			originalNamePtr := desc.Name
			desc.Name = msg.Name
			defer func() { desc.Name = originalNamePtr }()

			return proto.Equal(msg, desc)
		}() {
			return msg
		}
	}
	return nil
}

func (ti *typeInference) getFieldDescriptorFromAssignment(fieldAssignment *pb.ProtoMessageBuilder_FieldAssignment, pos int32) (*descriptorpb.FieldDescriptorProto, error) {
	fieldName := fieldAssignment.GetFieldName()
	fieldDesc := &descriptorpb.FieldDescriptorProto{
		Name:   &fieldName,
		Number: &pos,
	}

	nodeIdx, _ := expressions.ExtractNodeIndex(fieldAssignment)

	switch {
	case fieldAssignment.HasCountAggregation():
		fieldDesc.Type = descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()
	case fieldAssignment.HasAvgAggregation(), fieldAssignment.HasStdDevAggregation():
		fieldDesc.Type = descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()
	// For NoAggregation or VectorAggregation, continue recursing the AST to determine the output schema.
	case fieldAssignment.HasAggregatedFieldValue():
		inferredTypeDesc, err := ti.expressionResolver.Resolve(nodeIdx)
		if err != nil {
			return nil, fmt.Errorf("Failed to infer field type for expression node %q in report field %q. Error: %w", ti.config.GetExpressionNodes()[nodeIdx].String(), fieldName, err)
		}

		if err := ti.normalizeAndTrackType(inferredTypeDesc); err != nil {
			return nil, err
		}

		// Map the strictly necessary type bindings onto our active output field descriptor.
		fieldDesc.Type = inferredTypeDesc.Type
		fieldDesc.TypeName = inferredTypeDesc.TypeName
		fieldDesc.Label = inferredTypeDesc.Label

		if fieldAssignment.HasVectorAggregation() {
			if inferredTypeDesc.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
				extractedTypeName := inferredTypeDesc.GetTypeName()
				if extractedTypeName == "" {
					extractedTypeName = inferredTypeDesc.GetType().String()
				}
				return nil, fmt.Errorf("Invalid vector aggregation on type REPEATED %s in field assignment to field %q. Vector aggregations can only be applied to singular types.", extractedTypeName, fieldName)
			}
			fieldDesc.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
		}
	default:
		return nil, fmt.Errorf("Unknown FieldAssignment type %q", fieldAssignment)
	}

	return fieldDesc, nil
}

// normalizeAndTrackType processes the FieldDescriptorProto returned by the ExpressionResolver.
// It checks whether the inferred type is a protobuf message or enum, ensuring their parent files
// are properly registered as dependencies in the adhocDescriptor output schema. If a message
// type is absent from the active output descriptors, it is generalized to google.protobuf.Any.
func (ti *typeInference) normalizeAndTrackType(inferredTypeDesc *descriptorpb.FieldDescriptorProto) error {
	var dependencies []string

	if inferredTypeDesc.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
		msgName := protoreflect.FullName(strings.TrimPrefix(inferredTypeDesc.GetTypeName(), "."))
		if !ti.IsTypeInOutputDescriptors(msgName) {
			dependencies = append(dependencies, "google/protobuf/any.proto")
			inferredTypeDesc.TypeName = proto.String(".google.protobuf.Any")
		} else {
			if msg, _ := ti.typeResolver.FindMessageByName(msgName); msg != nil {
				if dep := msg.Descriptor().ParentFile(); dep != nil && dep.Path() != "" {
					dependencies = append(dependencies, dep.Path())
				}
			}
		}
	} else if inferredTypeDesc.GetType() == descriptorpb.FieldDescriptorProto_TYPE_ENUM {
		enumName := protoreflect.FullName(strings.TrimPrefix(inferredTypeDesc.GetTypeName(), "."))
		enumType, err := ti.typeResolver.FindEnumByName(enumName)
		if err != nil {
			return fmt.Errorf("No definition found for enum type %q", enumName)
		}

		ti.referencedEnums[enumType.Descriptor().FullName()] = enumType.Descriptor()
		if dep := enumType.Descriptor().ParentFile(); dep != nil && dep.Path() != "" {
			dependencies = append(dependencies, dep.Path())
		}
	}

	ti.adhocDescriptor.Dependency = append(ti.adhocDescriptor.Dependency, dependencies...)
	return nil
}

func (ti *typeInference) validateTriggerTypes() []error {
	var errs []error
	for _, trigger := range ti.config.GetTriggers() {
		if ct := trigger.GetConditionalTrigger(); ct != nil {
			if !ct.HasSelectorNodeIndex() {
				continue
			}

			var initNodeIndex uint32
			hasInit := false

			switch ct.WhichConditionType() {
			case pb.ConditionalTrigger_RisingEdge_case:
				re := ct.GetRisingEdge()
				if re.HasInitializeNodeIndex() {
					initNodeIndex = re.GetInitializeNodeIndex()
					hasInit = true
				}
			case pb.ConditionalTrigger_FallingEdge_case:
				fe := ct.GetFallingEdge()
				if fe.HasInitializeNodeIndex() {
					initNodeIndex = fe.GetInitializeNodeIndex()
					hasInit = true
				}
			case pb.ConditionalTrigger_AllChanges_case:
				ac := ct.GetAllChanges()
				if ac.HasInitializeNodeIndex() {
					initNodeIndex = ac.GetInitializeNodeIndex()
					hasInit = true
				}
			}

			if hasInit {
				selectorType, err := ti.expressionResolver.Resolve(ct.GetSelectorNodeIndex())
				if err != nil {
					errs = append(errs, fmt.Errorf("Failed to resolve selector expression type for trigger %q: %w", trigger.GetName(), err))
					continue
				}

				initType, err := ti.expressionResolver.Resolve(initNodeIndex)
				if err != nil {
					errs = append(errs, fmt.Errorf("Failed to resolve initialize expression type for trigger %q: %w", trigger.GetName(), err))
					continue
				}

				// Check compatibility
				selectorIsNumeric := isNumeric(selectorType.Type)
				selectorIsBool := selectorType.GetType() == descriptorpb.FieldDescriptorProto_TYPE_BOOL

				initIsNumeric := isNumeric(initType.Type)
				initIsBool := initType.GetType() == descriptorpb.FieldDescriptorProto_TYPE_BOOL

				compatible := (selectorIsNumeric && initIsNumeric) || (selectorIsBool && initIsBool)

				if !compatible && ct.WhichConditionType() == pb.ConditionalTrigger_AllChanges_case {
					ac := ct.GetAllChanges()
					if ac.GetRisingOptions() == nil && ac.GetFallingOptions() == nil {
						if selectorType.GetType() == initType.GetType() {
							switch selectorType.GetType() {
							case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_TYPE_ENUM:
								compatible = selectorType.GetTypeName() == initType.GetTypeName()
							default:
								compatible = true
							}
						}
					}
				}

				if !compatible {
					errs = append(errs, fmt.Errorf("Trigger %q: initialize expression type %v is not compatible with selector expression type %v", trigger.GetName(), initType.GetType(), selectorType.GetType()))
				}
			}
		}
	}
	return errs
}
