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

package validators_test

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
	"sdv.googlesource.com/mcg/mcg/session"
	"sdv.googlesource.com/mcg/mcg/validators"
	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

const defaultContainerName = ".default.container.name"

func TestTriggerShallowValidationPasses(t *testing.T) {
	testCases := []struct {
		name    string
		trigger *pb.Trigger
	}{
		{
			name: "conditionalTrigger",
			trigger: pb.Trigger_builder{
				Name: "TriggerName1",
				ConditionalTrigger: pb.ConditionalTrigger_builder{
					TriggerNames:      []string{"ConditionalTriggerName1"},
					AllChanges:        pb.ConditionalTrigger_ConditionTypeAllChanges_builder{}.Build(),
					SelectorNodeIndex: new(uint32),
				}.Build(),
			}.Build(),
		},
		{
			name: "dataTrigger",
			trigger: pb.Trigger_builder{
				Name: "TriggerName1",
				DataTrigger: pb.DataTrigger_builder{
					SourceName: "DataSourceName1",
				}.Build(),
			}.Build(),
		},
		{
			name: "periodicTrigger",
			trigger: pb.Trigger_builder{
				Name: "TriggerName1",
				PeriodicTrigger: pb.PeriodicTrigger_builder{
					Interval: durationpb.New(100 * time.Millisecond),
				}.Build(),
			}.Build(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validators.ShallowValidateTrigger(tc.trigger); err != nil {
				t.Errorf("validators.ShallowValidateTrigger(%v) = %v, want nil", tc.trigger, err)
			}
		})
	}
}

func TestTriggerShallowValidationFails(t *testing.T) {
	testCases := []struct {
		name      string
		trigger   *pb.Trigger
		wantError *mcgerrors.StatusError
	}{
		{
			name: "noTriggerType",
			trigger: pb.Trigger_builder{
				Name: "TriggerName1",
			}.Build(),
			wantError: mcgerrors.TriggerTypeUnknown,
		},
		{
			name: "dataTriggerMissingName",
			trigger: pb.Trigger_builder{
				DataTrigger: pb.DataTrigger_builder{
					SourceName: "DataSourceName1",
				}.Build(),
			}.Build(),
			wantError: mcgerrors.TriggerNameMissing,
		},
		{
			name: "periodicTriggerMissingName",
			trigger: pb.Trigger_builder{
				PeriodicTrigger: pb.PeriodicTrigger_builder{
					Interval: durationpb.New(time.Second),
				}.Build(),
			}.Build(),
			wantError: mcgerrors.TriggerNameMissing,
		},
		{
			name: "conditionalTriggerMissingName",
			trigger: pb.Trigger_builder{
				ConditionalTrigger: pb.ConditionalTrigger_builder{
					TriggerNames:      []string{"ConditionalTriggerName1"},
					AllChanges:        pb.ConditionalTrigger_ConditionTypeAllChanges_builder{}.Build(),
					SelectorNodeIndex: new(uint32),
				}.Build(),
			}.Build(),
			wantError: mcgerrors.TriggerNameMissing,
		},
		{
			name: "dataTriggerNoSourceName",
			trigger: pb.Trigger_builder{
				Name:        "TriggerName1",
				DataTrigger: pb.DataTrigger_builder{}.Build(),
			}.Build(),
			wantError: mcgerrors.DataTriggerSourceMissing,
		},
		{
			name: "periodicTriggerNoInterval",
			trigger: pb.Trigger_builder{
				Name:            "TriggerName1",
				PeriodicTrigger: pb.PeriodicTrigger_builder{}.Build(),
			}.Build(),
			wantError: mcgerrors.PeriodicTriggerIntervalMissing,
		},
		{
			name: "periodicTriggerZeroInterval",
			trigger: pb.Trigger_builder{
				Name: "TriggerName1",
				PeriodicTrigger: pb.PeriodicTrigger_builder{
					Interval: durationpb.New(0),
				}.Build(),
			}.Build(),
			wantError: mcgerrors.PeriodicTriggerIntervalMissing,
		},
		{
			name: "periodicTriggerNegativeInterval",
			trigger: pb.Trigger_builder{
				Name: "TriggerName1",
				PeriodicTrigger: pb.PeriodicTrigger_builder{
					Interval: durationpb.New(-3),
				}.Build(),
			}.Build(),
			wantError: mcgerrors.PeriodicTriggerIntervalNegative,
		},
		{
			name: "periodicTriggerZeroCount",
			trigger: pb.Trigger_builder{
				Name: "TriggerName1",
				PeriodicTrigger: pb.PeriodicTrigger_builder{
					Interval: durationpb.New(100 * time.Millisecond),
					Count:    new(uint32),
				}.Build(),
			}.Build(),
			wantError: mcgerrors.PeriodicTriggerInvalidCount,
		},
		{
			name: "conditionalTriggerNoParentTriggerNames",
			trigger: pb.Trigger_builder{
				Name: "TriggerName1",
				ConditionalTrigger: pb.ConditionalTrigger_builder{
					TriggerNames:      nil,
					AllChanges:        pb.ConditionalTrigger_ConditionTypeAllChanges_builder{}.Build(),
					SelectorNodeIndex: new(uint32),
				}.Build(),
			}.Build(),
			wantError: mcgerrors.ConditionalTriggerParentTriggersMissing,
		},
		{
			name: "conditionalTriggerNoSelectorNodeIndex",
			trigger: pb.Trigger_builder{
				Name: "TriggerName1",
				ConditionalTrigger: pb.ConditionalTrigger_builder{
					TriggerNames:      []string{"ConditionalTriggerName1"},
					AllChanges:        pb.ConditionalTrigger_ConditionTypeAllChanges_builder{}.Build(),
					SelectorNodeIndex: nil,
				}.Build(),
			}.Build(),
			wantError: mcgerrors.ConditionalTriggerExpressionIdMissing,
		},
		{
			name: "conditionalTriggerNoConditionType",
			trigger: pb.Trigger_builder{
				Name: "TriggerName1",
				ConditionalTrigger: pb.ConditionalTrigger_builder{
					TriggerNames:      []string{"ConditionalTriggerName1"},
					SelectorNodeIndex: new(uint32),
				}.Build(),
			}.Build(),
			wantError: mcgerrors.ConditionalTriggerConditionTypeMissing,
		},
		{
			name: "conditionalTriggerRisingEdgeWithFireInitialAndEdgeOptions",
			trigger: pb.Trigger_builder{
				Name: "TriggerName1",
				ConditionalTrigger: pb.ConditionalTrigger_builder{
					TriggerNames:      []string{"ConditionalTriggerName1"},
					SelectorNodeIndex: new(uint32),
					RisingEdge: pb.ConditionalTrigger_ConditionTypeRisingEdge_builder{
						RisingOptions: pb.ConditionalTrigger_EdgeOptions_builder{}.Build(),
						FireInitial:   true,
					}.Build(),
				}.Build(),
			}.Build(),
			wantError: mcgerrors.ConditionTypeEdgeOptionWithFireInitial,
		},
		{
			name: "conditionalTriggerFallingEdgeWithFireInitialAndEdgeOptions",
			trigger: pb.Trigger_builder{
				Name: "TriggerName1",
				ConditionalTrigger: pb.ConditionalTrigger_builder{
					TriggerNames:      []string{"ConditionalTriggerName1"},
					SelectorNodeIndex: new(uint32),
					FallingEdge: pb.ConditionalTrigger_ConditionTypeFallingEdge_builder{
						FallingOptions: pb.ConditionalTrigger_EdgeOptions_builder{}.Build(),
						FireInitial:    true,
					}.Build(),
				}.Build(),
			}.Build(),
			wantError: mcgerrors.ConditionTypeEdgeOptionWithFireInitial,
		},
		{
			name: "conditionalTriggerAllChangesWithFireInitialAndRisingEdgeOptions",
			trigger: pb.Trigger_builder{
				Name: "TriggerName1",
				ConditionalTrigger: pb.ConditionalTrigger_builder{
					TriggerNames:      []string{"ConditionalTriggerName1"},
					SelectorNodeIndex: new(uint32),
					AllChanges: pb.ConditionalTrigger_ConditionTypeAllChanges_builder{
						RisingOptions: pb.ConditionalTrigger_EdgeOptions_builder{}.Build(),
						FireInitial:   true,
					}.Build(),
				}.Build(),
			}.Build(),
			wantError: mcgerrors.ConditionTypeEdgeOptionWithFireInitial,
		},
		{
			name: "conditionalTriggerAllChangesWithFireInitialAndFallingEdgeOptions",
			trigger: pb.Trigger_builder{
				Name: "TriggerName1",
				ConditionalTrigger: pb.ConditionalTrigger_builder{
					TriggerNames:      []string{"ConditionalTriggerName1"},
					SelectorNodeIndex: new(uint32),
					AllChanges: pb.ConditionalTrigger_ConditionTypeAllChanges_builder{
						FallingOptions: pb.ConditionalTrigger_EdgeOptions_builder{}.Build(),
						FireInitial:    true,
					}.Build(),
				}.Build(),
			}.Build(),
			wantError: mcgerrors.ConditionTypeEdgeOptionWithFireInitial,
		},
		{
			name: "conditionalTriggerAllChangesWithFireInitialAndRisingandFallingEdgeOptions",
			trigger: pb.Trigger_builder{
				Name: "TriggerName1",
				ConditionalTrigger: pb.ConditionalTrigger_builder{
					TriggerNames:      []string{"ConditionalTriggerName1"},
					SelectorNodeIndex: new(uint32),
					AllChanges: pb.ConditionalTrigger_ConditionTypeAllChanges_builder{
						RisingOptions:  pb.ConditionalTrigger_EdgeOptions_builder{}.Build(),
						FallingOptions: pb.ConditionalTrigger_EdgeOptions_builder{}.Build(),
						FireInitial:    true,
					}.Build(),
				}.Build(),
			}.Build(),
			wantError: mcgerrors.ConditionTypeEdgeOptionWithFireInitial,
		},
		{
			name: "conditionalTriggerRisingEdgeWithFireInitialAndInitializeNodeIndex",
			trigger: pb.Trigger_builder{
				Name: "TriggerName1",
				ConditionalTrigger: pb.ConditionalTrigger_builder{
					TriggerNames:      []string{"ConditionalTriggerName1"},
					SelectorNodeIndex: new(uint32),
					RisingEdge: pb.ConditionalTrigger_ConditionTypeRisingEdge_builder{
						InitializeNodeIndex: new(uint32),
						FireInitial:         true,
					}.Build(),
				}.Build(),
			}.Build(),
			wantError: mcgerrors.ConditionTypeInitializeNodeIndexWithFireInitial,
		},
		{
			name: "conditionalTriggerFallingEdgeWithFireInitialAndInitializeNodeIndex",
			trigger: pb.Trigger_builder{
				Name: "TriggerName1",
				ConditionalTrigger: pb.ConditionalTrigger_builder{
					TriggerNames:      []string{"ConditionalTriggerName1"},
					SelectorNodeIndex: new(uint32),
					FallingEdge: pb.ConditionalTrigger_ConditionTypeFallingEdge_builder{
						InitializeNodeIndex: new(uint32),
						FireInitial:         true,
					}.Build(),
				}.Build(),
			}.Build(),
			wantError: mcgerrors.ConditionTypeInitializeNodeIndexWithFireInitial,
		},
		{
			name: "conditionalTriggerAllChangesWithFireInitialAndInitializeNodeIndex",
			trigger: pb.Trigger_builder{
				Name: "TriggerName1",
				ConditionalTrigger: pb.ConditionalTrigger_builder{
					TriggerNames:      []string{"ConditionalTriggerName1"},
					SelectorNodeIndex: new(uint32),
					AllChanges: pb.ConditionalTrigger_ConditionTypeAllChanges_builder{
						InitializeNodeIndex: new(uint32),
						FireInitial:         true,
					}.Build(),
				}.Build(),
			}.Build(),
			wantError: mcgerrors.ConditionTypeInitializeNodeIndexWithFireInitial,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validators.ShallowValidateTrigger(tc.trigger); err == nil || err.Status == nil || err.Status.Message != tc.wantError.Status.Message {
				t.Errorf("validators.ShallowValidateTrigger(%v) = %v, want error with status message %s", tc.trigger, err, tc.wantError)
			}
		})
	}
}

func TestSourceShallowValidationPasses(t *testing.T) {
	testCases := []struct {
		name   string
		source *pb.Source
	}{
		{
			name: "aggregator",
			source: pb.Source_builder{
				Name: "SourceName1",
				Aggregator: pb.Aggregator_builder{
					TriggerNames: []string{"TriggerName1"},
					ResetOnGet:   true,
					MessageBuilder: pb.ProtoMessageBuilder_builder{
						MessageType: ".aggregation.message_builder",
					}.Build(),
				}.Build(),
			}.Build(),
		},
		{
			name: "dataSource",
			source: pb.Source_builder{
				Name: "SourceName1",
				DataSource: pb.DataSource_builder{
					SourceIdentifier: "ServiceName1",
				}.Build(),
			}.Build(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validators.ShallowValidateSource(tc.source); err != nil {
				t.Errorf("validators.ShallowValidateSource(%v) = %v, want nil", tc.source, err)
			}
		})
	}
}

func TestShallowValidateSourceWithoutNameFails(t *testing.T) {
	for _, source := range []*pb.Source{
		pb.Source_builder{
			Aggregator: pb.Aggregator_builder{
				TriggerNames: []string{"TriggerName1"},
				ResetOnGet:   true,
				MessageBuilder: pb.ProtoMessageBuilder_builder{
					MessageType: ".aggregation.message_builder",
				}.Build(),
			}.Build(),
		}.Build(),
		pb.Source_builder{
			DataSource: pb.DataSource_builder{
				SourceIdentifier: "ServiceName1",
			}.Build(),
		}.Build(),
	} {
		want := mcgerrors.SourceNameMissing
		if err := validators.ShallowValidateSource(source); err.Status.Message != want.Status.Message {
			t.Fatalf("Shallow validation for sources without name should fail %s", err.Status.Message)
		}
	}
}

func TestShallowValidateAggregatorWithoutMessageBuilderFails(t *testing.T) {
	source := pb.Source_builder{
		Name: "SourceName1",
		Aggregator: pb.Aggregator_builder{
			TriggerNames:   []string{"TriggerName1"},
			ResetOnGet:     true,
			MessageBuilder: nil,
		}.Build(),
	}.Build()

	want := mcgerrors.AggregatorWithBlankMessageBuilder
	if err := validators.ShallowValidateSource(source); err.Status.Message != want.Status.Message {
		t.Fatalf("Shallow validation for aggregators without message builder should fail %s", err.Status.Message)
	}
}

func TestShallowValidateAggregatorWithoutTriggerNamesFails(t *testing.T) {
	for _, triggerNamesTestCase := range [][]string{{}, nil} {
		source := pb.Source_builder{
			Name: "SourceName2",
			Aggregator: pb.Aggregator_builder{
				TriggerNames: triggerNamesTestCase,
				ResetOnGet:   true,
				MessageBuilder: pb.ProtoMessageBuilder_builder{
					MessageType: ".aggregation.message_builder",
				}.Build(),
			}.Build(),
		}.Build()

		want := mcgerrors.AggregatorTriggersMissing
		if err := validators.ShallowValidateSource(source); err.Status.Message != want.Status.Message {
			t.Fatalf("Shallow validation for aggregators without triggers should fail %s", err.Status.Message)
		}
	}
}

func TestShallowValidateDataSourceWithoutNameFails(t *testing.T) {
	source := pb.Source_builder{
		Name:       "SourceName1",
		DataSource: pb.DataSource_builder{}.Build(),
	}.Build()

	want := mcgerrors.DataSourceIdentifierMissing
	if err := validators.ShallowValidateSource(source); err.Status.Message != want.Status.Message {
		t.Fatalf("Shallow validation for data sources without source identifier should fail %s", err.Status.Message)
	}
}

func TestProtoMessageBuilderShallowValidationPasses(t *testing.T) {
	fieldAssignment := pb.ProtoMessageBuilder_FieldAssignment_builder{
		FieldName: "speed",
		AvgAggregation: pb.ProtoMessageBuilder_FieldAssignment_AvgAggregation_builder{
			ExpressionNodeIndex: new(uint32),
		}.Build(),
	}.Build()

	// MessageType should be empty or start with dot.
	for _, messageTypeTestCase := range []string{"", defaultContainerName} {
		pmb := pb.ProtoMessageBuilder_builder{
			MessageType:      messageTypeTestCase,
			FieldAssignments: []*pb.ProtoMessageBuilder_FieldAssignment{fieldAssignment},
		}.Build()

		if err := validators.ShallowValidateProtoMessageBuilder(session.MessageBuilderLocation{
			IsSource:      true,
			ContainerName: messageTypeTestCase,
		}, pmb); err != nil {
			t.Fatalf("With MessageType %s got %#v", messageTypeTestCase, err)
		}
	}
}

func TestShallowValidateProtoMessageBuilderWithoutDotPrefixFails(t *testing.T) {
	fieldAssignment := pb.ProtoMessageBuilder_FieldAssignment_builder{
		FieldName: "speed",
		AvgAggregation: pb.ProtoMessageBuilder_FieldAssignment_AvgAggregation_builder{
			ExpressionNodeIndex: new(uint32),
		}.Build(),
	}.Build()

	pmb := pb.ProtoMessageBuilder_builder{
		MessageType:      "aggregation.message_builder",
		FieldAssignments: []*pb.ProtoMessageBuilder_FieldAssignment{fieldAssignment},
	}.Build()

	want := mcgerrors.MessageBuilderMessageTypeNotStartingWithDot("path", pmb.GetMessageType())
	if err := validators.ShallowValidateProtoMessageBuilder(session.MessageBuilderLocation{
		IsSource:      false,
		ContainerName: pmb.GetMessageType(),
	}, pmb); err.Status.Message != want.Status.Message {
		t.Fatalf("Shallow validation for msg builders not starting with dot should fail %s", err.Status.Message)
	}
}

func TestShallowValidateProtoMessageBuilderWithoutFieldNameFails(t *testing.T) {
	fieldAssignment := pb.ProtoMessageBuilder_FieldAssignment_builder{
		AvgAggregation: pb.ProtoMessageBuilder_FieldAssignment_AvgAggregation_builder{
			ExpressionNodeIndex: new(uint32),
		}.Build(),
	}.Build()

	pmb := pb.ProtoMessageBuilder_builder{
		MessageType:      defaultContainerName,
		FieldAssignments: []*pb.ProtoMessageBuilder_FieldAssignment{fieldAssignment},
	}.Build()

	want := mcgerrors.MessageBuilderFieldAssignmentFieldNameMissing("path", 0)
	if err := validators.ShallowValidateProtoMessageBuilder(session.MessageBuilderLocation{
		IsSource:      false,
		ContainerName: defaultContainerName,
	}, pmb); err.Status.Message != want.Status.Message {
		t.Fatalf("Shallow validation for msg builders without field assignment field name should fail %s", err.Status.Message)
	}
}

func TestShallowValidateProtoMessageBuilderWithoutAggregatedFieldValueFails(t *testing.T) {
	fieldAssignment := pb.ProtoMessageBuilder_FieldAssignment_builder{
		FieldName: "speed",
	}.Build()

	pmb := pb.ProtoMessageBuilder_builder{
		MessageType:      defaultContainerName,
		FieldAssignments: []*pb.ProtoMessageBuilder_FieldAssignment{fieldAssignment},
	}.Build()

	want := mcgerrors.MessageBuilderFieldAssignmentAggregationMissing("path", 0)
	if err := validators.ShallowValidateProtoMessageBuilder(session.MessageBuilderLocation{
		IsSource:      false,
		ContainerName: defaultContainerName,
	}, pmb); err.Status.Message != want.Status.Message {
		t.Fatalf("Shallow validation for msg builders without field assignment aggregation should fail %s", err.Status.Message)
	}
}

func TestMetricsReportConfigShallowValidationPasses(t *testing.T) {
	for _, reportIncompleteTestCase := range []bool{true, false} {
		mrc := pb.MetricsReportConfig_builder{
			Name:             "MetricsReportConfigName1",
			TriggerNames:     []string{"TriggerName1"},
			ReportIncomplete: reportIncompleteTestCase,
			MessageBuilder: pb.ProtoMessageBuilder_builder{
				MessageType: ".report.message_builder",
			}.Build(),
		}.Build()

		if err := validators.ShallowValidateMetricsReportConfig(mrc); err != nil {
			t.Fatalf("ReportIncomplete: %t\n, error %#v", reportIncompleteTestCase, err)
		}
	}
}

func TestShallowValidateMetricsReportConfigWithoutNameFails(t *testing.T) {
	mrc := pb.MetricsReportConfig_builder{
		TriggerNames: []string{"TriggerName1"},
		MessageBuilder: pb.ProtoMessageBuilder_builder{
			MessageType: ".report.message_builder",
		}.Build(),
	}.Build()

	want := mcgerrors.MetricsReportConfigNameMissing
	if err := validators.ShallowValidateMetricsReportConfig(mrc); err.Status.Message != want.Status.Message {
		t.Fatalf("Shallow validation for metrics reports without name should fail %s", err.Status.Message)
	}
}

func TestShallowValidateNonIncompleteMetricsReportConfigWithoutTriggerNamesFails(t *testing.T) {
	for _, triggerNamesTestCase := range [][]string{{}, nil} {
		mrc := pb.MetricsReportConfig_builder{
			Name:             "MetricsReportConfigName1",
			TriggerNames:     triggerNamesTestCase,
			ReportIncomplete: false,
			MessageBuilder: pb.ProtoMessageBuilder_builder{
				MessageType: ".report.message_builder",
			}.Build(),
		}.Build()

		want := mcgerrors.MetricsReportConfigTriggersMissingAndNotSetIncomplete
		if err := validators.ShallowValidateMetricsReportConfig(mrc); err.Status.Message != want.Status.Message {
			t.Fatalf("Shallow validation for metrics reports without trigger names should fail %s", err.Status.Message)
		}
	}
}
