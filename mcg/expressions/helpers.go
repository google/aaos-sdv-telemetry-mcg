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

package expressions

import (
	"fmt"

	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

func ExtractNodeIndex(fa *pb.ProtoMessageBuilder_FieldAssignment) (uint32, bool) {
	switch fa.WhichAggregatedFieldValue() {
	case pb.ProtoMessageBuilder_FieldAssignment_AvgAggregation_case:
		return fa.GetAvgAggregation().GetExpressionNodeIndex(), true
	case pb.ProtoMessageBuilder_FieldAssignment_CountAggregation_case:
		return 0xFFFFFFFF, false
	case pb.ProtoMessageBuilder_FieldAssignment_DeltaAggregation_case:
		return fa.GetDeltaAggregation().GetExpressionNodeIndex(), true
	case pb.ProtoMessageBuilder_FieldAssignment_MaxAggregation_case:
		return fa.GetMaxAggregation().GetExpressionNodeIndex(), true
	case pb.ProtoMessageBuilder_FieldAssignment_MinAggregation_case:
		return fa.GetMinAggregation().GetExpressionNodeIndex(), true
	case pb.ProtoMessageBuilder_FieldAssignment_NoAggregation_case:
		return fa.GetNoAggregation().GetExpressionNodeIndex(), true
	case pb.ProtoMessageBuilder_FieldAssignment_StdDevAggregation_case:
		return fa.GetStdDevAggregation().GetExpressionNodeIndex(), true
	case pb.ProtoMessageBuilder_FieldAssignment_SumAggregation_case:
		return fa.GetSumAggregation().GetExpressionNodeIndex(), true
	case pb.ProtoMessageBuilder_FieldAssignment_VectorAggregation_case:
		return fa.GetVectorAggregation().GetExpressionNodeIndex(), true
	default:
		panic(fmt.Sprintf("new FieldAssignment kind: %s", fa.WhichAggregatedFieldValue().String()))
	}
}

func SetNodeIndex(fa *pb.ProtoMessageBuilder_FieldAssignment, nid *uint32) {
	switch fa.WhichAggregatedFieldValue() {
	case pb.ProtoMessageBuilder_FieldAssignment_AvgAggregation_case:
		fa.GetAvgAggregation().SetExpressionNodeIndex(*nid)
	case pb.ProtoMessageBuilder_FieldAssignment_CountAggregation_case:
		if !(nid == nil || *nid == 0xFFFFFFFF) {
			// Ignore the sentinel value returned by ExtractNodeIndex
			panic("Attempt to set the expression node of a CountAggregation")
		}
	case pb.ProtoMessageBuilder_FieldAssignment_DeltaAggregation_case:
		fa.GetDeltaAggregation().SetExpressionNodeIndex(*nid)
	case pb.ProtoMessageBuilder_FieldAssignment_MaxAggregation_case:
		fa.GetMaxAggregation().SetExpressionNodeIndex(*nid)
	case pb.ProtoMessageBuilder_FieldAssignment_MinAggregation_case:
		fa.GetMinAggregation().SetExpressionNodeIndex(*nid)
	case pb.ProtoMessageBuilder_FieldAssignment_NoAggregation_case:
		fa.GetNoAggregation().SetExpressionNodeIndex(*nid)
	case pb.ProtoMessageBuilder_FieldAssignment_StdDevAggregation_case:
		fa.GetStdDevAggregation().SetExpressionNodeIndex(*nid)
	case pb.ProtoMessageBuilder_FieldAssignment_SumAggregation_case:
		fa.GetSumAggregation().SetExpressionNodeIndex(*nid)
	case pb.ProtoMessageBuilder_FieldAssignment_VectorAggregation_case:
		fa.GetVectorAggregation().SetExpressionNodeIndex(*nid)
	default:
		panic(fmt.Sprintf("new FieldAssignment kind: %s", fa.WhichAggregatedFieldValue().String()))
	}
}

// TODO(b/350487318): Ensure unknown types will throw an error.
func IsUnaryOperator(m *pb.CombinationNode) bool {
	switch m.WhichOperator() {
	// go/keep-sorted start
	case pb.CombinationNode_ArithmeticOperator_case:
		switch m.GetArithmeticOperator() {
		case pb.CombinationNode_ABSOLUTE, pb.CombinationNode_UNARY_MINUS:
			return true
		case pb.CombinationNode_ADD, pb.CombinationNode_SUBTRACT, pb.CombinationNode_MULTIPLY, pb.CombinationNode_DIVIDE, pb.CombinationNode_MODULO_TRUNC, pb.CombinationNode_POWER:
			return false
		}
	case pb.CombinationNode_ListOperator_case:
		switch m.GetListOperator() {
		case pb.CombinationNode_LENGTH:
			return true
		}
	case pb.CombinationNode_LogicalOperator_case:
		switch m.GetLogicalOperator() {
		case pb.CombinationNode_AND, pb.CombinationNode_OR, pb.CombinationNode_XOR:
			return false
		case pb.CombinationNode_NOT:
			return true
		}
	case pb.CombinationNode_RelationalOperator_case:
		switch m.GetRelationalOperator() {
		case pb.CombinationNode_EQ, pb.CombinationNode_NOT_EQ, pb.CombinationNode_GT, pb.CombinationNode_GT_OR_EQ, pb.CombinationNode_LT, pb.CombinationNode_LT_OR_EQ, pb.CombinationNode_CONTAINS, pb.CombinationNode_DOES_NOT_CONTAIN, pb.CombinationNode_ALL_EQ:
			return false
		}
	case pb.CombinationNode_RoundingOperator_case:
		switch m.GetRoundingOperator() {
		case pb.CombinationNode_CEIL, pb.CombinationNode_FLOOR, pb.CombinationNode_ROUND:
			return true
		}
		// go/keep-sorted end
	}
	return false
}
