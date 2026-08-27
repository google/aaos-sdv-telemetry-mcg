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
	"math"
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
	p := expressions.NewParserShunt(false)
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

	p := expressions.NewParserShunt(false)
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

func TestUnaryOperatorCombinations(t *testing.T) {
	validCases := []string{
		"--5",
		"-- 5",
		"---5",
		"--- 5",
		"----5",
		"---- 5",
		"- -5",
		"- - 5",
		"- - -5",
		"- - - 5",
		"-(-5)",
		"!-5",
		"! -5",
		"!!false",
		"!! false",
		"! !false",
		"! ! false",
		"!!!!!false",
		"-!true",
		"- !true",
		"! - ! - 5",
		"!-!-5",
		"!-! - 5",
		"! - ! -5",
		"- ! - ! 5",
		"-!-!5",
		"-abs(-5)",
		"-floor(4.5)",
		"!contains(my_source.val, 5)",
		"abs(abs(-5))",
		"floor(ceil(4.5))",
	}
	for _, expr := range validCases {
		t.Run("valid_"+expr, func(t *testing.T) {
			sess := map[uint32]expressions.Text{1: {Uncompiled: expr}}
			p := expressions.NewParserShunt(false)
			_, _, err := p.CompileAll(sess)
			if err != nil {
				t.Errorf("CompileAll(%q) failed unexpectedly: %v", expr, err)
			}
		})
	}
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
				p := expressions.NewParserShunt(false)

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
				p := expressions.NewParserShunt(false)
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

	p := expressions.NewParserShunt(false)
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
		{
			name:       "test_field_access_after_expression",
			expression: "(a + b).foo.bar",
			expectRoot: 3,
			expect: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "a",
					}.Build(),
				}.Build(),
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "b",
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
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName:          "",
						FieldNames:          []string{"foo", "bar"},
						ExpressionNodeIndex: proto.Uint32(2),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "nested_arrays_supported",
			expression: "source[0].x[1].y",
			expectRoot: 6,
			expect: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(0),
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
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName:          "",
						FieldNames:          []string{"x"},
						ExpressionNodeIndex: proto.Uint32(2),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(1),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:    proto.Uint32(3),
						RightIndex:   proto.Uint32(4),
						ListOperator: pb.CombinationNode_SUBSCRIPT.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName:          "",
						FieldNames:          []string{"y"},
						ExpressionNodeIndex: proto.Uint32(5),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "subscript_field_access_with_infinity",
			expression: "source[0].infinity",
			expectRoot: 3,
			expect: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(0),
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
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName:          "",
						FieldNames:          []string{"infinity"},
						ExpressionNodeIndex: proto.Uint32(2),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "subscript_field_access_with_true",
			expression: "source[0].true",
			expectRoot: 3,
			expect: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(0),
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
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName:          "",
						FieldNames:          []string{"true"},
						ExpressionNodeIndex: proto.Uint32(2),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "subscript_field_access_with_nan",
			expression: "source[0].nan",
			expectRoot: 3,
			expect: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(0),
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
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName:          "",
						FieldNames:          []string{"nan"},
						ExpressionNodeIndex: proto.Uint32(2),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "subscript_field_access_with_digit_prefix_ident",
			expression: "source[0].123abc",
			expectRoot: 3,
			expect: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(0),
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
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName:          "",
						FieldNames:          []string{"123abc"},
						ExpressionNodeIndex: proto.Uint32(2),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "subscript_field_access_with_infinity",
			expression: "source[0].infinity",
			expectRoot: 3,
			expect: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(0),
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
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName:          "",
						FieldNames:          []string{"infinity"},
						ExpressionNodeIndex: proto.Uint32(2),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "subscript_field_access_with_true",
			expression: "source[0].true",
			expectRoot: 3,
			expect: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(0),
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
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName:          "",
						FieldNames:          []string{"true"},
						ExpressionNodeIndex: proto.Uint32(2),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "subscript_field_access_with_nan",
			expression: "source[0].nan",
			expectRoot: 3,
			expect: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(0),
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
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName:          "",
						FieldNames:          []string{"nan"},
						ExpressionNodeIndex: proto.Uint32(2),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "test_floor_leading_dot",
			expression: "floor(.5)",
			expectRoot: 1,
			expect: []*pb.Node{
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						FloatValue: proto.Float32(0.5),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:        proto.Uint32(0),
						RoundingOperator: pb.CombinationNode_FLOOR.Enum(),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "test_ceil_leading_dot",
			expression: "ceil(.5)",
			expectRoot: 1,
			expect: []*pb.Node{
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						FloatValue: proto.Float32(0.5),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:        proto.Uint32(0),
						RoundingOperator: pb.CombinationNode_CEIL.Enum(),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "test_add_leading_dot",
			expression: ".25 + .75",
			expectRoot: 2,
			expect: []*pb.Node{
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						FloatValue: proto.Float32(0.25),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						FloatValue: proto.Float32(0.75),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(0),
						RightIndex:         proto.Uint32(1),
						ArithmeticOperator: pb.CombinationNode_ADD.Enum(),
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
			p := expressions.NewParserShunt(false)
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
			for i := 0; i < len(tc.expect); i++ {
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
			name:       "float32_leading_dot_0_5",
			expression: ".5",
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

			p := expressions.NewParserShunt(false)
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

	p := expressions.NewParserShunt(false)
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

			p := expressions.NewParserShunt(false)
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
			name:        "op_subscript_after_binary_op",
			expression:  "true&&[5]",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"true&&[5]\": Missing operand(s) for binary operator: OperatorAnd",
		},
		{
			name:        "postfix_field_access_without_expression",
			expression:  ".foo.bar.baz",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \".foo.bar.baz\": postfix field access must follow an expression",
		},
		{
			name:        "unparenthesized_floor_rejected",
			expression:  "floor 4.5",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"floor 4.5\": OperatorFloor requires parentheses",
		},
		{
			name:        "unparenthesized_round_rejected",
			expression:  "round 4.5",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"round 4.5\": OperatorRound requires parentheses",
		},
		{
			name:        "unparenthesized_ceil_rejected",
			expression:  "ceil 4.5",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"ceil 4.5\": OperatorCeil requires parentheses",
		},
		{
			name:        "unparenthesized_abs_rejected",
			expression:  "abs 2",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"abs 2\": OperatorAbsolute requires parentheses",
		},
		{
			name:        "unparenthesized_length_rejected",
			expression:  "length [1, 2]",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"length [1, 2]\": OperatorLength requires parentheses",
		},
		{
			name:        "unparenthesized_contains_rejected",
			expression:  "contains source.field 5",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"contains source.field 5\": OperatorContains requires parentheses",
		},
		{
			name:        "unparenthesized_doesnotcontain_rejected",
			expression:  "doesnotcontain source.field 5",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"doesnotcontain source.field 5\": OperatorDoesNotContain requires parentheses",
		},
		{
			name:        "unparenthesized_alleq_rejected",
			expression:  "alleq source.field true",
			expectError: "FAILED_PRECONDITION: Failed to parse expression \"alleq source.field true\": OperatorAllEq requires parentheses",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := expressions.NewParserShunt(false)
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

func TestShuntParseDisallowComparisonChaining(t *testing.T) {
	tests := []struct {
		name           string
		expression     string
		wantDisallowed bool
	}{
		// --- Blocked when enabled (strict mode), allowed when disabled ---
		{
			name:           "lt_lt",
			expression:     "a < b < c",
			wantDisallowed: true,
		},
		{
			name:           "lteq_lteq",
			expression:     "a <= b <= c",
			wantDisallowed: true,
		},
		{
			name:           "gt_gt",
			expression:     "a > b > c",
			wantDisallowed: true,
		},
		{
			name:           "gteq_gteq",
			expression:     "a >= b >= c",
			wantDisallowed: true,
		},
		{
			name:           "eq_eq",
			expression:     "a == b == c",
			wantDisallowed: true,
		},
		{
			name:           "noteq_noteq",
			expression:     "a != b != c",
			wantDisallowed: true,
		},
		{
			name:           "mixed_lt_lteq",
			expression:     "a < b <= c",
			wantDisallowed: true,
		},
		{
			name:           "mixed_eq_lt",
			expression:     "a == b < c",
			wantDisallowed: true,
		},
		{
			name:           "mixed_lt_eq",
			expression:     "a < b == c",
			wantDisallowed: true,
		},
		{
			name:           "mixed_gt_eq",
			expression:     "a > b == c",
			wantDisallowed: true,
		},
		{
			name:           "mixed_eq_noteq",
			expression:     "a == b != c",
			wantDisallowed: true,
		},
		{
			name:           "long_chain",
			expression:     "a < b < c < d",
			wantDisallowed: true,
		},
		{
			name:           "long_mixed_chain",
			expression:     "a < b <= c == d > e",
			wantDisallowed: true,
		},
		{
			name:           "chaining_with_arithmetic",
			expression:     "a + b < c < d * e",
			wantDisallowed: true,
		},
		{
			name:           "subscript_chaining",
			expression:     "a < b[0] < c",
			wantDisallowed: true,
		},
		{
			name:           "chaining_after_logical",
			expression:     "a < b && b < c < d",
			wantDisallowed: true,
		},
		{
			name:           "chaining_inside_function",
			expression:     "floor(a < b < c)",
			wantDisallowed: true,
		},
		{
			name:           "chaining_inside_not",
			expression:     "!(a < b < c)",
			wantDisallowed: true,
		},
		{
			name:           "whole_chain_parenthesized",
			expression:     "(a < b < c)",
			wantDisallowed: true,
		},
		{
			name:           "intra_expression_leak_protection",
			expression:     "(a < b) && a < b < c",
			wantDisallowed: true,
		},
		{
			name:           "intra_expression_leak_protection_reverse",
			expression:     "a < b < (a < b)",
			wantDisallowed: true,
		},

		// --- Allowed in both default and strict modes (explicit intent) ---
		{
			name:           "parenthesized_left",
			expression:     "(a < b) < c",
			wantDisallowed: false,
		},
		{
			name:           "subscript_parenthesized",
			expression:     "(a < b[0]) < c",
			wantDisallowed: false,
		},
		{
			name:           "parenthesized_right",
			expression:     "a < (b <= c)",
			wantDisallowed: false,
		},
		{
			name:           "parenthesized_logical",
			expression:     "(a < b) == (c < d)",
			wantDisallowed: false,
		},
		{
			name:           "logical_and",
			expression:     "a < b && b < c",
			wantDisallowed: false,
		},
		{
			name:           "function_like_contains",
			expression:     "contains(source.field, 5) == true",
			wantDisallowed: false,
		},
		{
			name:           "relation_inside_function_argument",
			expression:     "contains(a < b, source)",
			wantDisallowed: false,
		},
		{
			name:           "function_like_alleq",
			expression:     "alleq(source.field, true) == true",
			wantDisallowed: false,
		},
		{
			name:           "parenthesized_mixed",
			expression:     "(a == b) != c",
			wantDisallowed: false,
		},
		{
			name:           "function_and_relational",
			expression:     "contains(a, b) && c < d",
			wantDisallowed: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sess := map[uint32]expressions.Text{
				1: {Uncompiled: tc.expression},
			}

			// Test with chaining allowed (disallow = false)
			pAllowed := expressions.NewParserShunt(false)
			if _, _, err := pAllowed.CompileAll(sess); err != nil {
				t.Errorf("CompileAll(%q) with disallow=false unexpected error: %v", tc.expression, err)
			}

			// Test with chaining disallowed (disallow = true)
			pDisallowed := expressions.NewParserShunt(true)
			_, _, err := pDisallowed.CompileAll(sess)
			if tc.wantDisallowed {
				if err == nil {
					t.Fatalf("CompileAll(%q) with disallow=true expected error, got nil", tc.expression)
				}
				if !strings.Contains(err.Error(), "comparison operator chaining is disallowed") {
					t.Errorf("CompileAll(%q) with disallow=true expected error containing %q, got: %q", tc.expression, "comparison operator chaining is disallowed", err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("CompileAll(%q) with disallow=true unexpected error: %v", tc.expression, err)
				}
			}
		})
	}
}

func TestShuntParseDisallowComparisonChaining_InterExpressionLeak(t *testing.T) {
	// Compile multiple expressions in the same session.
	// Expr 1 is parenthesized: (a < b)
	// Expr 2 is chained: a < b < c
	// If there is inter-expression leakage, Expr 2 might incorrectly succeed
	// because the node index for "a < b" (deduplicated) was marked parenthesized in Expr 1.
	sess := map[uint32]expressions.Text{
		1: {Uncompiled: "(a < b)"},
		2: {Uncompiled: "a < b < c"},
	}
	p := expressions.NewParserShunt(true)
	_, _, err := p.CompileAll(sess)
	if err == nil {
		t.Fatalf("CompileAll(%v) expected error due to comparison operator chaining in expression 2, but got none", sess)
	}
	if !strings.Contains(err.Error(), "comparison operator chaining is disallowed") {
		t.Errorf("CompileAll(%v) expected error containing %q, got: %q", sess, "comparison operator chaining is disallowed", err.Error())
	}
}

func TestShuntParseNumberLimits(t *testing.T) {
	int32Leaf := func(v int32) *pb.Node {
		return pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{Int32Value: proto.Int32(v)}.Build()}.Build()
	}
	int64Leaf := func(v int64) *pb.Node {
		return pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{Int64Value: proto.Int64(v)}.Build()}.Build()
	}
	wrapInUnaryMinus := func(child *pb.Node) []*pb.Node {
		return []*pb.Node{
			child,
			pb.Node_builder{
				CombinationNode: pb.CombinationNode_builder{
					LeftIndex:          proto.Uint32(0),
					ArithmeticOperator: pb.CombinationNode_UNARY_MINUS.Enum(),
				}.Build(),
			}.Build(),
		}
	}

	cases := []struct {
		expression  string
		expectNodes []*pb.Node
		expectRoot  uint32
	}{
		// Single numbers
		{fmt.Sprintf("%d", math.MinInt32), []*pb.Node{int32Leaf(math.MinInt32)}, 0},
		{fmt.Sprintf("%d", math.MaxInt32), []*pb.Node{int32Leaf(math.MaxInt32)}, 0},
		{fmt.Sprintf("%d", math.MinInt64), []*pb.Node{int64Leaf(math.MinInt64)}, 0},
		{fmt.Sprintf("%d", math.MaxInt64), []*pb.Node{int64Leaf(math.MaxInt64)}, 0},

		// Negated negative limits (the unary minus operator on these will overflow).
		{fmt.Sprintf("-%d", math.MinInt32), wrapInUnaryMinus(int32Leaf(math.MinInt32)), 1},
		{fmt.Sprintf("-(%d)", math.MinInt32), wrapInUnaryMinus(int32Leaf(math.MinInt32)), 1},
		{fmt.Sprintf("-%d", math.MinInt64), wrapInUnaryMinus(int64Leaf(math.MinInt64)), 1},
		{fmt.Sprintf("-(%d)", math.MinInt64), wrapInUnaryMinus(int64Leaf(math.MinInt64)), 1},
	}

	for _, tc := range cases {
		t.Run(tc.expression, func(t *testing.T) {
			p := expressions.NewParserShunt(false)
			sess := map[uint32]expressions.Text{1: {Uncompiled: tc.expression}}
			rootIndices, nodes, err := p.CompileAll(sess)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rootIndices[1] != tc.expectRoot {
				t.Errorf("expected root index %d, got %d", tc.expectRoot, rootIndices[1])
			}
			if diff := cmp.Diff(tc.expectNodes, nodes, protocmp.Transform()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestShuntParseScientificNotationNotAccepted(t *testing.T) {
	// Scientific notation is not parsed as float literals, but instead as identifiers / field paths:
	t.Run("parsed_as_field_identifier", func(t *testing.T) {
		cases := []struct {
			name        string
			expression  string
			expectNodes []*pb.Node
			expectRoot  uint32
		}{
			{
				name:       "positive_scientific_integer_as_field",
				expression: "1e5",
				expectNodes: []*pb.Node{
					pb.Node_builder{
						FieldLeafNode: pb.FieldLeafNode_builder{
							SourceName: "1e5",
						}.Build(),
					}.Build(),
				},
				expectRoot: 0,
			},
			{
				name:       "positive_scientific_capital_e_as_field",
				expression: "1E5",
				expectNodes: []*pb.Node{
					pb.Node_builder{
						FieldLeafNode: pb.FieldLeafNode_builder{
							SourceName: "1E5",
						}.Build(),
					}.Build(),
				},
				expectRoot: 0,
			},
			{
				name:       "positive_scientific_float_as_nested_field",
				expression: "1.5e3",
				expectNodes: []*pb.Node{
					pb.Node_builder{
						FieldLeafNode: pb.FieldLeafNode_builder{
							SourceName: "1",
							FieldNames: []string{"5e3"},
						}.Build(),
					}.Build(),
				},
				expectRoot: 0,
			},
			{
				name:       "positive_scientific_negative_exponent_as_subtraction",
				expression: "1e-5",
				expectNodes: []*pb.Node{
					pb.Node_builder{
						FieldLeafNode: pb.FieldLeafNode_builder{
							SourceName: "1e",
						}.Build(),
					}.Build(),
					pb.Node_builder{
						ConstantLeafNode: pb.ConstantLeafNode_builder{
							Int32Value: proto.Int32(5),
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
				expectRoot: 2,
			},
			{
				name:       "positive_scientific_positive_exponent_as_addition",
				expression: "1e+5",
				expectNodes: []*pb.Node{
					pb.Node_builder{
						FieldLeafNode: pb.FieldLeafNode_builder{
							SourceName: "1e",
						}.Build(),
					}.Build(),
					pb.Node_builder{
						ConstantLeafNode: pb.ConstantLeafNode_builder{
							Int32Value: proto.Int32(5),
						}.Build(),
					}.Build(),
					pb.Node_builder{
						CombinationNode: pb.CombinationNode_builder{
							LeftIndex:          proto.Uint32(0),
							RightIndex:         proto.Uint32(1),
							ArithmeticOperator: pb.CombinationNode_ADD.Enum(),
						}.Build(),
					}.Build(),
				},
				expectRoot: 2,
			},
			{
				name:       "hex_float_integer_as_field",
				expression: "0x1p5",
				expectNodes: []*pb.Node{
					pb.Node_builder{
						FieldLeafNode: pb.FieldLeafNode_builder{
							SourceName: "0x1p5",
						}.Build(),
					}.Build(),
				},
				expectRoot: 0,
			},
			{
				name:       "hex_float_capital_p_as_field",
				expression: "0X1P5",
				expectNodes: []*pb.Node{
					pb.Node_builder{
						FieldLeafNode: pb.FieldLeafNode_builder{
							SourceName: "0X1P5",
						}.Build(),
					}.Build(),
				},
				expectRoot: 0,
			},
			{
				name:       "hex_float_negative_exponent_as_subtraction",
				expression: "0x1p-5",
				expectNodes: []*pb.Node{
					pb.Node_builder{
						FieldLeafNode: pb.FieldLeafNode_builder{
							SourceName: "0x1p",
						}.Build(),
					}.Build(),
					pb.Node_builder{
						ConstantLeafNode: pb.ConstantLeafNode_builder{
							Int32Value: proto.Int32(5),
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
				expectRoot: 2,
			},
			{
				name:       "hex_float_positive_exponent_as_addition",
				expression: "0x1p+5",
				expectNodes: []*pb.Node{
					pb.Node_builder{
						FieldLeafNode: pb.FieldLeafNode_builder{
							SourceName: "0x1p",
						}.Build(),
					}.Build(),
					pb.Node_builder{
						ConstantLeafNode: pb.ConstantLeafNode_builder{
							Int32Value: proto.Int32(5),
						}.Build(),
					}.Build(),
					pb.Node_builder{
						CombinationNode: pb.CombinationNode_builder{
							LeftIndex:          proto.Uint32(0),
							RightIndex:         proto.Uint32(1),
							ArithmeticOperator: pb.CombinationNode_ADD.Enum(),
						}.Build(),
					}.Build(),
				},
				expectRoot: 2,
			},
			{
				name:       "hex_float_dot_mantissa_as_field_path",
				expression: "0x.p1",
				expectNodes: []*pb.Node{
					pb.Node_builder{
						FieldLeafNode: pb.FieldLeafNode_builder{
							SourceName: "0x",
							FieldNames: []string{"p1"},
						}.Build(),
					}.Build(),
				},
				expectRoot: 0,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				p := expressions.NewParserShunt(false)
				sess := map[uint32]expressions.Text{1: {Uncompiled: tc.expression}}
				rootIndices, nodes, err := p.CompileAll(sess)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if rootIndices[1] != tc.expectRoot {
					t.Errorf("expected root index %d, got %d", tc.expectRoot, rootIndices[1])
				}
				if diff := cmp.Diff(tc.expectNodes, nodes, protocmp.Transform()); diff != "" {
					t.Errorf("mismatch (-want +got):\n%s", diff)
				}
			})
		}
	})

	t.Run("rejected_expressions", func(t *testing.T) {
		invalidCases := []string{
			".5e2",
			"-1e5",
			"-1.5e-5",
			"-2.25E+4",
			".5p2",
			"-0x1p5",
			"-0x1p-5",
		}
		for _, expr := range invalidCases {
			t.Run(expr, func(t *testing.T) {
				p := expressions.NewParserShunt(false)
				sess := map[uint32]expressions.Text{1: {Uncompiled: expr}}
				_, _, err := p.CompileAll(sess)
				if err == nil {
					t.Fatalf("expected error for %q in parser, got success", expr)
				}
			})
		}
	})
}

func TestShuntParseInf(t *testing.T) {
	infCasingCases := []string{
		"inf",
		"Inf",
		"INF",
		"iNf",
		"infinity",
		"Infinity",
		"INFINITY",
		"InFiNiTy",
	}

	// 1. No prefix (parsed as +Inf) and negative prefix '-' (parsed as -Inf)
	prefixes := []struct {
		prefix    string
		expectVal float32
	}{
		{"", float32(math.Inf(1))},
		{"-", float32(math.Inf(-1))},
	}
	for _, pfx := range prefixes {
		for _, expr := range infCasingCases {
			fullExpr := pfx.prefix + expr
			t.Run(fullExpr, func(t *testing.T) {
				p := expressions.NewParserShunt(false)
				sess := map[uint32]expressions.Text{1: {Uncompiled: fullExpr}}
				rootIndices, nodes, err := p.CompileAll(sess)
				if err != nil {
					t.Fatalf("unexpected error for %q: %v", fullExpr, err)
				}
				expectNodes := []*pb.Node{
					pb.Node_builder{
						ConstantLeafNode: pb.ConstantLeafNode_builder{
							FloatValue: proto.Float32(pfx.expectVal),
						}.Build(),
					}.Build(),
				}
				if rootIndices[1] != 0 {
					t.Errorf("expected root index 0, got %d", rootIndices[1])
				}
				if diff := cmp.Diff(expectNodes, nodes, protocmp.Transform()); diff != "" {
					t.Errorf("mismatch (-want +got):\n%s", diff)
				}
			})
		}
	}

	// 2. Positive prefix '+' -> fails because '+' is a binary addition operator without a left operand
	for _, expr := range infCasingCases {
		t.Run("positive_prefix_+"+expr, func(t *testing.T) {
			p := expressions.NewParserShunt(false)
			sess := map[uint32]expressions.Text{1: {Uncompiled: "+" + expr}}
			_, _, err := p.CompileAll(sess)
			if err == nil {
				t.Fatalf("expected error for +%s, got success", expr)
			}
		})
	}
}

func TestShuntParseNaN(t *testing.T) {
	cmpOpts := []cmp.Option{
		protocmp.Transform(),
		cmp.Comparer(func(x, y float32) bool {
			return (math.IsNaN(float64(x)) && math.IsNaN(float64(y))) || x == y
		}),
	}

	nanCasingCases := []string{
		"nan",
		"NaN",
		"NAN",
		"nAn",
	}
	for _, expr := range nanCasingCases {
		t.Run("nan_no_prefix_"+expr, func(t *testing.T) {
			p := expressions.NewParserShunt(false)
			sess := map[uint32]expressions.Text{1: {Uncompiled: expr}}
			rootIndices, nodes, err := p.CompileAll(sess)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", expr, err)
			}
			expectNodes := []*pb.Node{
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						FloatValue: proto.Float32(float32(math.NaN())),
					}.Build(),
				}.Build(),
			}
			if rootIndices[1] != 0 {
				t.Errorf("expected root index 0, got %d", rootIndices[1])
			}
			if diff := cmp.Diff(expectNodes, nodes, cmpOpts...); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})

		t.Run("negative_nan_-"+expr, func(t *testing.T) {
			p := expressions.NewParserShunt(false)
			sess := map[uint32]expressions.Text{1: {Uncompiled: "-" + expr}}
			rootIndices, nodes, err := p.CompileAll(sess)
			if err != nil {
				t.Fatalf("unexpected error for -%s: %v", expr, err)
			}
			expectNodes := []*pb.Node{
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						FloatValue: proto.Float32(float32(math.NaN())),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(0),
						ArithmeticOperator: pb.CombinationNode_UNARY_MINUS.Enum(),
					}.Build(),
				}.Build(),
			}
			if rootIndices[1] != 1 {
				t.Errorf("expected root index 1, got %d", rootIndices[1])
			}
			if diff := cmp.Diff(expectNodes, nodes, cmpOpts...); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})

		t.Run("positive_nan_+"+expr, func(t *testing.T) {
			p := expressions.NewParserShunt(false)
			sess := map[uint32]expressions.Text{1: {Uncompiled: "+" + expr}}
			_, _, err := p.CompileAll(sess)
			if err == nil {
				t.Fatalf("expected error for +%s, got success", expr)
			}
		})
	}
}

func TestShuntParse_InvalidFieldPaths(t *testing.T) {
	cases := []struct {
		name       string
		expression string
	}{
		{
			name:       "consecutive_dots_in_root_field",
			expression: "source..field",
		},
		{
			name:       "trailing_dot_in_root_field",
			expression: "source.field.",
		},
		{
			name:       "consecutive_dots_in_postfix_field",
			expression: "source[0]..field",
		},
		{
			name:       "triple_dots",
			expression: "...",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := expressions.NewParserShunt(false)
			sess := map[uint32]expressions.Text{1: {Uncompiled: tc.expression}}
			_, _, err := p.CompileAll(sess)
			if err == nil {
				t.Fatalf("expected compile error for %q, got success", tc.expression)
			}
		})
	}
}

func TestShuntParse_UnaryOperatorsWithParentheses(t *testing.T) {
	testCases := []struct {
		name        string
		expression  string
		expectNodes []*pb.Node
		expectRoot  uint32
	}{
		{
			name:       "not_with_parenthesized_field",
			expression: "!(source.a)",
			expectNodes: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
						FieldNames: []string{"a"},
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:       proto.Uint32(0),
						LogicalOperator: pb.CombinationNode_NOT.Enum(),
					}.Build(),
				}.Build(),
			},
			expectRoot: 1,
		},
		{
			name:       "not_with_parenthesized_bool_literal",
			expression: "!(true)",
			expectNodes: []*pb.Node{
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						BoolValue: proto.Bool(true),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:       proto.Uint32(0),
						LogicalOperator: pb.CombinationNode_NOT.Enum(),
					}.Build(),
				}.Build(),
			},
			expectRoot: 1,
		},
		{
			name:       "not_with_parenthesized_comparison",
			expression: "!(source.a < 5)",
			expectNodes: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
						FieldNames: []string{"a"},
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(5),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(0),
						RightIndex:         proto.Uint32(1),
						RelationalOperator: pb.CombinationNode_LT.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:       proto.Uint32(2),
						LogicalOperator: pb.CombinationNode_NOT.Enum(),
					}.Build(),
				}.Build(),
			},
			expectRoot: 3,
		},
		{
			name:       "unary_minus_with_parenthesized_field",
			expression: "-(source.a)",
			expectNodes: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
						FieldNames: []string{"a"},
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(0),
						ArithmeticOperator: pb.CombinationNode_UNARY_MINUS.Enum(),
					}.Build(),
				}.Build(),
			},
			expectRoot: 1,
		},
		{
			name:       "unary_minus_with_parenthesized_constant",
			expression: "-(123)",
			expectNodes: []*pb.Node{
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(123),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(0),
						ArithmeticOperator: pb.CombinationNode_UNARY_MINUS.Enum(),
					}.Build(),
				}.Build(),
			},
			expectRoot: 1,
		},
		{
			name:       "unary_minus_with_parenthesized_binary_addition",
			expression: "-(source.a + 1)",
			expectNodes: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
						FieldNames: []string{"a"},
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(1),
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
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(2),
						ArithmeticOperator: pb.CombinationNode_UNARY_MINUS.Enum(),
					}.Build(),
				}.Build(),
			},
			expectRoot: 3,
		},
		{
			name:       "double_not_with_parentheses",
			expression: "!(!(source.a))",
			expectNodes: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
						FieldNames: []string{"a"},
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:       proto.Uint32(0),
						LogicalOperator: pb.CombinationNode_NOT.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:       proto.Uint32(1),
						LogicalOperator: pb.CombinationNode_NOT.Enum(),
					}.Build(),
				}.Build(),
			},
			expectRoot: 2,
		},
		{
			name:       "double_unary_minus_with_parentheses",
			expression: "-(-(source.a))",
			expectNodes: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
						FieldNames: []string{"a"},
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(0),
						ArithmeticOperator: pb.CombinationNode_UNARY_MINUS.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(1),
						ArithmeticOperator: pb.CombinationNode_UNARY_MINUS.Enum(),
					}.Build(),
				}.Build(),
			},
			expectRoot: 2,
		},
		{
			name:       "not_of_unary_minus_with_parentheses",
			expression: "!(-(source.a))",
			expectNodes: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
						FieldNames: []string{"a"},
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(0),
						ArithmeticOperator: pb.CombinationNode_UNARY_MINUS.Enum(),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:       proto.Uint32(1),
						LogicalOperator: pb.CombinationNode_NOT.Enum(),
					}.Build(),
				}.Build(),
			},
			expectRoot: 2,
		},
		{
			name:       "function_floor_single_arg_not_redundant",
			expression: "floor(source.a)",
			expectNodes: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
						FieldNames: []string{"a"},
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:        proto.Uint32(0),
						RoundingOperator: pb.CombinationNode_FLOOR.Enum(),
					}.Build(),
				}.Build(),
			},
			expectRoot: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := expressions.NewParserShunt(false)
			sess := map[uint32]expressions.Text{1: {Uncompiled: tc.expression}}
			rootIndices, nodes, err := p.CompileAll(sess)
			if err != nil {
				t.Fatalf("unexpected compile error: %v", err)
			}
			if rootIndices[1] != tc.expectRoot {
				t.Errorf("expected root index %d, got %d", tc.expectRoot, rootIndices[1])
			}
			if diff := cmp.Diff(tc.expectNodes, nodes, protocmp.Transform()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestShuntParse_RedundantParenthesesRejected(t *testing.T) {
	cases := []struct {
		name       string
		expression string
	}{
		{
			name:       "redundant_bare_parentheses_identifier",
			expression: "((source.a))",
		},
		{
			name:       "redundant_bare_parentheses_constant",
			expression: "((123))",
		},
		{
			name:       "redundant_outer_parentheses_binary_expr",
			expression: "((source.a + 1))",
		},
		{
			name:       "redundant_double_parens_in_unary_minus",
			expression: "-((source.a))",
		},
		{
			name:       "redundant_double_parens_in_not",
			expression: "!((source.a))",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := expressions.NewParserShunt(false)
			sess := map[uint32]expressions.Text{1: {Uncompiled: tc.expression}}
			_, _, err := p.CompileAll(sess)
			if err == nil {
				t.Fatalf("expected error for %q, got success", tc.expression)
			}
			if !strings.Contains(err.Error(), "Found redundant parenthesis") {
				t.Errorf("expected error containing 'Found redundant parenthesis', got: %v", err)
			}
		})
	}
}

func TestShuntParse_NestedFunctionCalls(t *testing.T) {
	cases := []struct {
		name        string
		expression  string
		expectRoot  uint32
		expectNodes []*pb.Node
	}{
		{
			name:       "both_arguments_nested",
			expression: "contains(doesnotcontain(source.a, 1), doesnotcontain(source.b, 2))",
			expectRoot: 6,
			expectNodes: []*pb.Node{
				pb.Node_builder{FieldLeafNode: pb.FieldLeafNode_builder{SourceName: "source", FieldNames: []string{"a"}}.Build()}.Build(),
				pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{Int32Value: proto.Int32(1)}.Build()}.Build(),
				pb.Node_builder{CombinationNode: pb.CombinationNode_builder{LeftIndex: proto.Uint32(0), RightIndex: proto.Uint32(1), RelationalOperator: pb.CombinationNode_DOES_NOT_CONTAIN.Enum()}.Build()}.Build(),
				pb.Node_builder{FieldLeafNode: pb.FieldLeafNode_builder{SourceName: "source", FieldNames: []string{"b"}}.Build()}.Build(),
				pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{Int32Value: proto.Int32(2)}.Build()}.Build(),
				pb.Node_builder{CombinationNode: pb.CombinationNode_builder{LeftIndex: proto.Uint32(3), RightIndex: proto.Uint32(4), RelationalOperator: pb.CombinationNode_DOES_NOT_CONTAIN.Enum()}.Build()}.Build(),
				pb.Node_builder{CombinationNode: pb.CombinationNode_builder{LeftIndex: proto.Uint32(2), RightIndex: proto.Uint32(5), RelationalOperator: pb.CombinationNode_CONTAINS.Enum()}.Build()}.Build(),
			},
		},
		{
			name:       "second_argument_nested",
			expression: "doesnotcontain(source.a, contains(source.b, 1))",
			expectRoot: 4,
			expectNodes: []*pb.Node{
				pb.Node_builder{FieldLeafNode: pb.FieldLeafNode_builder{SourceName: "source", FieldNames: []string{"a"}}.Build()}.Build(),
				pb.Node_builder{FieldLeafNode: pb.FieldLeafNode_builder{SourceName: "source", FieldNames: []string{"b"}}.Build()}.Build(),
				pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{Int32Value: proto.Int32(1)}.Build()}.Build(),
				pb.Node_builder{CombinationNode: pb.CombinationNode_builder{LeftIndex: proto.Uint32(1), RightIndex: proto.Uint32(2), RelationalOperator: pb.CombinationNode_CONTAINS.Enum()}.Build()}.Build(),
				pb.Node_builder{CombinationNode: pb.CombinationNode_builder{LeftIndex: proto.Uint32(0), RightIndex: proto.Uint32(3), RelationalOperator: pb.CombinationNode_DOES_NOT_CONTAIN.Enum()}.Build()}.Build(),
			},
		},
		{
			name:       "first_argument_nested",
			expression: "doesnotcontain(contains(source.a, 1), source.b)",
			expectRoot: 4,
			expectNodes: []*pb.Node{
				pb.Node_builder{FieldLeafNode: pb.FieldLeafNode_builder{SourceName: "source", FieldNames: []string{"a"}}.Build()}.Build(),
				pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{Int32Value: proto.Int32(1)}.Build()}.Build(),
				pb.Node_builder{CombinationNode: pb.CombinationNode_builder{LeftIndex: proto.Uint32(0), RightIndex: proto.Uint32(1), RelationalOperator: pb.CombinationNode_CONTAINS.Enum()}.Build()}.Build(),
				pb.Node_builder{FieldLeafNode: pb.FieldLeafNode_builder{SourceName: "source", FieldNames: []string{"b"}}.Build()}.Build(),
				pb.Node_builder{CombinationNode: pb.CombinationNode_builder{LeftIndex: proto.Uint32(2), RightIndex: proto.Uint32(3), RelationalOperator: pb.CombinationNode_DOES_NOT_CONTAIN.Enum()}.Build()}.Build(),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := expressions.NewParserShunt(false)
			sess := map[uint32]expressions.Text{1: {Uncompiled: tc.expression}}
			rootIndices, nodes, err := p.CompileAll(sess)
			if err != nil {
				t.Fatalf("unexpected compile error: %v", err)
			}
			if rootIndices[1] != tc.expectRoot {
				t.Errorf("expected root index %d, got %d", tc.expectRoot, rootIndices[1])
			}
			if diff := cmp.Diff(tc.expectNodes, nodes, protocmp.Transform()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestShuntParse_MatchedDelimiters(t *testing.T) {
	// abs(a[5] + 6)
	p := expressions.NewParserShunt(false)
	sess := map[uint32]expressions.Text{1: {Uncompiled: "abs(a[5] + 6)"}}
	rootIndices, nodes, err := p.CompileAll(sess)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}
	if rootIndices[1] != 5 {
		t.Errorf("expected root index 5, got %d", rootIndices[1])
	}

	expectNodes := []*pb.Node{
		pb.Node_builder{FieldLeafNode: pb.FieldLeafNode_builder{SourceName: "a"}.Build()}.Build(),
		pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{Int32Value: proto.Int32(5)}.Build()}.Build(),
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{LeftIndex: proto.Uint32(0), RightIndex: proto.Uint32(1), ListOperator: pb.CombinationNode_SUBSCRIPT.Enum()}.Build()}.Build(),
		pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{Int32Value: proto.Int32(6)}.Build()}.Build(),
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{LeftIndex: proto.Uint32(2), RightIndex: proto.Uint32(3), ArithmeticOperator: pb.CombinationNode_ADD.Enum()}.Build()}.Build(),
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{LeftIndex: proto.Uint32(4), ArithmeticOperator: pb.CombinationNode_ABSOLUTE.Enum()}.Build()}.Build(),
	}

	if diff := cmp.Diff(expectNodes, nodes, protocmp.Transform()); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestShuntParse_MismatchedDelimiters(t *testing.T) {
	cases := []struct {
		name        string
		expression  string
		expectError string
	}{
		{
			name:        "paren_closed_with_bracket",
			expression:  "(1 + 2]",
			expectError: "Found closing square bracket without matching opening square bracket",
		},
		{
			name:        "bracket_closed_with_paren",
			expression:  "source[0)",
			expectError: "Found closing parenthesis without matching opening parenthesis",
		},
		{
			name:        "bracket_with_expression_closed_with_paren",
			expression:  "source[1 + 2)",
			expectError: "Found closing parenthesis without matching opening parenthesis",
		},
		{
			name:        "function_paren_closed_with_bracket",
			expression:  "alleq(1, 2]",
			expectError: "Found closing square bracket without matching opening square bracket",
		},
		{
			name:        "function_subscript_closed_with_paren",
			expression:  "abs(a[5)",
			expectError: "Found closing parenthesis without matching opening parenthesis",
		},
		{
			name:        "function_paren_closed_with_bracket_no_subscript",
			expression:  "abs(a5])",
			expectError: "Found closing square bracket without matching opening square bracket",
		},
		{
			name:        "function_paren_closed_with_bracket_followed_by_tokens",
			expression:  "abs(1] bla)",
			expectError: "Found closing square bracket without matching opening square bracket",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := expressions.NewParserShunt(false)
			sess := map[uint32]expressions.Text{1: {Uncompiled: tc.expression}}
			_, _, err := p.CompileAll(sess)
			if err == nil {
				t.Fatalf("expected error for %q, got success", tc.expression)
			}
			if !strings.Contains(err.Error(), tc.expectError) {
				t.Errorf("For expression %q, expected error containing %q, but got %q", tc.expression, tc.expectError, err.Error())
			}
		})
	}
}

func TestShuntParse_TimestampFunctionErrors(t *testing.T) {
	cases := []struct {
		name        string
		expression  string
		expectError string
	}{
		{
			name:        "timestamp_zero_args",
			expression:  "timestamp()",
			expectError: "timestamp function requires a parameter",
		},
		{
			name:        "timestamp_whitespace_args",
			expression:  "timestamp(   )",
			expectError: "timestamp function requires a parameter",
		},
		{
			name:        "timestamp_multiple_args",
			expression:  "timestamp(REALTIME_CLOCK, MONOTONIC_TIME_SINCE_BOOT)",
			expectError: "timestamp function expects exactly one parameter",
		},
		{
			name:        "timestamp_invalid_clock_name",
			expression:  "timestamp(UNKNOWN_CLOCK)",
			expectError: "\"UNKNOWN_CLOCK\" is not a valid timestamp parameter",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := expressions.NewParserShunt(false)
			sess := map[uint32]expressions.Text{1: {Uncompiled: tc.expression}}
			_, _, err := p.CompileAll(sess)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", tc.expression)
			}
			if !strings.Contains(err.Error(), tc.expectError) {
				t.Errorf("error %q does not contain expected substring %q", err.Error(), tc.expectError)
			}
		})
	}
}

func TestShuntParse_FunctionArityAndCommaErrors(t *testing.T) {
	cases := []struct {
		name        string
		expression  string
		expectError string
	}{
		{
			name:        "abs_too_many_args",
			expression:  "abs(1, 2)",
			expectError: "Found operand(s) but no operator",
		},
		{
			name:        "round_zero_args",
			expression:  "round()",
			expectError: "Missing operand for unary operator: OperatorRound",
		},
		{
			name:        "contains_too_few_args",
			expression:  "contains(1)",
			expectError: "Missing operand(s) for binary operator: OperatorContains",
		},
		{
			name:        "contains_too_many_args",
			expression:  "contains(1, 2, 3)",
			expectError: "Found operand(s) but no operator",
		},
		{
			name:        "abs_trailing_comma",
			expression:  "abs(1,)",
			expectError: "unexpected trailing comma",
		},
		{
			name:        "contains_consecutive_commas",
			expression:  "contains(1,, 2)",
			expectError: "unexpected comma: missing argument before comma",
		},
		{
			name:        "comma_outside_function",
			expression:  "1, 2",
			expectError: "unexpected comma outside function call",
		},
		{
			name:        "comma_in_parentheses_outside_function",
			expression:  "(1, 2)",
			expectError: "unexpected comma outside function call",
		},
		{
			name:        "comma_in_subscript",
			expression:  "source[1, 2]",
			expectError: "unexpected comma",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := expressions.NewParserShunt(false)
			sess := map[uint32]expressions.Text{1: {Uncompiled: tc.expression}}
			_, _, err := p.CompileAll(sess)
			if err == nil {
				t.Fatalf("expected error for %q, got success", tc.expression)
			}
			if !strings.Contains(err.Error(), tc.expectError) {
				t.Errorf("For expression %q, expected error containing %q, but got %q", tc.expression, tc.expectError, err.Error())
			}
		})
	}
}

func TestShuntParse_NegativeArgumentAfterComma(t *testing.T) {
	testCases := []struct {
		name       string
		expression string
		expect     []*pb.Node
	}{
		{
			name:       "contains_with_negative_integer",
			expression: "contains(source.field_1, -1)",
			expect: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
						FieldNames: []string{"field_1"},
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						Int32Value: proto.Int32(-1),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(0),
						RightIndex:         proto.Uint32(1),
						RelationalOperator: pb.CombinationNode_CONTAINS.Enum(),
					}.Build(),
				}.Build(),
			},
		},
		{
			name:       "doesnotcontain_with_negative_float",
			expression: "doesnotcontain(source.field_1, -10.5)",
			expect: []*pb.Node{
				pb.Node_builder{
					FieldLeafNode: pb.FieldLeafNode_builder{
						SourceName: "source",
						FieldNames: []string{"field_1"},
					}.Build(),
				}.Build(),
				pb.Node_builder{
					ConstantLeafNode: pb.ConstantLeafNode_builder{
						FloatValue: proto.Float32(-10.5),
					}.Build(),
				}.Build(),
				pb.Node_builder{
					CombinationNode: pb.CombinationNode_builder{
						LeftIndex:          proto.Uint32(0),
						RightIndex:         proto.Uint32(1),
						RelationalOperator: pb.CombinationNode_DOES_NOT_CONTAIN.Enum(),
					}.Build(),
				}.Build(),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := expressions.NewParserShunt(false)
			sess := map[uint32]expressions.Text{1: {Uncompiled: tc.expression}}
			rootIndices, nodes, err := p.CompileAll(sess)
			if err != nil {
				t.Fatalf("unexpected compile error: %v", err)
			}
			if rootIndices[1] != uint32(len(tc.expect)-1) {
				t.Errorf("expected root index %d, got %d", len(tc.expect)-1, rootIndices[1])
			}
			if diff := cmp.Diff(tc.expect, nodes, protocmp.Transform()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestShuntParse_FunctionCommaInfixOperators(t *testing.T) {
	// contains(1 + 2, 3 * 4) must evaluate (1 + 2) and (3 * 4) as separate arguments, not bind + across the comma
	p := expressions.NewParserShunt(false)
	sess := map[uint32]expressions.Text{1: {Uncompiled: "contains(1 + 2, 3 * 4)"}}
	rootIndices, nodes, err := p.CompileAll(sess)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	expectNodes := []*pb.Node{
		pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{Int32Value: proto.Int32(1)}.Build()}.Build(),
		pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{Int32Value: proto.Int32(2)}.Build()}.Build(),
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{LeftIndex: proto.Uint32(0), RightIndex: proto.Uint32(1), ArithmeticOperator: pb.CombinationNode_ADD.Enum()}.Build()}.Build(),
		pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{Int32Value: proto.Int32(3)}.Build()}.Build(),
		pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{Int32Value: proto.Int32(4)}.Build()}.Build(),
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{LeftIndex: proto.Uint32(3), RightIndex: proto.Uint32(4), ArithmeticOperator: pb.CombinationNode_MULTIPLY.Enum()}.Build()}.Build(),
		pb.Node_builder{CombinationNode: pb.CombinationNode_builder{LeftIndex: proto.Uint32(2), RightIndex: proto.Uint32(5), RelationalOperator: pb.CombinationNode_CONTAINS.Enum()}.Build()}.Build(),
	}

	if rootIndices[1] != 6 {
		t.Errorf("expected root index 6, got %d", rootIndices[1])
	}
	if diff := cmp.Diff(expectNodes, nodes, protocmp.Transform()); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}
