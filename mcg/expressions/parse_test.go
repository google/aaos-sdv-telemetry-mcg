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

package expressions_test

import (
	"fmt"
	"strings"
	"testing"
	"unicode"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"sdv.googlesource.com/mcg/mcg/expressions"
	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

func TestShuntParseEveryOperator(t *testing.T) {
	cases := []struct {
		name       string
		expression string
		// Left and right index must be nil, only the operation is checked
		expectOp *pb.CombinationNode
	}{
		{
			name:       "op_add",
			expression: "1 + 1",
			expectOp: pb.CombinationNode_builder{
				ArithmeticOperator: pb.CombinationNode_ADD.Enum(),
			}.Build(),
		},
		{
			name:       "op_subtract",
			expression: "1 - 1",
			expectOp: pb.CombinationNode_builder{
				ArithmeticOperator: pb.CombinationNode_SUBTRACT.Enum(),
			}.Build(),
		},
		{
			name:       "op_multiply",
			expression: "1 * 1",
			expectOp: pb.CombinationNode_builder{
				ArithmeticOperator: pb.CombinationNode_MULTIPLY.Enum(),
			}.Build(),
		},
		{
			name:       "op_divide",
			expression: "1 / 1",
			expectOp: pb.CombinationNode_builder{
				ArithmeticOperator: pb.CombinationNode_DIVIDE.Enum(),
			}.Build(),
		},
		{
			name:       "op_power",
			expression: "1 ** 1",
			expectOp: pb.CombinationNode_builder{
				ArithmeticOperator: pb.CombinationNode_POWER.Enum(),
			}.Build(),
		},
		{
			name:       "op_modulo",
			expression: "1 % 1",
			expectOp: pb.CombinationNode_builder{
				ArithmeticOperator: pb.CombinationNode_MODULO_TRUNC.Enum(),
			}.Build(),
		},
		{
			name:       "op_abs",
			expression: "abs(-4.5)",
			expectOp: pb.CombinationNode_builder{
				ArithmeticOperator: pb.CombinationNode_ABSOLUTE.Enum(),
			}.Build(),
		},
		{
			name:       "op_and",
			expression: "true && true",
			expectOp: pb.CombinationNode_builder{
				LogicalOperator: pb.CombinationNode_AND.Enum(),
			}.Build(),
		},
		{
			name:       "op_or",
			expression: "true || true",
			expectOp: pb.CombinationNode_builder{
				LogicalOperator: pb.CombinationNode_OR.Enum(),
			}.Build(),
		},
		{
			name:       "op_xor",
			expression: "true ^ true",
			expectOp: pb.CombinationNode_builder{
				LogicalOperator: pb.CombinationNode_XOR.Enum(),
			}.Build(),
		},
		{
			name:       "op_not",
			expression: "!true",
			expectOp: pb.CombinationNode_builder{
				LogicalOperator: pb.CombinationNode_NOT.Enum(),
			}.Build(),
		},
		{
			name:       "op_eq",
			expression: "1 == 1",
			expectOp: pb.CombinationNode_builder{
				RelationalOperator: pb.CombinationNode_EQ.Enum(),
			}.Build(),
		},
		{
			name:       "op_not_eq",
			expression: "1 != 1",
			expectOp: pb.CombinationNode_builder{
				RelationalOperator: pb.CombinationNode_NOT_EQ.Enum(),
			}.Build(),
		},
		{
			name:       "op_gt",
			expression: "1 > 1",
			expectOp: pb.CombinationNode_builder{
				RelationalOperator: pb.CombinationNode_GT.Enum(),
			}.Build(),
		},
		{
			name:       "op_gteq",
			expression: "1 >= 1",
			expectOp: pb.CombinationNode_builder{
				RelationalOperator: pb.CombinationNode_GT_OR_EQ.Enum(),
			}.Build(),
		},
		{
			name:       "op_lt",
			expression: "1 < 1",
			expectOp: pb.CombinationNode_builder{
				RelationalOperator: pb.CombinationNode_LT.Enum(),
			}.Build(),
		},
		{
			name:       "op_lt_eq",
			expression: "1 <= 1",
			expectOp: pb.CombinationNode_builder{
				RelationalOperator: pb.CombinationNode_LT_OR_EQ.Enum(),
			}.Build(),
		},
		{
			name:       "op_floor",
			expression: "floor(4.5)",
			expectOp: pb.CombinationNode_builder{
				RoundingOperator: pb.CombinationNode_FLOOR.Enum(),
			}.Build(),
		},
		{
			name:       "op_round",
			expression: "round(4.5)",
			expectOp: pb.CombinationNode_builder{
				RoundingOperator: pb.CombinationNode_ROUND.Enum(),
			}.Build(),
		},
		{
			name:       "op_ceil",
			expression: "ceil(4.5)",
			expectOp: pb.CombinationNode_builder{
				RoundingOperator: pb.CombinationNode_CEIL.Enum(),
			}.Build(),
		},
		{
			name:       "op_alleq",
			expression: "alleq(source.field_0, true)",
			expectOp: pb.CombinationNode_builder{
				RelationalOperator: pb.CombinationNode_EQ.Enum(),
			}.Build(),
		},
		{
			name:       "op_contains",
			expression: "contains(source.field_1, 2)",
			expectOp: pb.CombinationNode_builder{
				RelationalOperator: pb.CombinationNode_CONTAINS.Enum(),
			}.Build(),
		},
		{
			name:       "op_doesnotcontain",
			expression: "doesnotcontain(source.field_2, 4)",
			expectOp: pb.CombinationNode_builder{
				RelationalOperator: pb.CombinationNode_DOES_NOT_CONTAIN.Enum(),
			}.Build(),
		},
		{
			name:       "op_length",
			expression: "length(source.field_3)",
			expectOp: pb.CombinationNode_builder{
				ListOperator: pb.CombinationNode_LENGTH.Enum(),
			}.Build(),
		},
		{
			name:       "op_subscript",
			expression: "source.field_4[0]",
			expectOp: pb.CombinationNode_builder{
				ListOperator: pb.CombinationNode_SUBSCRIPT.Enum(),
			}.Build(),
		},
	}
	sess := make(map[uint32]expressions.Text)
	for idx, tc := range cases {
		sess[uint32(idx)] = expressions.Text{Uncompiled: tc.expression}
	}
	p := expressions.NewParserShunt()
	mapping, nodes, err := p.CompileAll(sess)
	if err != nil {
		t.Fatal(err)
	}
	for idx, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := nodes[mapping[uint32(idx)]]
			want, got := tc.expectOp, n.GetCombinationNode()
			if diff := cmp.Diff(want, got, protocmp.Transform(),
				protocmp.IgnoreFields(new(pb.CombinationNode), "left_index", "right_index"),
			); diff != "" {
				t.Errorf("Unexpected difference (-want +got):\n%s", diff)
			}
		})
	}
}

func TestShuntParseSource(t *testing.T) {
	sess := make(map[uint32]expressions.Text)
	sess[1] = expressions.Text{Uncompiled: "my_source"}
	sess[2] = expressions.Text{Uncompiled: "my_source.value"}
	sess[3] = expressions.Text{Uncompiled: "!my_source.subfield.data_present"}

	p := expressions.NewParserShunt()
	mapping, nodes, err := p.CompileAll(sess)
	if err != nil {
		t.Fatal(err)
	}

	got, want := nodes[mapping[1]], pb.Node_builder{
		FieldLeafNode: pb.FieldLeafNode_builder{
			SourceName: "my_source",
		}.Build(),
	}.Build()
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("Unexpected difference (-want +got):\n%s", diff)
	}

	got, want = nodes[mapping[2]], pb.Node_builder{
		FieldLeafNode: pb.FieldLeafNode_builder{
			SourceName: "my_source",
			FieldNames: []string{"value"},
		}.Build(),
	}.Build()
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("Unexpected difference (-want +got):\n%s", diff)
	}

	notNode := nodes[mapping[3]]
	pubNode := nodes[notNode.GetCombinationNode().GetLeftIndex()]
	got, want = pubNode, pb.Node_builder{
		FieldLeafNode: pb.FieldLeafNode_builder{
			SourceName: "my_source",
			FieldNames: []string{"subfield", "data_present"},
		}.Build(),
	}.Build()
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("Unexpected difference (-want +got):\n%s", diff)
	}
}

func equalOperators(a, b *pb.CombinationNode) bool {
	return cmp.Equal(a, b, protocmp.Transform(), protocmp.IgnoreFields(new(pb.CombinationNode), "left_index", "right_index"))
}

func TestShuntParsePrecedence(t *testing.T) {
	operatorInfo := []struct {
		name            string
		operator        string
		precedenceLevel int
		isUnary         bool
	}{
		{name: "or", operator: "||", precedenceLevel: 4},
		{name: "and", operator: "&&", precedenceLevel: 5},
		{name: "xor", operator: "^", precedenceLevel: 7},
		{name: "eq", operator: "==", precedenceLevel: 9},
		{name: "noteq", operator: "!=", precedenceLevel: 9},
		{name: "lessthan", operator: "<", precedenceLevel: 10},
		{name: "lesseq", operator: "<=", precedenceLevel: 10},
		{name: "greaterthan", operator: ">", precedenceLevel: 10},
		{name: "greatereq", operator: ">=", precedenceLevel: 10},
		{name: "add", operator: "+", precedenceLevel: 12},
		{name: "subtract", operator: "-", precedenceLevel: 12},
		{name: "multiply", operator: "*", precedenceLevel: 13},
		{name: "divide", operator: "/", precedenceLevel: 13},
		{name: "modulo", operator: "%", precedenceLevel: 13},
		{name: "power", operator: "**", precedenceLevel: 15},
	}
	unaryOperatorInfo := []struct {
		name            string
		operator        string
		precedenceLevel int
	}{
		{name: "logicalnot", operator: "!", precedenceLevel: 14},
		{name: "unaryminus", operator: "-", precedenceLevel: 12},
	}
	for outerIdx, opInfoA := range operatorInfo {
		for _, opInfoB := range operatorInfo[outerIdx+1:] {
			t.Run(fmt.Sprint(opInfoA.name, "_vs_", opInfoB.name), func(t *testing.T) {
				sess := make(map[uint32]expressions.Text)
				sess[1] = expressions.Text{Uncompiled: fmt.Sprintf(
					"1 %s 1 %s 1", opInfoA.operator, opInfoB.operator,
				)}
				sess[2] = expressions.Text{Uncompiled: fmt.Sprintf(
					"1 %s 1 %s 1", opInfoB.operator, opInfoA.operator,
				)}
				sess[3] = expressions.Text{Uncompiled: fmt.Sprintf(
					"(1 %s 1) %s 1", opInfoA.operator, opInfoB.operator,
				)}
				// (!) Not swapped like test case 2
				sess[4] = expressions.Text{Uncompiled: fmt.Sprintf(
					"1 %s (1 %s 1)", opInfoA.operator, opInfoB.operator,
				)}
				p := expressions.NewParserShunt()

				mapping, nodes, err := p.CompileAll(sess)
				if err != nil {
					t.Fatal(err)
				}
				outerOpAFirst := nodes[mapping[1]].GetCombinationNode()
				outerOpBFirst := nodes[mapping[2]].GetCombinationNode()
				outerOpBAlways := nodes[mapping[3]].GetCombinationNode()
				outerOpAAlways := nodes[mapping[4]].GetCombinationNode()
				precedenceDifferent := equalOperators(outerOpAFirst, outerOpBFirst)
				selectedOperationA := equalOperators(outerOpAFirst, outerOpAAlways)
				selectedOperationB := equalOperators(outerOpAFirst, outerOpBAlways)
				selectedOperation2A := equalOperators(outerOpBFirst, outerOpAAlways)
				selectedOperation2B := equalOperators(outerOpBFirst, outerOpBAlways)
				if !((selectedOperationA != selectedOperationB) && (selectedOperation2A != selectedOperation2B)) {
					t.Fatal("logic assert failed (operator not one of the two choices)")
				}
				if want, got := (opInfoA.precedenceLevel != opInfoB.precedenceLevel), precedenceDifferent; want != got {
					t.Errorf("wrong precedence-different result (want %v, got %v)", want, got)
				}
				if precedenceDifferent {
					if want, got := (opInfoA.precedenceLevel > opInfoB.precedenceLevel), selectedOperationB; want != got {
						t.Errorf("wrong precedence order (want A>B=%v, got %v)", want, got)
					}
				}
				t.Logf("precedenceDifferent=%v selectedOperationA=%v selectedOperationA!=2A=%v", precedenceDifferent, selectedOperationA, selectedOperationA != selectedOperation2A)
				t.Logf("wantPrecedenceDifferent=%v wantPrecedenceAHigher=%v", opInfoA.precedenceLevel != opInfoB.precedenceLevel, (opInfoA.precedenceLevel > opInfoB.precedenceLevel))
			})
		}
		for _, opInfoU := range unaryOperatorInfo {
			t.Run(fmt.Sprint(opInfoA.name, "_vs_", opInfoU.name), func(t *testing.T) {
				sess := make(map[uint32]expressions.Text)
				sess[1] = expressions.Text{Uncompiled: fmt.Sprintf(
					"%s abs(1) %s 2", opInfoU.operator, opInfoA.operator,
				)}
				sess[2] = expressions.Text{Uncompiled: fmt.Sprintf(
					"(%s abs(1)) %s 2", opInfoU.operator, opInfoA.operator,
				)}
				sess[3] = expressions.Text{Uncompiled: fmt.Sprintf(
					"%s (abs(1) %s 2)", opInfoU.operator, opInfoA.operator,
				)}
				p := expressions.NewParserShunt()
				mapping, nodes, err := p.CompileAll(sess)
				if err != nil {
					t.Fatal(err)
				}
				outerOpTest := nodes[mapping[1]].GetCombinationNode()
				outerOpUAlways := nodes[mapping[2]].GetCombinationNode()
				outerOpAAlways := nodes[mapping[3]].GetCombinationNode()
				selectedOperationU := equalOperators(outerOpTest, outerOpUAlways)
				selectedOperationA := equalOperators(outerOpTest, outerOpAAlways)

				if selectedOperationA == selectedOperationU {
					t.Fatal("logic assert failed (expected oneof, got either or both)")
				}
				if want, got := (opInfoA.precedenceLevel <= opInfoU.precedenceLevel), selectedOperationU; want != got {
					t.Errorf("wrong precedence order (want A <= U = %v, got %v)", want, got)
				}
			})
		}
	}
}

func TestShuntParseWhitespace(t *testing.T) {
	var allWhiteSpace []rune = []rune("1 +")
	for _, r16 := range unicode.White_Space.R16 {
		for i := r16.Lo; i <= r16.Hi; i += r16.Stride {
			allWhiteSpace = append(allWhiteSpace, rune(i))
		}
	}
	for _, r32 := range unicode.White_Space.R32 {
		for i := r32.Lo; i <= r32.Hi; i += r32.Stride {
			allWhiteSpace = append(allWhiteSpace, rune(i))
		}
	}
	allWhiteSpace = append(allWhiteSpace, '1')
	sess := make(map[uint32]expressions.Text)
	sess[1] = expressions.Text{Uncompiled: string(allWhiteSpace)}

	p := expressions.NewParserShunt()
	if _, _, err := p.CompileAll(sess); err != nil {
		t.Error(err)
	}
}

func TestShuntParseSingleExpressionSuccess(t *testing.T) {
	cases := []struct {
		name       string
		expression string
		expectRoot uint32
		expect     []*pb.Node
	}{
		{
			name:       "test_parse_mixed_leaf_node_and_operator_types",
			expression: "source.field_1 / 3 != 100 && source.field_2",
			expectRoot: 6,
			/*
				And(
					NotEq(
						Div(
							Field(source.index_1) {1}
							3 {2}
						) {3}
						100 {4}
					) {5}
					Field(source.index_2) {6}
				) {7}
			*/
			expect: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
						FieldNames: []string{"field_1"},
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(3),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(0),
						RightIndex:         proto.Uint32(1),
						ArithmeticOperator: pb.CombinationNode_DIVIDE.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(100),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(2),
						RightIndex:         proto.Uint32(3),
						RelationalOperator: pb.CombinationNode_NOT_EQ.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
						FieldNames: []string{"field_2"},
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:       proto.Uint32(4),
						RightIndex:      proto.Uint32(5),
						LogicalOperator: pb.CombinationNode_AND.Enum(),
					}.Build(),
				}.Build(),
			},
		},
		{
			name: "test_parse_ambiguous_operators",
			// want to show:
			// * "-" is the sign of a number in this context, not the "subtract" operator
			// * ">=" is not pre-maturely parsed as ">"
			// * "!" is properly parsed as unary operator NOT
			expression: "-3 >= source.field_1 && !source.field_2",
			expectRoot: 5,
			/*
				And(
					GtEq(
						-3 {1}
						Field(source.field_1) {2}
					) {3}
					Not(
						Field(source.field_2) {4}
					) {5}
				) {6}
			*/
			expect: []*pb.Node{
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(-3),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
						FieldNames: []string{"field_1"},
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(0),
						RightIndex:         proto.Uint32(1),
						RelationalOperator: pb.CombinationNode_GT_OR_EQ.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
						FieldNames: []string{"field_2"},
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:       proto.Uint32(3),
						LogicalOperator: pb.CombinationNode_NOT.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:       proto.Uint32(2),
						RightIndex:      proto.Uint32(4),
						LogicalOperator: pb.CombinationNode_AND.Enum(),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "test_consecutive_negatives",
			expression: "4--4",
			expectRoot: 2,
			expect: []*pb.Node{
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(4),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(-4),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(0),
						RightIndex:         proto.Uint32(1),
						ArithmeticOperator: pb.CombinationNode_SUBTRACT.Enum(),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "test_floor_with_parenthesis_inside",
			expression: "floor((1/2)+5)",
			expectRoot: 5,
			expect: []*pb.Node{
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(1),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(2),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(0),
						RightIndex:         proto.Uint32(1),
						ArithmeticOperator: pb.CombinationNode_DIVIDE.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(5),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(2),
						RightIndex:         proto.Uint32(3),
						ArithmeticOperator: pb.CombinationNode_ADD.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:        proto.Uint32(4),
						RoundingOperator: pb.CombinationNode_FLOOR.Enum(),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "test_one_plus_floor",
			expression: "1+floor(4.5)",
			expectRoot: 3,
			expect: []*pb.Node{
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(1),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						FloatValue: proto.Float32(4.5),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:        proto.Uint32(1),
						RoundingOperator: pb.CombinationNode_FLOOR.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(0),
						RightIndex:         proto.Uint32(2),
						ArithmeticOperator: pb.CombinationNode_ADD.Enum(),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "test_unary_minus",
			expression: "-abs(-2)/-floor(4.5)",
			expectRoot: 6,
			expect: []*pb.Node{
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(-2),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(0),
						ArithmeticOperator: pb.CombinationNode_ABSOLUTE.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						FloatValue: proto.Float32(4.5),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:        proto.Uint32(2),
						RoundingOperator: pb.CombinationNode_FLOOR.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(3),
						ArithmeticOperator: pb.CombinationNode_UNARY_MINUS.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(1),
						RightIndex:         proto.Uint32(4),
						ArithmeticOperator: pb.CombinationNode_DIVIDE.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(5),
						ArithmeticOperator: pb.CombinationNode_UNARY_MINUS.Enum(),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "test_floor_with_additions_inside",
			expression: "floor(1.1+3.3+2.2)",
			expectRoot: 5,
			expect: []*pb.Node{
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						DoubleValue: proto.Float64(1.1),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						DoubleValue: proto.Float64(3.3),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(0),
						RightIndex:         proto.Uint32(1),
						ArithmeticOperator: pb.CombinationNode_ADD.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						DoubleValue: proto.Float64(2.2),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(2),
						RightIndex:         proto.Uint32(3),
						ArithmeticOperator: pb.CombinationNode_ADD.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:        proto.Uint32(4),
						RoundingOperator: pb.CombinationNode_FLOOR.Enum(),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "test_nested_rounding_operators",
			expression: "floor(round(ceil(1.23)))",
			expectRoot: 3,
			expect: []*pb.Node{
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						DoubleValue: proto.Float64(1.23),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:        proto.Uint32(0),
						RoundingOperator: pb.CombinationNode_CEIL.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:        proto.Uint32(1),
						RoundingOperator: pb.CombinationNode_ROUND.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:        proto.Uint32(2),
						RoundingOperator: pb.CombinationNode_FLOOR.Enum(),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "test_true_and_doesnotcontain",
			expression: "false && doesnotcontain(source.field_1, true)",
			expectRoot: 4,
			expect: []*pb.Node{
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						BoolValue: proto.Bool(false),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
						FieldNames: []string{"field_1"},
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						BoolValue: proto.Bool(true),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(1),
						RightIndex:         proto.Uint32(2),
						RelationalOperator: pb.CombinationNode_DOES_NOT_CONTAIN.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:       proto.Uint32(0),
						RightIndex:      proto.Uint32(3),
						LogicalOperator: pb.CombinationNode_AND.Enum(),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "test_contains_and_summed_value",
			expression: "contains(source.field_1, (1+5))",
			expectRoot: 4,
			expect: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
						FieldNames: []string{"field_1"},
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(1),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(5),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(1),
						RightIndex:         proto.Uint32(2),
						ArithmeticOperator: pb.CombinationNode_ADD.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(0),
						RightIndex:         proto.Uint32(3),
						RelationalOperator: pb.CombinationNode_CONTAINS.Enum(),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "test_timestamp_function",
			expression: "timestamp(REALTIME_CLOCK) + timestamp(MONOTONIC_TIME_SINCE_BOOT) + timestamp(MONOTONIC_TIME_SINCE_BOOT_OR_RESUME)",
			expectRoot: 4,
			expect: []*pb.Node{
				pb.Node_builder{
					FunctionLeafNode: pb.FunctionLeafNode_builder{
						GetCurrentTimestamp: pb.GetCurrentTimestampFunction_builder{
							TimeSource: pb.GetCurrentTimestampFunction_REALTIME_CLOCK,
						}.Build(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					FunctionLeafNode: pb.FunctionLeafNode_builder{
						GetCurrentTimestamp: pb.GetCurrentTimestampFunction_builder{
							TimeSource: pb.GetCurrentTimestampFunction_MONOTONIC_TIME_SINCE_BOOT,
						}.Build(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(0),
						RightIndex:         proto.Uint32(1),
						ArithmeticOperator: pb.CombinationNode_ADD.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					FunctionLeafNode: pb.FunctionLeafNode_builder{
						GetCurrentTimestamp: pb.GetCurrentTimestampFunction_builder{
							TimeSource: pb.GetCurrentTimestampFunction_MONOTONIC_TIME_SINCE_BOOT_OR_RESUME,
						}.Build(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(2),
						RightIndex:         proto.Uint32(3),
						ArithmeticOperator: pb.CombinationNode_ADD.Enum(),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "test_subscript_with_index_expression",
			expression: "source[length(source) / 2 + 5]",
			expectRoot: 6,
			expect: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:    proto.Uint32(0),
						ListOperator: pb.CombinationNode_LENGTH.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(2),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(1),
						RightIndex:         proto.Uint32(2),
						ArithmeticOperator: pb.CombinationNode_DIVIDE.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(5),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(3),
						RightIndex:         proto.Uint32(4),
						ArithmeticOperator: pb.CombinationNode_ADD.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:    proto.Uint32(0),
						RightIndex:   proto.Uint32(5),
						ListOperator: pb.CombinationNode_SUBSCRIPT.Enum(),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "test_subscript_negative_index",
			expression: "-source[-1]",
			expectRoot: 3,
			expect: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(-1),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:    proto.Uint32(0),
						RightIndex:   proto.Uint32(1),
						ListOperator: pb.CombinationNode_SUBSCRIPT.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(2),
						ArithmeticOperator: pb.CombinationNode_UNARY_MINUS.Enum(),
					}.Build(),
				}.Build(),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sess := make(map[uint32]expressions.Text)
			sess[1] = expressions.Text{Uncompiled: tc.expression}

			fmt.Println("start test", tc.name)
			p := expressions.NewParserShunt()
			mapping, nodes, err := p.CompileAll(sess)
			if err != nil {
				t.Fatal(err)
			}
			if tc.expectRoot != mapping[1] {
				t.Errorf("wrong root node, expected %d got %d", tc.expectRoot, mapping[1])
			}
			if len(tc.expect) != len(nodes) {
				t.Errorf("wrong node count, expected %d got %d", len(tc.expect), len(nodes))
			}
			for i := 1; i < len(tc.expect); i++ {
				got, want := nodes[i], tc.expect[i]
				if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
					t.Errorf("[%d] wrong node contents, difference (-want +got):\n%s", i, diff)
				}
			}
		})
	}
}

func TestShuntParseConstantLeafNodeTypes(t *testing.T) {
	cases := []struct {
		name       string
		expression string
		wantNode   func(*pb.ConstantLeafNode) bool
	}{
		{
			name:       "bool_true",
			expression: "true",
			wantNode:   func(n *pb.ConstantLeafNode) bool { return n.GetBoolValue() == true },
		},
		{
			name:       "bool_false",
			expression: "false",
			wantNode:   func(n *pb.ConstantLeafNode) bool { return n.GetBoolValue() == false },
		},
		{
			name:       "int32_small",
			expression: "123",
			wantNode:   func(n *pb.ConstantLeafNode) bool { return n.GetInt32Value() == 123 },
		},
		{
			name:       "int32_max",
			expression: "2147483647", // Max int32
			wantNode:   func(n *pb.ConstantLeafNode) bool { return n.GetInt32Value() == 2147483647 },
		},
		{
			name:       "int64_overflow_int32",
			expression: "2147483648", // Max int32 + 1
			wantNode:   func(n *pb.ConstantLeafNode) bool { return n.GetInt64Value() == 2147483648 },
		},
		{
			name:       "float32_exact_1_0",
			expression: "1.0",
			wantNode:   func(n *pb.ConstantLeafNode) bool { return n.GetFloatValue() == 1.0 },
		},
		{
			name:       "float32_exact_0_5",
			expression: "0.5",
			wantNode:   func(n *pb.ConstantLeafNode) bool { return n.GetFloatValue() == 0.5 },
		},
		{
			name:       "float32_exact_1_25",
			expression: "1.25",
			wantNode:   func(n *pb.ConstantLeafNode) bool { return n.GetFloatValue() == 1.25 },
		},
		{
			name:       "double_inexact_float32",
			expression: "1.23", // Not exactly representable by float32
			wantNode:   func(n *pb.ConstantLeafNode) bool { return n.GetDoubleValue() == 1.23 },
		},
		{
			name:       "double_large_precision",
			expression: "3.1415926535",
			wantNode:   func(n *pb.ConstantLeafNode) bool { return n.GetDoubleValue() == 3.1415926535 },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sess := make(map[uint32]expressions.Text)
			// Use an arbitrary key, as there's only one expression per test case.
			sess[0] = expressions.Text{Uncompiled: tc.expression}

			p := expressions.NewParserShunt()
			mapping, nodes, err := p.CompileAll(sess)
			if err != nil {
				t.Fatalf("p.CompileAll(sess) for expression %q failed: %v", tc.expression, err)
			}

			if len(nodes) != 1 {
				t.Fatalf("len(nodes) = %d, want %d", len(nodes), 1)
			}

			// For a single constant expression, the root node will be at index 0.
			rootNodeIdx := mapping[0]
			rootNode := nodes[rootNodeIdx]

			if constantNode := rootNode.GetConstantLeafNode(); constantNode == nil {
				t.Fatalf("rootNode.GetConstantLeafNode() for expression %q = nil, want non-nil. Node type was %v", tc.expression, rootNode.WhichNodeType())
			}

			constantNode := rootNode.GetConstantLeafNode()
			if got := tc.wantNode(constantNode); !got {
				t.Errorf("tc.wantNode(%v) for expression %q = %v, want %v", constantNode.WhichNodeValue(), tc.expression, got, true)
			}
		})
	}
}

func TestShuntParseReuseNodes(t *testing.T) {
	sess := make(map[uint32]expressions.Text)
	sess[1] = expressions.Text{Uncompiled: "(2+3*3)/2"}
	sess[2] = expressions.Text{Uncompiled: "3*3"}

	p := expressions.NewParserShunt()
	mapping, nodes, err := p.CompileAll(sess)
	if err != nil {
		t.Fatal(err)
	}

	divNode := nodes[mapping[1]]
	addNode := nodes[divNode.GetCombinationNode().GetLeftIndex()]
	mulNodeIdx := addNode.GetCombinationNode().GetRightIndex()
	if mulNodeIdx != mapping[2] {
		t.Log(mapping)
		t.Log(nodes)
		t.Error("Expression nodes not properly reused at root")
	}
}

// Testing Expression Errors with missing operands
func TestShuntParseMissingOperandError(t *testing.T) {
	cases := []struct {
		name        string
		expression  string
		expectError string
	}{
		{
			name:        "op_add_missing_right",
			expression:  "1 + ",
			expectError: "Failed to parse expression \"1 + \"",
		},
		{
			name:        "op_multiply_missing_left",
			expression:  "* 1",
			expectError: "Failed to parse expression \"* 1\"",
		},
		{
			name:        "op_unary_minus_missing",
			expression:  "- ",
			expectError: "Failed to parse expression \"- \"",
		},
	}

	for idx, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sess := make(map[uint32]expressions.Text)
			sess[uint32(idx)] = expressions.Text{Uncompiled: tc.expression}

			p := expressions.NewParserShunt()
			_, _, err := p.CompileAll(sess)
			if want, got := tc.expectError, err; err == nil || !strings.Contains(got.Error(), tc.expectError) {
				t.Errorf("p.CompileAll(sess) = _, _, %q, want containing %q", got, want)
			}
		})
	}
}

func TestShuntParseEdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		expression  string
		expectError string // Expected error message substring
	}{
		{
			name:        "empty_string",
			expression:  "",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"\": Found no valid operands",
		},
		{
			name:        "whitespace_only",
			expression:  "   \t\n",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"   \\t\\n\": Found no valid operands",
		},
		{
			name:        "single_operator",
			expression:  "+",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"+\": Missing operand(s) for binary operator: OperatorAdd",
		},
		{
			name:        "missing_closing_paren",
			expression:  "(1 + 2",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"(1 + 2\": Found opening parenthesis without matching closing parenthesis",
		},
		{
			name:        "missing_opening_paren",
			expression:  "1 + 2)",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"1 + 2)\": Found closing parenthesis without matching opening parenthesis",
		},
		{
			name:        "leading_closing_paren",
			expression:  ")1 + 2",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \")1 + 2\": Found closing parenthesis without matching opening parenthesis",
		},
		{
			name:        "empty_parentheses",
			expression:  "()",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"()\": Found redundant parenthesis",
		},
		{
			name:        "redundant_parentheses",
			expression:  "(10)",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"(10)\": Found redundant parenthesis",
		},
		{
			name:        "invalid_timestamp_source",
			expression:  "timestamp(UNKNOWN_SOURCE)",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"timestamp(UNKNOWN_SOURCE)\": \"UNKNOWN_SOURCE\" is not a valid timestamp parameter",
		},
		{
			name:        "invalid_operator",
			expression:  "++1",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"++1\": Unknown operator \"++\"",
		},
		{
			name:        "missing_operand_for_binary_operator",
			expression:  "1**",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"1**\": Missing operand(s) for binary operator: OperatorPower",
		},
		{
			name:        "invalid_unary_operator",
			expression:  "!!true",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"!!true\": Unknown operator \"!!\"",
		},
		{
			name:        "empty_function_operator",
			expression:  "abs()",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"abs()\": Missing operand for unary operator: OperatorAbsolute",
		},
		{
			name:        "missing_operands",
			expression:  "1 + -",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"1 + -\": Missing operand(s) for binary operator: OperatorAdd",
		},
		{
			name:        "empty_length",
			expression:  "length()",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"length()\": Missing operand for unary operator: OperatorLength",
		},
		{
			name:        "op_subscript_missing_index",
			expression:  "source[]",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"source[]\": Missing operand(s) for binary operator: OperatorSubscript",
		},
		{
			name:        "name_tbd",
			expression:  "true&&[5]",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"true&&[5]\": Missing operand(s) for binary operator: OperatorAnd",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := expressions.NewParserShunt()
			sess := make(map[uint32]expressions.Text)
			sess[1] = expressions.Text{Uncompiled: tc.expression}

			_, _, err := p.CompileAll(sess)
			if err == nil {
				t.Fatalf("Expected an error for expression %q, but got none", tc.expression)
			}
			if !strings.Contains(err.Error(), tc.expectError) {
				t.Errorf("For expression %q, expected error containing %q, but got %q", tc.expression, tc.expectError, err.Error())
			}
		})
	}
}
