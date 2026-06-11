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

package requests

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/descriptorpb"

	"sdv.googlesource.com/mcg/mcg/constants"
	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
	"sdv.googlesource.com/mcg/mcg/expressions"
	"sdv.googlesource.com/mcg/mcg/mcuuid"
	"sdv.googlesource.com/mcg/mcg/session"
	"sdv.googlesource.com/mcg/mcg/type_resolvers"
	"sdv.googlesource.com/mcg/mcg/validators"
	"sdv.googlesource.com/mcg/mcg/vs_cache"
	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

type MetricsConfigRequest struct {
	Triggers []TriggerRequest `json:"triggers"`
	// Deprecated: Use DataSources and Aggregators instead.
	DeprecatedSources []DeprecatedSourceRequest `json:"publishers"`
	DataSources       []DataSourceRequest       `json:"data_sources"`
	Aggregators       []AggregatorRequest       `json:"aggregators"`
	ReportConfigs     []ReportConfigRequest     `json:"report_configs"`
	StartTrigger      string                    `json:"start_trigger_name,omitempty"`
	// Deprecated: Use StopTrigger instead.
	EndTrigger  string `json:"end_trigger_name,omitempty"`
	StopTrigger string `json:"stop_trigger_name,omitempty"`
	// Deprecated: Use DeactivateTrigger instead.
	FinishTrigger     string `json:"finish_trigger_name,omitempty"`
	DeactivateTrigger string `json:"deactivate_trigger_name,omitempty"`

	// Generate a metrics config with an existing UUID instead of a random one.
	ExistingUUID *mcuuid.MCUUID `json:"existing_uuid"`

	// An array of base64 encoded binary `descriptorpb.FileDescriptorSet`s
	// defining additional descriptors needed for the MetricsConfig. When
	// inference is enabled, these schemas are parsed, structurally optimized,
	// tree-shaken and then added to the MetricsConfig. If message inference is
	// disabled, they are copied verbatim into the resulting MetricsConfig
	// without filtering.
	DescriptorProtos [][]byte `json:"descriptor_protos"`

	// A base64 encoded binary format of a `descriptorpb.FileDescriptorSet` containing VSIDL
	// annotations for vehicle signals. One `descriptorpb.FileDescriptorSet` (converted into one
	// byte array) can consist of multiple proto files depending on each other.
	//
	// A base64 string: "CiBnb29nbGUvcHJvdG9idWY...Glvbg"
	VehicleSignals []byte `json:"vehicle_signals"`

	// A string referencing a registered vehicle signal version. This value takes precedence over
	// `vehicle_signals` if both are set"
	VSignalsVersion string `json:"vs_version"`

	// Deprecated: Use DataSourceMessageTypes instead.
	ServiceTypeHints       map[string]string `json:"service_type_hints,omitempty"`
	DataSourceMessageTypes map[string]string `json:"data_source_message_types,omitempty"`

	RetainAggregationsOnStop bool `json:"retain_aggregations_on_stop,omitempty"`
}

func (req *MetricsConfigRequest) ValidateSchemaConsistency(apiVersion constants.APIVersion) []*mcgerrors.StatusError {
	var canonicalFields, deprecatedFields []string
	var validationErrors []*mcgerrors.StatusError

	add := func(list *[]string, name string, condition bool) {
		if condition {
			*list = append(*list, name)
		}
	}

	// Check canonical fields.
	add(&canonicalFields, "data_sources", len(req.DataSources) > 0)
	for i, ds := range req.DataSources {
		if err := ds.ValidateCanonical(); err != nil {
			validationErrors = append(validationErrors, err)
		}
		add(&canonicalFields, fmt.Sprintf("data_sources[%d].connection_type(ON_DEMAND)", i), ds.ConnectionType == "ON_DEMAND")
		add(&deprecatedFields, fmt.Sprintf("data_sources[%d].connection_type(GETTER)", i), ds.ConnectionType == "GETTER")
	}

	add(&canonicalFields, "aggregators", len(req.Aggregators) > 0)
	for _, agg := range req.Aggregators {
		if err := agg.ValidateCanonical(); err != nil {
			validationErrors = append(validationErrors, err)
		}
	}
	add(&canonicalFields, "stop_trigger_name", req.StopTrigger != "")
	add(&canonicalFields, "deactivate_trigger_name", req.DeactivateTrigger != "")
	add(&canonicalFields, "data_source_message_types", len(req.DataSourceMessageTypes) > 0)

	// Check deprecated fields.
	add(&deprecatedFields, "publishers", len(req.DeprecatedSources) > 0)
	for i, source := range req.DeprecatedSources {
		if err := source.ValidateDeprecated(); err != nil {
			validationErrors = append(validationErrors, err)
		}
		if source.Service != nil {
			add(&canonicalFields, fmt.Sprintf("publishers[%d].service.connection_type(ON_DEMAND)", i), source.Service.ConnectionType == "ON_DEMAND")
			add(&deprecatedFields, fmt.Sprintf("publishers[%d].service.connection_type(GETTER)", i), source.Service.ConnectionType == "GETTER")
		}
	}

	add(&deprecatedFields, "end_trigger_name", req.EndTrigger != "")
	add(&deprecatedFields, "finish_trigger_name", req.FinishTrigger != "")
	add(&deprecatedFields, "service_type_hints", len(req.ServiceTypeHints) > 0)

	for i, trigger := range req.Triggers {
		if trigger.Data != nil {
			add(&canonicalFields, fmt.Sprintf("triggers[%d].data.source_name", i), trigger.Data.Source != "")
			add(&deprecatedFields, fmt.Sprintf("triggers[%d].data.publisher_name", i), trigger.Data.Publisher != "")
		}
	}

	if apiVersion == constants.APIVersionV1 && len(canonicalFields) > 0 {
		validationErrors = append(validationErrors, mcgerrors.InvalidArgument(fmt.Sprintf(
			"Configuration contains canonical fields which are not allowed in V1. Please use deprecated fields or switch to V2.\n"+
				"Canonical fields found: %v", canonicalFields)))
	}
	if apiVersion == constants.APIVersionV2 && len(deprecatedFields) > 0 {
		validationErrors = append(validationErrors, mcgerrors.InvalidArgument(fmt.Sprintf(
			"Configuration contains deprecated fields which are not allowed in V2. Please use canonical fields or switch to V1.\n"+
				"Deprecated fields found: %v", deprecatedFields)))
	}

	return validationErrors
}

func (req *MetricsConfigRequest) ToSession(ctx context.Context) (*session.Session, []*mcgerrors.StatusError) {
	sess := &session.Session{
		Sources:                    make(map[string]*pb.Source),
		Triggers:                   make(map[string]*pb.Trigger),
		ReportConfigs:              make(map[string]*pb.MetricsReportConfig),
		Expressions:                make(map[uint32]expressions.Text),
		NextUncompiledExpressionID: 1,
		FieldTypes:                 make(map[session.FieldTypeLocation]string),
		DataSourceMessageTypes:     make(map[string]string),
		IgnoreValidations:          false,
		NoMessageInference:         false,
	}

	var errorList []*mcgerrors.StatusError

	// 1. Initialize the Enriched Type Resolver from the Vehicle Signals catalog.
	if errs := initializeTypeRegistry(ctx, req, sess); errs != nil {
		return sess, errs
	}

	// 2. Extend the registry with custom user-provided Protobuf schemas from the request payload.
	if errs := extendRegistryWithCustomDescriptors(req, sess); errs != nil {
		return sess, errs
	}

	if errs := parseTriggers(req, sess); errs != nil {
		errorList = append(errorList, errs...)
	}

	// ValidateSchemaConsistency guarantees that there is no mixing of canonical and deprecated
	// fields in the request, meaning we can safely parse all blocks without colliding.
	if errs := parseDeprecatedSources(req, sess); errs != nil {
		errorList = append(errorList, errs...)
	}

	if errs := parseDataSources(req, sess); errs != nil {
		errorList = append(errorList, errs...)
	}

	if errs := parseAggregators(req, sess); errs != nil {
		errorList = append(errorList, errs...)
	}

	if errs := parseReportConfigs(req, sess); errs != nil {
		errorList = append(errorList, errs...)
	}

	sess.StartTrigger = req.StartTrigger

	sess.StopTrigger = req.StopTrigger
	if sess.StopTrigger == "" {
		sess.StopTrigger = req.EndTrigger
	}

	sess.DeactivateTrigger = req.DeactivateTrigger
	if sess.DeactivateTrigger == "" {
		sess.DeactivateTrigger = req.FinishTrigger
	}

	sess.DataSourceMessageTypes = req.DataSourceMessageTypes
	if len(req.ServiceTypeHints) > 0 {
		sess.DataSourceMessageTypes = req.ServiceTypeHints
	}

	sess.RetainAggregationsOnStop = req.RetainAggregationsOnStop

	return sess, errorList
}

// parseTriggers extracts and validates trigger configurations from a request,
// populating the given session. It ensures triggers are formatted correctly
// and contain no duplicate names.
func parseTriggers(req *MetricsConfigRequest, sess *session.Session) []*mcgerrors.StatusError {
	var errorList []*mcgerrors.StatusError
	for _, trigger := range req.Triggers {
		triggerPb, err := trigger.ToProto(sess)
		if err != nil {
			errorList = append(errorList, err)
			continue
		}
		if err = validators.ShallowValidateTrigger(triggerPb); err != nil {
			errorList = append(errorList, err)
			continue
		}
		if _, dup := sess.Triggers[triggerPb.GetName()]; dup {
			errorList = append(errorList, mcgerrors.AlreadyExists(fmt.Sprintf("Multiple triggers named %q", triggerPb.GetName())))
			continue
		}
		sess.Triggers[triggerPb.GetName()] = triggerPb
	}
	return errorList
}

// parseDeprecatedSources parses legacy V1 sources (publishers) sequentially,
// validating and storing them in the session if well-formed.
func parseDeprecatedSources(req *MetricsConfigRequest, sess *session.Session) []*mcgerrors.StatusError {
	var errorList []*mcgerrors.StatusError
	for _, sourceReq := range req.DeprecatedSources {
		sourcePb, err := sourceReq.ToProto(sess)
		if err != nil {
			errorList = append(errorList, err)
			continue
		}
		if err := validators.ShallowValidateSource(sourcePb); err != nil {
			errorList = append(errorList, err)
			continue
		}
		if _, dup := sess.Sources[sourcePb.GetName()]; dup {
			errorList = append(errorList, mcgerrors.AlreadyExists(fmt.Sprintf("Multiple publishers named %q", sourcePb.GetName())))
			continue
		}
		sess.Sources[sourcePb.GetName()] = sourcePb
	}
	return errorList
}

// parseDataSources converts requested canonical data sources into their native
// protobuf counterparts, running shallow validations and storing them in the session.
func parseDataSources(req *MetricsConfigRequest, sess *session.Session) []*mcgerrors.StatusError {
	var errorList []*mcgerrors.StatusError
	for _, sourceReq := range req.DataSources {
		sourcePb, err := sourceReq.ToProto(sess)
		if err != nil {
			errorList = append(errorList, err)
			continue
		}
		if err := validators.ShallowValidateDataSource(sourcePb.GetDataSource()); err != nil {
			errorList = append(errorList, err)
			continue
		}
		if _, dup := sess.Sources[sourcePb.GetName()]; dup {
			errorList = append(errorList, mcgerrors.AlreadyExists(fmt.Sprintf("Name %q is used twice", sourcePb.GetName())))
			continue
		}
		sess.Sources[sourcePb.GetName()] = sourcePb
	}
	return errorList
}

// parseAggregators sequentially processes aggregators,
// validating schema structure before storing them in the session.
func parseAggregators(req *MetricsConfigRequest, sess *session.Session) []*mcgerrors.StatusError {
	var errorList []*mcgerrors.StatusError
	for _, sourceReq := range req.Aggregators {
		sourcePb, err := sourceReq.ToProto(sess)
		if err != nil {
			errorList = append(errorList, err)
			continue
		}
		if err := validators.ShallowValidateAggregator(sourcePb.GetName(), sourcePb.GetAggregator()); err != nil {
			errorList = append(errorList, err)
			continue
		}
		if _, dup := sess.Sources[sourcePb.GetName()]; dup {
			errorList = append(errorList, mcgerrors.AlreadyExists(fmt.Sprintf("Name %q is used twice", sourcePb.GetName())))
			continue
		}
		sess.Sources[sourcePb.GetName()] = sourcePb
	}
	return errorList
}

// parseReportConfigs processes and shallow validates the requested metrics reports,
// storing valid definitions in the session.
func parseReportConfigs(req *MetricsConfigRequest, sess *session.Session) []*mcgerrors.StatusError {
	var errorList []*mcgerrors.StatusError
	for _, reportConfig := range req.ReportConfigs {
		reportConfigPb, err := reportConfig.ToProto(sess)
		if err != nil {
			errorList = append(errorList, err)
			continue
		}
		if err = validators.ShallowValidateMetricsReportConfig(reportConfigPb); err != nil {
			errorList = append(errorList, err)
			continue
		}
		if _, dup := sess.ReportConfigs[reportConfigPb.GetName()]; dup {
			errorList = append(errorList, mcgerrors.AlreadyExists(fmt.Sprintf("Multiple report configs named %q", reportConfigPb.GetName())))
			continue
		}
		sess.ReportConfigs[reportConfigPb.GetName()] = reportConfigPb
	}
	return errorList
}

// initializeTypeRegistry builds the EnrichedTypeResolver from either the cached
// VSIDL catalog version or the inline VehicleSignals payload, and stores it in the session.
func initializeTypeRegistry(ctx context.Context, req *MetricsConfigRequest, sess *session.Session) []*mcgerrors.StatusError {
	var errorList []*mcgerrors.StatusError

	if req.VSignalsVersion != "" {
		// Attempt to load the pre-compiled type registry from the active VSIDL cache.
		cache, err := vs_cache.GetCacheFromContext(ctx)
		if err != nil {
			errorList = append(errorList, mcgerrors.CacheRetrievalError())
		}
		if cache == nil || !cache.IsActive() {
			errorList = append(errorList, mcgerrors.NoCacheFailedLookup(req.VSignalsVersion))
			return errorList
		}

		typeResolver, err := cache.Get(ctx, req.VSignalsVersion)
		if err != nil {
			errorList = append(errorList, mcgerrors.FileDescriptorSetVehicleSignalsFailToParse(err))
			return errorList
		}
		if typeResolver == nil {
			errorList = append(errorList, mcgerrors.CacheFailedLookup(req.VSignalsVersion))
			return errorList
		}

		sess.ParsedTypes = *typeResolver
	} else {
		// Fallback to parsing and decoding the raw inline VehicleSignals payload directly.
		typeResolver, err := type_resolvers.NewEnrichedTypeResolverFromBytes(req.VehicleSignals)
		if err != nil {
			errorList = append(errorList, mcgerrors.FileDescriptorSetVehicleSignalsFailToParse(err))
			return errorList
		} else {
			sess.ParsedTypes = *typeResolver
		}
	}
	return errorList
}

// extendRegistryWithCustomDescriptors parses custom descriptor_protos from the request
// and injects them into the session's type registry. This allows the backend to validate and
// serialize generic Any payloads via type_urls, as well as support custom message_types
// defined explicitly in the configuration.
func extendRegistryWithCustomDescriptors(req *MetricsConfigRequest, sess *session.Session) []*mcgerrors.StatusError {
	var errorList []*mcgerrors.StatusError
	var inputDescriptors []*descriptorpb.FileDescriptorProto

	for idx, by := range req.DescriptorProtos {
		fds, err := type_resolvers.UnmarshalFileDescriptorSet(by)
		if err != nil {
			errorList = append(errorList, mcgerrors.FileDescriptorSetDescriptorProtosFailToParse(err, idx))
			continue
		}

		for _, fdp := range fds.File {
			inputDescriptors = append(inputDescriptors, fdp)
		}
	}

	fileDescriptorSet := &descriptorpb.FileDescriptorSet{File: inputDescriptors}
	// Add types from descriptor protos to resolver
	err := sess.ParsedTypes.ExtendLocalTypes(fileDescriptorSet)
	if err != nil {
		errorList = append(errorList, mcgerrors.FileDescriptorSetDescriptorProtosFailToParse(err, -1))
		return errorList
	}

	sess.InputDescriptors = inputDescriptors
	return errorList
}
