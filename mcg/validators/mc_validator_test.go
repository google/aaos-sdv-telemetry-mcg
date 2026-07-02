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
	goerrors "errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"sdv.googlesource.com/mcg/mcg/constants"
	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
	"sdv.googlesource.com/mcg/mcg/validators"
	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

const (
	EIPF_B_TEXTPROTO_FILENAME = "../testdata/eipf_b.textproto"
)

func assertErrors(v *validators.McValidator, wants ...*mcgerrors.StatusError) error {
	got := v.ErrorList
	if len(got) != len(wants) {
		return goerrors.New(fmt.Sprintf("Wrong amount of errors returned, want %d, got %d", len(wants), len(got)))
	}

	for i, want := range wants {
		if want.Status.Message != got[i].Status.Message {
			return goerrors.New(fmt.Sprintf("Errors don't match. want \"%s\"\ngot: \"%s\"\n", want.Status.Message, got[i].Status.Message))
		}
	}

	return nil
}

func readMetricsConfigFromFile(filename string, t *testing.T) *pb.MetricsConfig {
	textProtoAsBytes, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var mc pb.MetricsConfig
	if err := prototext.Unmarshal(textProtoAsBytes, &mc); err != nil {
		t.Fatal(err)
	}
	return &mc
}

func TestValidateFileDescriptorWithoutFileNamePasses(t *testing.T) {
	mc := readMetricsConfigFromFile(EIPF_B_TEXTPROTO_FILENAME, t)
	mc.SetUuid(uuid.New().String())
	mc.GetDescriptorProtos()[0].Name = nil

	v := validators.NewMcValidator(mc, false)
	validators.Validate(v)

	if len(v.ErrorList) != 0 {
		validators.PrintErrorList(v)
		t.Fatal("Validation should pass but returned the above error(s).")
	}
}

func TestValidateWithValidMetricsConfigsPasses(t *testing.T) {
	node := pb.Node_builder{
		ConstantLeafNode: pb.ConstantLeafNode_builder{
			Int32Value: proto.Int32(1),
		}.Build(),
	}.Build()

	dataSource := pb.Source_builder{
		Name: "SourceName",
		DataSource: pb.DataSource_builder{
			SourceIdentifier: "SourceIdentifier",
		}.Build(),
	}.Build()

	trigger := pb.Trigger_builder{
		Name: "TriggerName",
		PeriodicTrigger: pb.PeriodicTrigger_builder{
			Interval: durationpb.New(100 * time.Millisecond),
		}.Build(),
	}.Build()

	mrc := pb.MetricsReportConfig_builder{
		Name:             "MetricsReportConfigName",
		TriggerNames:     []string{"TriggerName"},
		ReportIncomplete: true,
		MessageBuilder: pb.ProtoMessageBuilder_builder{
			MessageType: ".google.protobuf.Any",
			FieldAssignments: []*pb.ProtoMessageBuilder_FieldAssignment{
				pb.ProtoMessageBuilder_FieldAssignment_builder{
					FieldName: "field_1",
					MaxAggregation: pb.ProtoMessageBuilder_FieldAssignment_MaxAggregation_builder{
						ExpressionNodeIndex: new(uint32),
					}.Build(),
				}.Build(),
			},
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Uuid:                 "1cbec8e6-46f5-4649-9fa2-09f81bc93f8b",
		Version:              constants.MetricsConfigVersion,
		ExpressionNodes:      []*pb.Node{node},
		Sources:              []*pb.Source{dataSource},
		Triggers:             []*pb.Trigger{trigger},
		MetricsReportConfigs: []*pb.MetricsReportConfig{mrc},
	}.Build(), false)

	validators.Validate(v)

	if len(v.ErrorList) != 0 {
		validators.PrintErrorList(v)
		t.Fatal("Validation should pass with well configured metrics configs, but the above errors were thrown.")
	}
}

func TestValidateNoSourceTriggerCyclesPasses(t *testing.T) {
	dataSource := pb.Source_builder{
		Name: "DataSourceName",
		DataSource: pb.DataSource_builder{
			SourceIdentifier: "SourceIdentifier",
		}.Build(),
	}.Build()
	dataTrigger1 := pb.Trigger_builder{
		Name: "DataTrigger1",
		DataTrigger: pb.DataTrigger_builder{
			SourceName: dataSource.GetName(),
		}.Build(),
	}.Build()
	aggregator := pb.Source_builder{
		Name: "AggregatorName",
		Aggregator: pb.Aggregator_builder{
			TriggerNames: []string{dataTrigger1.GetName()},
			ResetOnGet:   true,
			MessageBuilder: pb.ProtoMessageBuilder_builder{
				MessageType: ".google.protobuf.BoolValue",
			}.Build(),
		}.Build(),
	}.Build()
	dataTrigger2 := pb.Trigger_builder{
		Name: "DataTrigger2",
		DataTrigger: pb.DataTrigger_builder{
			SourceName: aggregator.GetName(),
		}.Build(),
	}.Build()
	conditionalTrigger1 := pb.Trigger_builder{
		Name: "ConditionalTriggerName1",
		ConditionalTrigger: pb.ConditionalTrigger_builder{
			TriggerNames:      []string{dataTrigger2.GetName()},
			AllChanges:        pb.ConditionalTrigger_ConditionTypeAllChanges_builder{}.Build(),
			SelectorNodeIndex: new(uint32),
		}.Build(),
	}.Build()
	conditionalTrigger2 := pb.Trigger_builder{
		Name: "ConditionalTriggerName2",
		ConditionalTrigger: pb.ConditionalTrigger_builder{
			TriggerNames:      []string{dataTrigger2.GetName()},
			AllChanges:        pb.ConditionalTrigger_ConditionTypeAllChanges_builder{}.Build(),
			SelectorNodeIndex: new(uint32),
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Sources:  []*pb.Source{dataSource, aggregator},
		Triggers: []*pb.Trigger{dataTrigger1, dataTrigger2, conditionalTrigger1, conditionalTrigger2},
	}.Build(), false)

	validators.ValidateNoSourceTriggerCycles(v)
	if len(v.ErrorList) != 0 {
		validators.PrintErrorList(v)
		t.Fatal("Validation of source and triggers dependencies should pass when no cycles are present.")
	}
}

func TestValidateNoSourceTriggerCyclesWithDiamondCycleStartingFromSourcePasses(t *testing.T) {
	aggregator := pb.Source_builder{
		Name: "AggregatorName",
		Aggregator: pb.Aggregator_builder{
			TriggerNames: []string{"TriggerName1", "TriggerName2"},
			ResetOnGet:   true,
			MessageBuilder: pb.ProtoMessageBuilder_builder{
				MessageType: ".google.protobuf.UInt32Value",
			}.Build(),
		}.Build(),
	}.Build()
	dataSource := pb.Source_builder{
		Name: "DataSourceName",
		DataSource: pb.DataSource_builder{
			SourceIdentifier: "SourceIdentifier",
		}.Build(),
	}.Build()
	dataTrigger1 := pb.Trigger_builder{
		Name: "TriggerName1",
		DataTrigger: pb.DataTrigger_builder{
			SourceName: "DataSourceName",
		}.Build(),
	}.Build()
	dataTrigger2 := pb.Trigger_builder{
		Name: "TriggerName2",
		DataTrigger: pb.DataTrigger_builder{
			SourceName: "DataSourceName",
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Sources:  []*pb.Source{aggregator, dataSource},
		Triggers: []*pb.Trigger{dataTrigger1, dataTrigger2},
	}.Build(), false)

	validators.ValidateNoSourceTriggerCycles(v)
	if len(v.ErrorList) != 0 {
		validators.PrintErrorList(v)
		t.Fatal("Validation of source and triggers dependencies should pass despite a 'diamond cycle' because it's a directed acyclic graph (DAG).")
	}
}

func TestValidateNoSourceTriggerCyclesWithCycleStartingFromTriggerFails(t *testing.T) {
	aggregator := pb.Source_builder{
		Name: "AggregatorName",
		Aggregator: pb.Aggregator_builder{
			TriggerNames: []string{"TriggerName"},
			ResetOnGet:   true,
			MessageBuilder: pb.ProtoMessageBuilder_builder{
				MessageType: ".google.protobuf.Duration",
			}.Build(),
		}.Build(),
	}.Build()
	dataTrigger := pb.Trigger_builder{
		Name: "TriggerName",
		DataTrigger: pb.DataTrigger_builder{
			SourceName: "AggregatorName",
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Sources:  []*pb.Source{aggregator},
		Triggers: []*pb.Trigger{dataTrigger},
	}.Build(), false)

	validators.ValidateNoSourceTriggerCycles(v)
	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.CyclicDependency(dataTrigger.GetName()).Status.Message {
		validators.PrintErrorList(v)
		t.Fatalf("Validation should not pass with cyclic dependencies: %v", v.ErrorList)
	}
}

func TestValidateNoSourceTriggerCyclesWithCycleStartingFromSourceFails(t *testing.T) {
	aggregator := pb.Source_builder{
		Name: "AggregatorName",
		Aggregator: pb.Aggregator_builder{
			TriggerNames: []string{"TriggerName"},
			ResetOnGet:   true,
			MessageBuilder: pb.ProtoMessageBuilder_builder{
				MessageType: ".google.protobuf.SourceContext",
			}.Build(),
		}.Build(),
	}.Build()
	dataTrigger := pb.Trigger_builder{
		Name: "TriggerName",
		DataTrigger: pb.DataTrigger_builder{
			SourceName: "AggregatorName",
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Sources:  []*pb.Source{aggregator},
		Triggers: []*pb.Trigger{dataTrigger},
	}.Build(), false)

	validators.ValidateNoSourceTriggerCycles(v)
	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.CyclicDependency(dataTrigger.GetName()).Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation should not pass with cyclic dependencies.")
	}
}

func TestValidateNoSourceTriggerCyclesWithConditionalTriggerFormingCycleFails(t *testing.T) {
	aggregator := pb.Source_builder{
		Name: "AggregatorName",
		Aggregator: pb.Aggregator_builder{
			TriggerNames: []string{"TriggerName2"},
			ResetOnGet:   true,
			MessageBuilder: pb.ProtoMessageBuilder_builder{
				MessageType: ".google.protobuf.Struct",
			}.Build(),
		}.Build(),
	}.Build()
	dataTrigger := pb.Trigger_builder{
		Name: "TriggerName1",
		DataTrigger: pb.DataTrigger_builder{
			SourceName: "AggregatorName",
		}.Build(),
	}.Build()
	conditionalTrigger := pb.Trigger_builder{
		Name: "TriggerName2",
		ConditionalTrigger: pb.ConditionalTrigger_builder{
			TriggerNames:      []string{"TriggerName1"},
			AllChanges:        pb.ConditionalTrigger_ConditionTypeAllChanges_builder{}.Build(),
			SelectorNodeIndex: new(uint32),
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Sources:  []*pb.Source{aggregator},
		Triggers: []*pb.Trigger{dataTrigger, conditionalTrigger},
	}.Build(), false)

	validators.ValidateNoSourceTriggerCycles(v)
	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.CyclicDependency(dataTrigger.GetName()).Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation should not pass with cyclic dependencies.")
	}
}

func TestValidateNoSourceTriggerCyclesWithPeriodicTriggerFormingCycleFails(t *testing.T) {
	aggregator := pb.Source_builder{
		Name: "AggregatorName",
		Aggregator: pb.Aggregator_builder{
			TriggerNames: []string{"TriggerName2"},
			ResetOnGet:   true,
			MessageBuilder: pb.ProtoMessageBuilder_builder{
				MessageType: ".google.protobuf.Struct",
			}.Build(),
		}.Build(),
	}.Build()
	dataTrigger := pb.Trigger_builder{
		Name: "TriggerName1",
		DataTrigger: pb.DataTrigger_builder{
			SourceName: "AggregatorName",
		}.Build(),
	}.Build()
	periodicTrigger := pb.Trigger_builder{
		Name: "TriggerName2",
		PeriodicTrigger: pb.PeriodicTrigger_builder{
			Interval:     durationpb.New(100 * time.Millisecond),
			TriggerNames: []string{"TriggerName1"},
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Sources:  []*pb.Source{aggregator},
		Triggers: []*pb.Trigger{dataTrigger, periodicTrigger},
	}.Build(), false)

	validators.ValidateNoSourceTriggerCycles(v)
	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.CyclicDependency(dataTrigger.GetName()).Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation should not pass with cyclic dependencies involving periodic triggers.")
	}
}

func TestValidateExpressionNodesPasses(t *testing.T) {
	nodes := []*pb.Node{
		pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{
			Int32Value: proto.Int32(1),
		}.Build()}.Build(),
		pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{
			Int32Value: proto.Int32(2),
		}.Build()}.Build(),
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{
			LeftIndex:          proto.Uint32(0),
			RightIndex:         proto.Uint32(1),
			ArithmeticOperator: pb.CombinationNode_DIVIDE.Enum(),
		}.Build()}.Build(),
		pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{
			Int32Value: proto.Int32(5),
		}.Build()}.Build(),
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{
			LeftIndex:          proto.Uint32(2),
			RightIndex:         proto.Uint32(3),
			ArithmeticOperator: pb.CombinationNode_ADD.Enum(),
		}.Build()}.Build(),
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{
			LeftIndex:        proto.Uint32(4),
			RoundingOperator: pb.CombinationNode_FLOOR.Enum(),
		}.Build()}.Build(),
	}

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		ExpressionNodes: nodes,
	}.Build(), false)

	validators.ValidateExpressionNodes(v)
	if len(v.ErrorList) != 0 {
		validators.PrintErrorList(v)
		t.Fatal("Validation of expression nodes should pass when no cycles are present.")
	}
}

func TestValidateExpressionNodesWithCycleFails(t *testing.T) {
	nodes := []*pb.Node{
		pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{
			Int32Value: proto.Int32(1),
		}.Build()}.Build(),
		pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{
			Int32Value: proto.Int32(2),
		}.Build()}.Build(),
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{
			LeftIndex:          proto.Uint32(0),
			RightIndex:         proto.Uint32(3),
			ArithmeticOperator: pb.CombinationNode_DIVIDE.Enum(),
		}.Build()}.Build(),
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{
			LeftIndex:          proto.Uint32(1),
			RightIndex:         proto.Uint32(2),
			ArithmeticOperator: pb.CombinationNode_ADD.Enum(),
		}.Build()}.Build(),
	}

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		ExpressionNodes: nodes,
	}.Build(), false)

	validators.ValidateExpressionNodes(v)
	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.CyclicDependency("expression_nodes[2]").Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation of expression nodes should not pass with cyclic dependencies.")
	}
}

func TestValidateExpressionNodesWithUnaryOperatorWithRightNodeFails(t *testing.T) {
	nodes := []*pb.Node{
		pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{
			Int32Value: proto.Int32(1),
		}.Build()}.Build(),
		pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{
			Int32Value: proto.Int32(2),
		}.Build()}.Build(),
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{
			LeftIndex:       proto.Uint32(0),
			RightIndex:      proto.Uint32(1),
			LogicalOperator: pb.CombinationNode_NOT.Enum(),
		}.Build()}.Build(),
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{
			LeftIndex:        proto.Uint32(0),
			RightIndex:       proto.Uint32(1),
			RoundingOperator: pb.CombinationNode_FLOOR.Enum(),
		}.Build()}.Build(),
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{
			LeftIndex:        proto.Uint32(0),
			RightIndex:       proto.Uint32(1),
			RoundingOperator: pb.CombinationNode_CEIL.Enum(),
		}.Build()}.Build(),
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{
			LeftIndex:        proto.Uint32(0),
			RightIndex:       proto.Uint32(1),
			RoundingOperator: pb.CombinationNode_ROUND.Enum(),
		}.Build()}.Build(),
	}

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		ExpressionNodes: nodes,
	}.Build(), false)

	validators.ValidateExpressionNodes(v)

	if err := assertErrors(v,
		mcgerrors.UnaryOperatorExpressionNodeHasRightIndexSet(2),
		mcgerrors.UnaryOperatorExpressionNodeHasRightIndexSet(3),
		mcgerrors.UnaryOperatorExpressionNodeHasRightIndexSet(4),
		mcgerrors.UnaryOperatorExpressionNodeHasRightIndexSet(5),
	); err != nil {
		validators.PrintErrorList(v)
		t.Fatalf("Validation of expression nodes should not pass if an unary operator has the right node set. Error: %s", err)
	}
}

func TestValidateExpressionNodesWithNecessaryRightOrLeftNodeMissingFails(t *testing.T) {
	nodes := []*pb.Node{
		pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{
			Int32Value: proto.Int32(1),
		}.Build()}.Build(),
		// Non-unary-operator with missing right index
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{
			LeftIndex:       proto.Uint32(0),
			LogicalOperator: pb.CombinationNode_AND.Enum(),
		}.Build()}.Build(),
		// Non-unary-operator with missing left index
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{
			RightIndex:      proto.Uint32(0),
			LogicalOperator: pb.CombinationNode_AND.Enum(),
		}.Build()}.Build(),
		// Non-unary-operator with both indices missing
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{
			LogicalOperator: pb.CombinationNode_AND.Enum(),
		}.Build()}.Build(),
		// Unary-operator with missing left index
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{
			RoundingOperator: pb.CombinationNode_CEIL.Enum(),
		}.Build()}.Build(),
	}

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		ExpressionNodes: nodes,
	}.Build(), false)

	validators.ValidateExpressionNodes(v)

	if err := assertErrors(v,
		mcgerrors.NonUnaryCombinationExpressionNodeDoesntHaveRightIndexSet(1),
		mcgerrors.CombinationExpressionNodeDoesntHaveLeftIndexSet(2),
		mcgerrors.CombinationExpressionNodeDoesntHaveLeftIndexSet(3),
		mcgerrors.NonUnaryCombinationExpressionNodeDoesntHaveRightIndexSet(3),
		mcgerrors.CombinationExpressionNodeDoesntHaveLeftIndexSet(4),
	); err != nil {
		validators.PrintErrorList(v)
		t.Fatalf("Validation of expression nodes should not pass if a non-unary operator doens't have both left and right index set. Error: %s", err)
	}
}

func TestValidateExpressionNodesWithInvalidNodeExpressionReferenceFails(t *testing.T) {
	nodes := []*pb.Node{
		pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{
			Int32Value: proto.Int32(1),
		}.Build()}.Build(),
		// Non-unary-operator with invalid right index
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{
			LeftIndex:       proto.Uint32(0),
			RightIndex:      proto.Uint32(5),
			LogicalOperator: pb.CombinationNode_AND.Enum(),
		}.Build()}.Build(),
		// Non-unary-operator with invalid left index
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{
			LeftIndex:       proto.Uint32(6),
			RightIndex:      proto.Uint32(0),
			LogicalOperator: pb.CombinationNode_AND.Enum(),
		}.Build()}.Build(),
		// Unary-operator with invalid left index
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{
			LeftIndex:        proto.Uint32(7),
			RoundingOperator: pb.CombinationNode_CEIL.Enum(),
		}.Build()}.Build(),
	}

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		ExpressionNodes: nodes,
	}.Build(), false)

	validators.ValidateExpressionNodes(v)

	if err := assertErrors(v,
		mcgerrors.CombinationExpressionNodeWithInvalidExpressionNodeReference(1),
		mcgerrors.CombinationExpressionNodeWithInvalidExpressionNodeReference(2),
		mcgerrors.CombinationExpressionNodeWithInvalidExpressionNodeReference(3),
	); err != nil {
		validators.PrintErrorList(v)
		t.Fatalf("Validation of expression nodes should not pass if a node refers to another non-existent node. Error: %s", err)
	}
}

func TestValidateExpressionNodesWithInvalidSourceReferenceFails(t *testing.T) {
	dataSource := pb.Source_builder{
		Name: "DataSourceName",
		DataSource: pb.DataSource_builder{
			SourceIdentifier: "SourceIdentifier",
		}.Build(),
	}.Build()
	node := pb.Node_builder{
		FieldLeafNode: pb.FieldLeafNode_builder{
			SourceName: "DataSourceName1", // Non-existent source
			FieldNames: []string{"field_1"},
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		ExpressionNodes: []*pb.Node{node},
		Sources:         []*pb.Source{dataSource},
	}.Build(), false)

	validators.ValidateExpressionNodes(v)
	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.ExpressionNodeWithInvalidSourceReference(0, "DataSourceName1").Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation of expression nodes should not pass if a node refers to a non-existent source.")
	}
}

func TestValidateConditionalTriggerWithInvalidParentReferencesFails(t *testing.T) {
	invalidParentTriggerName := "InvalidParentTriggerName"
	trigger := pb.Trigger_builder{
		Name: "ConditionalTriggerName",
		ConditionalTrigger: pb.ConditionalTrigger_builder{
			TriggerNames:      []string{invalidParentTriggerName},
			AllChanges:        pb.ConditionalTrigger_ConditionTypeAllChanges_builder{}.Build(),
			SelectorNodeIndex: new(uint32),
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Triggers: []*pb.Trigger{trigger},
	}.Build(), false)

	validators.ValidateTriggers(v)

	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.ConditionalParentTriggerNameNotMatchingAnyExistingTrigger(invalidParentTriggerName).Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation should not pass with conditional trigger referring to a non-existent parent trigger")
	}
}

func TestValidatePeriodicTriggerWithInvalidParentReferencesFails(t *testing.T) {
	invalidParentTriggerName := "InvalidParentTriggerName"
	trigger := pb.Trigger_builder{
		Name: "PeriodicTriggerName",
		PeriodicTrigger: pb.PeriodicTrigger_builder{
			Interval:     durationpb.New(100 * time.Millisecond),
			TriggerNames: []string{invalidParentTriggerName},
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Triggers: []*pb.Trigger{trigger},
	}.Build(), false)

	validators.ValidateTriggers(v)

	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.PeriodicParentTriggerNameNotMatchingAnyExistingTrigger(invalidParentTriggerName).Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation should not pass with periodic trigger referring to a non-existent parent trigger")
	}
}

func TestValidateDataTriggerWithInvalidSourceReferencesFails(t *testing.T) {
	trigger := pb.Trigger_builder{
		Name: "DataTrigger",
		DataTrigger: pb.DataTrigger_builder{
			SourceName: "NonExistentSourceName",
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Triggers: []*pb.Trigger{trigger},
	}.Build(), false)

	validators.ValidateTriggers(v)

	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.DataTriggerSourceReferenceInvalid("NonExistentSourceName").Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation should not pass with data trigger referring to a non-existent source")
	}
}

func TestValidateConditionalTriggerWithInvalidConditionTypeFails(t *testing.T) {
	periodicTrigger := pb.Trigger_builder{
		Name: "PeriodicTriggerName",
		PeriodicTrigger: pb.PeriodicTrigger_builder{
			Interval: durationpb.New(100 * time.Millisecond),
		}.Build(),
	}.Build()

	testCases := []struct {
		name      string
		trigger   *pb.Trigger
		wantError *mcgerrors.StatusError
	}{
		{
			name: "negativeMinDuration",
			trigger: pb.Trigger_builder{
				Name: "ConditionalTriggerName",
				ConditionalTrigger: pb.ConditionalTrigger_builder{
					TriggerNames:      []string{periodicTrigger.GetName()},
					SelectorNodeIndex: new(uint32),
					RisingEdge: pb.ConditionalTrigger_ConditionTypeRisingEdge_builder{
						RisingOptions: pb.ConditionalTrigger_EdgeOptions_builder{
							MinDuration: durationpb.New(time.Duration(-3)),
						}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
			wantError: mcgerrors.ConditionalTriggerConditionTypeInvalidEdgeOptions(fmt.Errorf("min_duration must be >= 0, but is -3ns")),
		},
		{
			name: "noConditionType",
			trigger: pb.Trigger_builder{
				Name: "ConditionalTriggerName",
				ConditionalTrigger: pb.ConditionalTrigger_builder{
					TriggerNames:      []string{periodicTrigger.GetName()},
					SelectorNodeIndex: new(uint32),
				}.Build(),
			}.Build(),
			wantError: mcgerrors.ConditionalTriggerConditionTypeMissing,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v := validators.NewMcValidator(pb.MetricsConfig_builder{
				Triggers: []*pb.Trigger{tc.trigger, periodicTrigger},
			}.Build(), false)

			validators.ValidateTriggers(v)
			want, got := []*mcgerrors.StatusError{tc.wantError}, v.ErrorList
			if diff := cmp.Diff(
				want,
				got,
				cmp.Comparer(func(a, b *mcgerrors.StatusError) bool { return a.Status.Message == b.Status.Message }),
			); diff != "" {
				t.Errorf("Expected errors do not match: (-want +got)\n%s", diff)
			}
		})
	}
}

func TestValidateAggregatorWithInvalidTriggerReferencesFails(t *testing.T) {
	invalidParentTriggerName := "InvalidParentTriggerName"

	aggregator := pb.Source_builder{
		Name: "AggregatorName",
		Aggregator: pb.Aggregator_builder{
			TriggerNames: []string{invalidParentTriggerName},
			ResetOnGet:   true,
			MessageBuilder: pb.ProtoMessageBuilder_builder{
				MessageType: ".google.protobuf.Timestamp",
			}.Build(),
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Sources: []*pb.Source{aggregator},
	}.Build(), false)

	validators.ValidateSources(v)

	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.AggregatorParentTriggerNameNotMatchingAnyExistingTrigger(invalidParentTriggerName).Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation should not pass with aggregator referring to a non-existent trigger")
	}
}

func TestValidateSourcesWithNamingCollisionFails(t *testing.T) {
	dataSource := pb.Source_builder{
		Name: "Name",
		DataSource: pb.DataSource_builder{
			SourceIdentifier: "SourceIdentifier",
		}.Build(),
	}.Build()
	trigger := pb.Trigger_builder{
		Name: "Name",
		PeriodicTrigger: pb.PeriodicTrigger_builder{
			Interval: durationpb.New(100 * time.Millisecond),
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Sources:  []*pb.Source{dataSource},
		Triggers: []*pb.Trigger{trigger},
	}.Build(), false)

	validators.ValidateSources(v)

	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.TriggerNameCollisionWithSourceName(dataSource.GetName()).Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation should not pass with source which has the same name as a trigger")
	}
}

func TestValidateHasStartTriggerIfHasStopTriggerFails(t *testing.T) {
	trigger := pb.Trigger_builder{
		Name: "TriggerName",
		PeriodicTrigger: pb.PeriodicTrigger_builder{
			Interval: durationpb.New(100 * time.Millisecond),
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Triggers:        []*pb.Trigger{trigger},
		StopTriggerName: "TriggerName",
	}.Build(), false)

	validators.ValidateHasStartTriggerIfHasEndTrigger(v)

	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.StopTriggerSetWithoutStartTrigger.Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation should not pass with stop trigger set without start trigger set.")
	}
}

func TestValidateLifeCycleTriggersReferToExistingTriggersWithInvalidTriggerReferencesFails(t *testing.T) {
	type TestCase struct {
		error *mcgerrors.StatusError
		mc    *pb.MetricsConfig
	}
	for _, testCase := range []TestCase{
		{error: mcgerrors.StartTriggerNameNotMatchingAnyExistingTrigger("StartTriggerName"), mc: pb.MetricsConfig_builder{StartTriggerName: "StartTriggerName"}.Build()},
		{error: mcgerrors.StopTriggerNameNotMatchingAnyExistingTrigger("EndTriggerName"), mc: pb.MetricsConfig_builder{StopTriggerName: "EndTriggerName"}.Build()},
		{error: mcgerrors.DeactivateTriggerNameNotMatchingAnyExistingTrigger("FinishTriggerName"), mc: pb.MetricsConfig_builder{DeactivateTriggerName: "FinishTriggerName"}.Build()},
	} {
		v := validators.NewMcValidator(testCase.mc, false)

		validators.ValidateLifeCycleTriggersReferToExistingTriggers(v)

		if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != testCase.error.Status.Message {
			validators.PrintErrorList(v)
			t.Fatal("Validation should not pass with invalid trigger references as start / stop / deactivate triggers.")
		}
	}
}

func TestValidateMetricsReportConfigsWithDuplicateNamesFails(t *testing.T) {
	trigger := pb.Trigger_builder{
		Name: "TriggerName",
		PeriodicTrigger: pb.PeriodicTrigger_builder{
			Interval: durationpb.New(100 * time.Millisecond),
		}.Build(),
	}.Build()
	mrc := pb.MetricsReportConfig_builder{
		Name:             "MetricsReportConfigName",
		TriggerNames:     []string{"TriggerName"},
		ReportIncomplete: true,
		MessageBuilder: pb.ProtoMessageBuilder_builder{
			MessageType: ".google.protobuf.Type",
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Triggers:             []*pb.Trigger{trigger},
		MetricsReportConfigs: []*pb.MetricsReportConfig{mrc, mrc},
	}.Build(), false)

	validators.ValidateMetricsReportConfigs(v)

	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.DuplicateMetricsReportName(mrc.GetName()).Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation should not pass with duplicate metric report config names")
	}
}

func TestValidateMetricsReportConfigsWithNonExistentTriggerReferenceFails(t *testing.T) {
	mrc := pb.MetricsReportConfig_builder{
		Name:             "MetricsReportConfigName1",
		TriggerNames:     []string{"TriggerName"}, // non-existent trigger
		ReportIncomplete: true,
		MessageBuilder: pb.ProtoMessageBuilder_builder{
			MessageType: ".google.protobuf.Empty",
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		MetricsReportConfigs: []*pb.MetricsReportConfig{mrc},
	}.Build(), false)

	validators.ValidateMetricsReportConfigs(v)

	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.MetricsReportConfigWithInvalidTriggerReference(mrc.GetName()).Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation should not pass with a reference to non-existent trigger")
	}
}

func TestValidateMetricsReportConfigsWithInvalidExpressionNodeReferenceFails(t *testing.T) {
	trigger := pb.Trigger_builder{
		Name: "TriggerName",
		PeriodicTrigger: pb.PeriodicTrigger_builder{
			Interval: durationpb.New(100 * time.Millisecond),
		}.Build(),
	}.Build()
	mrc := pb.MetricsReportConfig_builder{
		Name:             "MetricsReportConfigName",
		TriggerNames:     []string{"TriggerName"},
		ReportIncomplete: true,
		MessageBuilder: pb.ProtoMessageBuilder_builder{
			MessageType: ".google.protobuf.BytesValue",
			FieldAssignments: []*pb.ProtoMessageBuilder_FieldAssignment{
				pb.ProtoMessageBuilder_FieldAssignment_builder{
					FieldName: "field_1",
					MaxAggregation: pb.ProtoMessageBuilder_FieldAssignment_MaxAggregation_builder{
						ExpressionNodeIndex: new(uint32),
					}.Build(),
				}.Build(),
			},
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Triggers:             []*pb.Trigger{trigger},
		MetricsReportConfigs: []*pb.MetricsReportConfig{mrc},
	}.Build(), false)

	validators.ValidateMetricsReportConfigs(v)

	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.MetricsReportConfigWithInvalidExpressionNodeReference(mrc.GetName(), mrc.GetMessageBuilder().GetFieldAssignments()[0].GetFieldName(), 0).Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation should not pass with invalid expression node reference from the metric report config")
	}
}

func TestAggregatorWithValidExpressionNodeReferencePasses(t *testing.T) {
	var aggregatorName string = "AggregatorName"

	node := pb.Node_builder{
		FieldLeafNode: pb.FieldLeafNode_builder{
			SourceName: aggregatorName,
			FieldNames: []string{"field_1"},
		}.Build(),
	}.Build()
	periodicTrigger := pb.Trigger_builder{
		Name: "PeriodicTriggerName",
		PeriodicTrigger: pb.PeriodicTrigger_builder{
			Interval: durationpb.New(100 * time.Millisecond),
		}.Build(),
	}.Build()
	maxAggregator := pb.Source_builder{
		Name: aggregatorName,
		Aggregator: pb.Aggregator_builder{
			TriggerNames: []string{periodicTrigger.GetName()},
			ResetOnGet:   true,
			MessageBuilder: pb.ProtoMessageBuilder_builder{
				MessageType: ".google.protobuf.DoubleValue",
				FieldAssignments: []*pb.ProtoMessageBuilder_FieldAssignment{
					pb.ProtoMessageBuilder_FieldAssignment_builder{
						FieldName: "field_1",
						MaxAggregation: pb.ProtoMessageBuilder_FieldAssignment_MaxAggregation_builder{
							ExpressionNodeIndex: new(uint32),
						}.Build(),
					}.Build(),
				},
			}.Build(),
		}.Build(),
	}.Build()
	countAggregator := pb.Source_builder{
		Name: "AggregatorName2",
		Aggregator: pb.Aggregator_builder{
			TriggerNames: []string{periodicTrigger.GetName()},
			ResetOnGet:   true,
			MessageBuilder: pb.ProtoMessageBuilder_builder{
				MessageType: ".google.protobuf.FloatValue",
				FieldAssignments: []*pb.ProtoMessageBuilder_FieldAssignment{
					pb.ProtoMessageBuilder_FieldAssignment_builder{
						FieldName:        "field_1",
						CountAggregation: pb.ProtoMessageBuilder_FieldAssignment_CountAggregation_builder{}.Build(),
					}.Build(),
				},
			}.Build(),
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		ExpressionNodes: []*pb.Node{node},
		Sources:         []*pb.Source{maxAggregator, countAggregator},
		Triggers:        []*pb.Trigger{periodicTrigger},
	}.Build(), false)

	validators.ValidateSources(v)
	if len(v.ErrorList) != 0 {
		validators.PrintErrorList(v)
		t.Fatal("Validation of aggregators should pass when they refer to existing expression nodes.")
	}
}

func TestAggregatorWithInvalidExpressionNodeReferenceFails(t *testing.T) {
	var aggregatorName string = "AggregatorName"

	periodicTrigger := pb.Trigger_builder{
		Name: "PeriodicTriggerName",
		PeriodicTrigger: pb.PeriodicTrigger_builder{
			Interval: durationpb.New(100 * time.Millisecond),
		}.Build(),
	}.Build()
	aggregator := pb.Source_builder{
		Name: aggregatorName,
		Aggregator: pb.Aggregator_builder{
			TriggerNames: []string{periodicTrigger.GetName()},
			ResetOnGet:   true,
			MessageBuilder: pb.ProtoMessageBuilder_builder{
				MessageType: ".google.protobuf.Int64Value",
				FieldAssignments: []*pb.ProtoMessageBuilder_FieldAssignment{
					pb.ProtoMessageBuilder_FieldAssignment_builder{
						FieldName: "field_1",
						MaxAggregation: pb.ProtoMessageBuilder_FieldAssignment_MaxAggregation_builder{
							ExpressionNodeIndex: new(uint32),
						}.Build(),
					}.Build(),
				},
			}.Build(),
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Sources:  []*pb.Source{aggregator},
		Triggers: []*pb.Trigger{periodicTrigger},
	}.Build(), false)

	validators.ValidateSources(v)
	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.AggregatorWithInvalidExpressionNodeReference(aggregatorName, 0).Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation of aggregators shouldn't pass when they refer to non-existent expression nodes.")
	}
}

func TestFieldAssExpressionNodeReferenceIsValidPasses(t *testing.T) {
	fieldAss := pb.ProtoMessageBuilder_FieldAssignment_builder{
		FieldName: "field_1",
		MaxAggregation: pb.ProtoMessageBuilder_FieldAssignment_MaxAggregation_builder{
			ExpressionNodeIndex: new(uint32),
		}.Build(),
	}.Build()

	for _, val := range []uint32{1, 2, 3, 4, 100} {
		if err := validators.FieldAssExpressionNodeReferenceIsValid(fieldAss, val, "sourceName"); err != nil {
			t.Fatal("FieldAssExpressionNodeReferenceIsValid shouldn't return an error when they refer to existing expression nodes.")
		}
	}
}

func TestFieldAssExpressionNodeReferenceIsValidFails(t *testing.T) {
	fieldAss := pb.ProtoMessageBuilder_FieldAssignment_builder{
		FieldName: "field_1",
		MaxAggregation: pb.ProtoMessageBuilder_FieldAssignment_MaxAggregation_builder{
			ExpressionNodeIndex: new(uint32),
		}.Build(),
	}.Build()

	if err := validators.FieldAssExpressionNodeReferenceIsValid(fieldAss, 0, "sourceName"); err == nil {
		t.Fatal("FieldAssExpressionNodeReferenceIsValid should fail when the field assignment refers to non-existent expression nodes.")
	}
}

func TestValidateMetricsReportConfigsWithUnknownMessageTypeFails(t *testing.T) {
	unknownMsgType := ".google.protobuf.DoesntExist"

	trigger := pb.Trigger_builder{
		Name: "TriggerName",
		PeriodicTrigger: pb.PeriodicTrigger_builder{
			Interval: durationpb.New(100 * time.Millisecond),
		}.Build(),
	}.Build()
	mrc := pb.MetricsReportConfig_builder{
		Name:             "MetricsReportConfigName",
		TriggerNames:     []string{"TriggerName"},
		ReportIncomplete: true,
		MessageBuilder: pb.ProtoMessageBuilder_builder{
			MessageType: unknownMsgType,
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Triggers:             []*pb.Trigger{trigger},
		MetricsReportConfigs: []*pb.MetricsReportConfig{mrc},
	}.Build(), false)

	validators.ValidateMetricsReportConfigs(v)

	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.UnknownMessageType(unknownMsgType).Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation should not pass with unknown message type in metric report config.")
	}
}

func TestValidateAggregatorWithUnknownMessageTypeFails(t *testing.T) {
	unknownMsgType := ".google.protobuf.DoesntExist"

	trigger := pb.Trigger_builder{
		Name: "TriggerName",
		PeriodicTrigger: pb.PeriodicTrigger_builder{
			Interval: durationpb.New(100 * time.Millisecond),
		}.Build(),
	}.Build()
	aggregator := pb.Source_builder{
		Name: "AggregatorName",
		Aggregator: pb.Aggregator_builder{
			TriggerNames: []string{trigger.GetName()},
			ResetOnGet:   true,
			MessageBuilder: pb.ProtoMessageBuilder_builder{
				MessageType: unknownMsgType,
			}.Build(),
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Triggers: []*pb.Trigger{trigger},
		Sources:  []*pb.Source{aggregator},
	}.Build(), false)

	validators.ValidateSources(v)

	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.UnknownMessageType(unknownMsgType).Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation should not pass with unknown message type in an aggregator.")
	}
}

func TestValidateDataTriggerWithOnDemandTypeDataSourceFails(t *testing.T) {
	dataSource := pb.Source_builder{
		Name: "DataSourceName",
		DataSource: pb.DataSource_builder{
			SourceIdentifier: "SourceIdentifier",
			ConnectionType:   pb.DataSource_ON_DEMAND,
		}.Build(),
	}.Build()
	dataTrigger := pb.Trigger_builder{
		Name: "TriggerName",
		DataTrigger: pb.DataTrigger_builder{
			SourceName: "DataSourceName",
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		Sources:  []*pb.Source{dataSource},
		Triggers: []*pb.Trigger{dataTrigger},
	}.Build(), false)

	validators.ValidateTriggers(v)

	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.DataTriggerUsingOnDemandDataSource.Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation should not pass with data trigger using data source with ON_DEMAND connection type.")
	}
}

func TestValidateFieldLeafNodeWithExpressionIndex(t *testing.T) {
	dataSource := pb.Source_builder{
		Name: "DataSourceName",
		DataSource: pb.DataSource_builder{
			SourceIdentifier: "SourceIdentifier",
		}.Build(),
	}.Build()

	// Node 0: Base FieldLeafNode
	node0 := pb.Node_builder{
		FieldLeafNode: pb.FieldLeafNode_builder{
			SourceName: "DataSourceName",
			FieldNames: []string{"field_1"},
		}.Build(),
	}.Build()

	t.Run("valid postfix validation passes", func(t *testing.T) {
		// Node 1: Valid Postfix FieldLeafNode pointing to Node 0
		node1 := pb.Node_builder{
			FieldLeafNode: pb.FieldLeafNode_builder{
				ExpressionNodeIndex: proto.Uint32(0),
				FieldNames:          []string{"field_2"},
			}.Build(),
		}.Build()

		v := validators.NewMcValidator(pb.MetricsConfig_builder{
			ExpressionNodes: []*pb.Node{node0, node1},
			Sources:         []*pb.Source{dataSource},
		}.Build(), false)

		validators.ValidateExpressionNodes(v)
		if len(v.ErrorList) != 0 {
			validators.PrintErrorList(v)
			t.Fatal("Validation should pass for valid postfix FieldLeafNode")
		}
	})

	t.Run("invalid expression index fails", func(t *testing.T) {
		// Node 2: Invalid Postfix FieldLeafNode pointing to out-of-bounds index (3)
		node2 := pb.Node_builder{
			FieldLeafNode: pb.FieldLeafNode_builder{
				ExpressionNodeIndex: proto.Uint32(3),
				FieldNames:          []string{"field_3"},
			}.Build(),
		}.Build()

		v := validators.NewMcValidator(pb.MetricsConfig_builder{
			ExpressionNodes: []*pb.Node{node0, node2},
			Sources:         []*pb.Source{dataSource},
		}.Build(), false)

		validators.ValidateExpressionNodes(v)
		if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.FieldLeafNodeWithInvalidExpressionNodeReference(3).Status.Message {
			validators.PrintErrorList(v)
			t.Fatal("Validation should fail for out-of-bounds ExpressionNodeIndex")
		}
	})

	t.Run("both source and expression index set fails", func(t *testing.T) {
		// Node 3: Invalid: Both SourceName and ExpressionNodeIndex set
		node3 := pb.Node_builder{
			FieldLeafNode: pb.FieldLeafNode_builder{
				SourceName:          "DataSourceName",
				ExpressionNodeIndex: proto.Uint32(0),
				FieldNames:          []string{"field_4"},
			}.Build(),
		}.Build()

		v := validators.NewMcValidator(pb.MetricsConfig_builder{
			ExpressionNodes: []*pb.Node{node0, node3},
			Sources:         []*pb.Source{dataSource},
		}.Build(), false)

		validators.ValidateExpressionNodes(v)
		if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.FieldLeafNodeWithBothSourceAndExpressionIndexSet(1).Status.Message {
			validators.PrintErrorList(v)
			t.Fatal("Validation should fail when both SourceName and ExpressionNodeIndex are set")
		}
	})

	t.Run("neither source nor expression index set fails", func(t *testing.T) {
		// Node 4: Invalid: Neither set
		node4 := pb.Node_builder{
			FieldLeafNode: pb.FieldLeafNode_builder{
				FieldNames: []string{"field_5"},
			}.Build(),
		}.Build()

		v := validators.NewMcValidator(pb.MetricsConfig_builder{
			ExpressionNodes: []*pb.Node{node4},
			Sources:         []*pb.Source{dataSource},
		}.Build(), false)

		validators.ValidateExpressionNodes(v)
		if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.FieldLeafNodeWithNeitherSourceNorExpressionIndexSet(0).Status.Message {
			validators.PrintErrorList(v)
			t.Fatal("Validation should fail when neither SourceName nor ExpressionNodeIndex are set")
		}
	})
}

func TestValidateExpressionNodesWithExpressionIndexCycleFails(t *testing.T) {
	// Node 0: FieldLeafNode pointing to Node 1
	node0 := pb.Node_builder{
		FieldLeafNode: pb.FieldLeafNode_builder{
			ExpressionNodeIndex: proto.Uint32(1),
			FieldNames:          []string{"field_1"},
		}.Build(),
	}.Build()

	// Node 1: CombinationNode pointing to Node 0 (Unary)
	node1 := pb.Node_builder{
		CombinationNode: pb.CombinationNode_builder{
			LeftIndex:        proto.Uint32(0),
			RoundingOperator: pb.CombinationNode_FLOOR.Enum(),
		}.Build(),
	}.Build()

	v := validators.NewMcValidator(pb.MetricsConfig_builder{
		ExpressionNodes: []*pb.Node{node0, node1},
	}.Build(), false)

	validators.ValidateExpressionNodes(v)
	if len(v.ErrorList) != 1 || v.ErrorList[0].Status.Message != mcgerrors.CyclicDependency("expression_nodes[0]").Status.Message {
		validators.PrintErrorList(v)
		t.Fatal("Validation should fail if there is a cycle involving expression_node_index")
	}
}
