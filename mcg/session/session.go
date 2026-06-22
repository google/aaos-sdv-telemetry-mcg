// Copyright 2023 Google LLC
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

package session

import (
	"fmt"

	"google.golang.org/protobuf/types/descriptorpb"

	"sdv.googlesource.com/mcg/mcg/expressions"
	"sdv.googlesource.com/mcg/mcg/mcuuid"
	"sdv.googlesource.com/mcg/mcg/type_resolvers"
	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

// Session represents a pending MetricsConfig the client is building.
//
// Sources are split up over multiple fields. Aggregators are
// in the Steps field, Data sources are explicitly represented, and
// Signal sources are represented using a name mapping from the source
// name to the signal name.
//
// Triggers are split up over condition triggers and two maps, each from the
// trigger name to the single parameter field.
type Session struct {
	VSIDLNames []string

	// Begin MetricsConfig data
	ConfigUUID                                   mcuuid.MCUUID
	StartTrigger, StopTrigger, DeactivateTrigger string
	Script                                       string

	Sources       map[string]*pb.Source
	Triggers      map[string]*pb.Trigger
	ReportConfigs map[string]*pb.MetricsReportConfig

	// Expression node indices in a session are actually references to this map, which contains the
	// uncompiled form of expressions
	Expressions map[uint32]expressions.Text
	// Next index to use in the `Expressions` map above. Monotonic; gaps are not filled except by
	// exporting and recreating the MetricsConfig
	NextUncompiledExpressionID uint32

	// Map from source identifier to full protobuf message name. If set for a data source, inference will
	// not attempt to infer the message name from the source identifier, but instead use what's present
	// in this map.
	DataSourceMessageTypes map[string]string

	// Request-provided input file descriptors parsed from the request config.
	InputDescriptors []*descriptorpb.FileDescriptorProto

	// Registry parsed from the VSIDL of the request. Select segments need to be included in the
	// output MetricsConfig
	ParsedTypes type_resolvers.EnrichedTypeResolver

	// Key is the name of either an aggregator or MetricsReportConfig.
	// The FieldName of the key is always the empty string.
	//
	// Value is either a protobuf primitive type name or a "."-prefixed message name.
	FieldTypes map[FieldTypeLocation]string

	// To bypass any validations for the metrics configs. Should default to false.
	IgnoreValidations bool

	// Disables Automatic Inference of Message Types where missing. Should default to false.
	NoMessageInference bool

	// Retain the state of aggregations when the metrics config is stopped.
	RetainAggregationsOnStop bool

	// DisallowComparisonOperatorChaining disallows chaining of comparison operators.
	DisallowComparisonOperatorChaining bool
}

// Save an expression into the session using a session-scoped expression ID.
func (s *Session) ExprStash(ptr *uint32, expr string) {
	*ptr = s.NextUncompiledExpressionID
	s.NextUncompiledExpressionID++
	s.Expressions[*ptr] = expressions.Text{Uncompiled: expr}
}

// Fully described path to a FieldAssignment in order to stash the user-specified field type.
type FieldTypeLocation struct {
	IsSource      bool
	ContainerName string
	FieldName     string
}

type MessageBuilderLocation struct {
	IsSource      bool
	ContainerName string
}

func (m *MessageBuilderLocation) ContainerPath() string {
	if m.IsSource {
		return fmt.Sprintf("sources[%s].aggregator", m.ContainerName)
	} else {
		return fmt.Sprintf("metrics_report_configs[%s]", m.ContainerName)
	}
}

func (m *MessageBuilderLocation) WithFieldName(fieldName string) FieldTypeLocation {
	return FieldTypeLocation{
		IsSource:      m.IsSource,
		ContainerName: m.ContainerName,
		FieldName:     fieldName,
	}
}

// Save a user-specified type inference hint to the session
func (s *Session) SaveFieldType(path FieldTypeLocation, hint string) {
	s.FieldTypes[path] = hint
}
