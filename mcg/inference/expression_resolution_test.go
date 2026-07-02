// Copyright 2026 Google LLC
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
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

func TestResolveConstLeafNode(t *testing.T) {
	tests := []struct {
		name     string
		node     *pb.ConstantLeafNode
		wantDesc *descriptorpb.FieldDescriptorProto
	}{
		{
			name:     "Int32",
			node:     pb.ConstantLeafNode_builder{Int32Value: proto.Int32(1)}.Build(),
			wantDesc: &descriptorpb.FieldDescriptorProto{Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()},
		},
		{
			name:     "Double",
			node:     pb.ConstantLeafNode_builder{DoubleValue: proto.Float64(1.0)}.Build(),
			wantDesc: &descriptorpb.FieldDescriptorProto{Type: descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()},
		},
		{
			name:     "Bool",
			node:     pb.ConstantLeafNode_builder{BoolValue: proto.Bool(true)}.Build(),
			wantDesc: &descriptorpb.FieldDescriptorProto{Type: descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc, err := resolveConstLeafNode(tt.node)
			if err != nil {
				t.Fatalf("resolveConstLeafNode failed: %v", err)
			}
			if diff := cmp.Diff(tt.wantDesc, desc, protocmp.Transform()); diff != "" {
				t.Errorf("Resolve() returned unexpected descriptor diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolveFunctionLeafNode(t *testing.T) {
	node := pb.FunctionLeafNode_builder{
		GetCurrentTimestamp: pb.GetCurrentTimestampFunction_builder{}.Build(),
	}.Build()

	desc, err := resolveFunctionLeafNode(node)
	if err != nil {
		t.Fatalf("resolveFunctionLeafNode failed: %v", err)
	}
	if desc.GetType() != descriptorpb.FieldDescriptorProto_TYPE_INT64 {
		t.Errorf("got type %v, want TYPE_INT64", desc.GetType())
	}
}

func TestIsIntAndIsNumeric(t *testing.T) {
	tInt32 := descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()
	tDouble := descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()
	tString := descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()

	if !isInt(tInt32) {
		t.Errorf("expected TYPE_INT32 to be int")
	}
	if isInt(tDouble) {
		t.Errorf("expected TYPE_DOUBLE not to be int")
	}

	if !isNumeric(tInt32) {
		t.Errorf("expected TYPE_INT32 to be numeric")
	}
	if !isNumeric(tDouble) {
		t.Errorf("expected TYPE_DOUBLE to be numeric")
	}
	if isNumeric(tString) {
		t.Errorf("expected TYPE_STRING not to be numeric")
	}
}

func TestResolveFieldPath(t *testing.T) {
	// Create a mock getDescriptorProto
	mockGetDescriptorProto := func(name protoreflect.FullName) *descriptorpb.DescriptorProto {
		if name == "pkg.Message" {
			return &descriptorpb.DescriptorProto{
				Name: proto.String("Message"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name: proto.String("int_field"),
						Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
					},
					{
						Name:     proto.String("msg_field"),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".pkg.NestedMessage"),
					},
					{
						Name:     proto.String("enum_field"),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
						TypeName: proto.String(".pkg.Enum"),
					},
					{
						Name:     proto.String("list_field"),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".pkg.NestedMessage"),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
					},
					{
						Name:     proto.String("my_map"),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".pkg.Message.MyMapEntry"),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
					},
				},
			}
		}
		if name == "pkg.Message.MyMapEntry" {
			return &descriptorpb.DescriptorProto{
				Name: proto.String("MyMapEntry"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name: proto.String("key"),
						Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					},
					{
						Name: proto.String("value"),
						Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
					},
				},
			}
		}
		if name == "pkg.NestedMessage" {
			return &descriptorpb.DescriptorProto{
				Name: proto.String("NestedMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name: proto.String("nested_int"),
						Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
					},
				},
			}
		}
		return nil
	}

	er := NewExpressionResolver(
		nil,
		nil,
		mockGetDescriptorProto,
	)

	tests := []struct {
		name        string
		msgFullName protoreflect.FullName
		fieldNames  []string
		wantDesc    *descriptorpb.FieldDescriptorProto
		wantErr     bool
	}{
		{
			name:        "Direct message reference",
			msgFullName: protoreflect.FullName("pkg.Message"),
			fieldNames:  []string{},
			wantDesc: &descriptorpb.FieldDescriptorProto{
				Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
				TypeName: proto.String(".pkg.Message"),
			},
		},
		{
			name:        "Int field",
			msgFullName: protoreflect.FullName("pkg.Message"),
			fieldNames:  []string{"int_field"},
			wantDesc: &descriptorpb.FieldDescriptorProto{
				Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
			},
		},
		{
			name:        "Enum field",
			msgFullName: protoreflect.FullName("pkg.Message"),
			fieldNames:  []string{"enum_field"},
			wantDesc: &descriptorpb.FieldDescriptorProto{
				Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
				TypeName: proto.String(".pkg.Enum"),
			},
		},
		{
			name:        "Message field",
			msgFullName: protoreflect.FullName("pkg.Message"),
			fieldNames:  []string{"msg_field"},
			wantDesc: &descriptorpb.FieldDescriptorProto{
				Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
				TypeName: proto.String(".pkg.NestedMessage"),
			},
		},
		{
			name:        "Nested int field",
			msgFullName: protoreflect.FullName("pkg.Message"),
			fieldNames:  []string{"msg_field", "nested_int"},
			wantDesc: &descriptorpb.FieldDescriptorProto{
				Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
			},
		},
		{
			name:        "Repeated field leaf",
			msgFullName: protoreflect.FullName("pkg.Message"),
			fieldNames:  []string{"list_field"},
			wantDesc: &descriptorpb.FieldDescriptorProto{
				Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
				TypeName: proto.String(".pkg.NestedMessage"),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
			},
		},
		{
			name:        "Repeated field error",
			msgFullName: protoreflect.FullName("pkg.Message"),
			fieldNames:  []string{"list_field", "nested_int"},
			wantErr:     true,
		},
		{
			name:        "Map field access error",
			msgFullName: protoreflect.FullName("pkg.Message"),
			fieldNames:  []string{"my_map", "key"},
			wantErr:     true,
		},
		{
			name:        "Unknown message",
			msgFullName: protoreflect.FullName("pkg.Unknown"),
			fieldNames:  []string{"int_field"},
			wantErr:     true,
		},
		{
			name:        "Unknown field",
			msgFullName: protoreflect.FullName("pkg.Message"),
			fieldNames:  []string{"unknown"},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc, err := er.resolveFieldPath(tt.msgFullName, tt.fieldNames)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got error %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if diff := cmp.Diff(tt.wantDesc, desc, protocmp.Transform()); diff != "" {
					t.Errorf("Resolve() returned unexpected descriptor diff (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestResolveCombinationNode(t *testing.T) {
	er := &ExpressionResolver{} // Empty resolver is enough for this test

	tests := []struct {
		name     string
		node     *pb.CombinationNode
		wantDesc *descriptorpb.FieldDescriptorProto
	}{
		{
			name:     "Logical",
			node:     pb.CombinationNode_builder{LogicalOperator: pb.CombinationNode_AND.Enum()}.Build(),
			wantDesc: &descriptorpb.FieldDescriptorProto{Type: descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()},
		},
		{
			name:     "Relational",
			node:     pb.CombinationNode_builder{RelationalOperator: pb.CombinationNode_EQ.Enum()}.Build(),
			wantDesc: &descriptorpb.FieldDescriptorProto{Type: descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()},
		},
		{
			name:     "Rounding",
			node:     pb.CombinationNode_builder{RoundingOperator: pb.CombinationNode_CEIL.Enum()}.Build(),
			wantDesc: &descriptorpb.FieldDescriptorProto{Type: descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc, err := er.resolveCombinationNode(tt.node)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.wantDesc, desc, protocmp.Transform()); diff != "" {
				t.Errorf("Resolve() returned unexpected descriptor diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolveArithExpressions(t *testing.T) {
	config := pb.MetricsConfig_builder{
		ExpressionNodes: []*pb.Node{
			pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{Int32Value: proto.Int32(1)}.Build()}.Build(),
			pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{DoubleValue: proto.Float64(2.0)}.Build()}.Build(),
		},
	}.Build()

	er := NewExpressionResolver(config.GetExpressionNodes(), nil, nil)

	tests := []struct {
		name       string
		operator   pb.CombinationNode_ArithmeticOperator
		leftIndex  uint32
		rightIndex uint32
		wantDesc   *descriptorpb.FieldDescriptorProto
	}{
		{"Int32 + Double -> Double", pb.CombinationNode_ADD, 0, 1, &descriptorpb.FieldDescriptorProto{
			Type: descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum(),
		}},
		{"Int32 / Int32 -> Double", pb.CombinationNode_DIVIDE, 0, 0, &descriptorpb.FieldDescriptorProto{
			Type: descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum(),
		}},
		{"Int32 % Int32 -> Int64", pb.CombinationNode_MODULO_TRUNC, 0, 0, &descriptorpb.FieldDescriptorProto{
			Type: descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
		}},
		{"UnaryMinus Int32 -> Int64", pb.CombinationNode_UNARY_MINUS, 0, 0, &descriptorpb.FieldDescriptorProto{Type: descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()}},

		{"UnaryMinus Double -> Double", pb.CombinationNode_UNARY_MINUS, 1, 0, &descriptorpb.FieldDescriptorProto{
			Type: descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum(),
		}},
		{"Int32 ^ Int32 -> Double", pb.CombinationNode_POWER, 0, 0, &descriptorpb.FieldDescriptorProto{
			Type: descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum(),
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := pb.CombinationNode_builder{
				ArithmeticOperator: tt.operator.Enum(),
				LeftIndex:          proto.Uint32(tt.leftIndex),
				RightIndex:         proto.Uint32(tt.rightIndex),
			}.Build()

			desc, err := er.resolveArithExpressions(node)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.wantDesc, desc, protocmp.Transform()); diff != "" {
				t.Errorf("Resolve() returned unexpected descriptor diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	er := NewExpressionResolver(
		[]*pb.Node{
			pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{Int32Value: proto.Int32(1)}.Build()}.Build(),
			pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{Int64Value: proto.Int64(1)}.Build()}.Build(), // Unused, just to test out of bounds
		},
		nil,
		nil,
	)

	desc, err := er.Resolve(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if desc.GetType() != descriptorpb.FieldDescriptorProto_TYPE_INT32 {
		t.Errorf("got type %v, want TYPE_INT32", desc.GetType())
	}

	erInvalid := NewExpressionResolver(
		[]*pb.Node{
			pb.Node_builder{}.Build(), // Missing node content
		},
		nil,
		nil,
	)
	_, errInvalid := erInvalid.Resolve(0)
	if errInvalid == nil {
		t.Errorf("expected error for empty node type")
	}
}

func TestResolveFieldLeafNodeNestedAccess(t *testing.T) {
	// Mock schema matching what both Data Sources and Aggregators might return
	mockGetDescriptorProto := func(name protoreflect.FullName) *descriptorpb.DescriptorProto {
		if name == "pkg.Message" {
			return &descriptorpb.DescriptorProto{
				Name: proto.String("Message"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("msg_field"),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".pkg.NestedMessage"),
					},
				},
			}
		}
		if name == "pkg.NestedMessage" {
			return &descriptorpb.DescriptorProto{
				Name: proto.String("NestedMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name: proto.String("nested_int"),
						Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
					},
				},
			}
		}
		return nil
	}

	// Mock source lookup: returns pkg.Message for both sources.
	mockGetSourceMessageName := func(sourceName string) (protoreflect.FullName, error) {
		if sourceName == "my_data_source" || sourceName == "my_aggregator" {
			return protoreflect.FullName("pkg.Message"), nil
		}
		return "", fmt.Errorf("unknown source: %s", sourceName)
	}

	er := NewExpressionResolver(
		[]*pb.Node{
			// Index 0: Accessing nested field on a data source
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					SourceName: "my_data_source",
					FieldNames: []string{"msg_field", "nested_int"},
				}.Build(),
			}.Build(),
			// Index 1: Accessing nested field on an aggregator
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					SourceName: "my_aggregator",
					FieldNames: []string{"msg_field", "nested_int"},
				}.Build(),
			}.Build(),
		},
		mockGetSourceMessageName,
		mockGetDescriptorProto,
	)

	// Validate Data Source nested access
	descDS, err := er.Resolve(0)
	if err != nil {
		t.Fatalf("unexpected error resolving data source nested field: %v", err)
	}
	wantDS := &descriptorpb.FieldDescriptorProto{Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()}
	if diff := cmp.Diff(wantDS, descDS, protocmp.Transform()); diff != "" {
		t.Errorf("Resolve() for data source returned unexpected descriptor diff (-want +got):\n%s", diff)
	}

	// Validate Aggregator nested access
	descAgg, err := er.Resolve(1)
	if err != nil {
		t.Fatalf("unexpected error resolving aggregator nested field: %v", err)
	}
	wantAgg := &descriptorpb.FieldDescriptorProto{Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()}
	if diff := cmp.Diff(wantAgg, descAgg, protocmp.Transform()); diff != "" {
		t.Errorf("Resolve() for aggregator returned unexpected descriptor diff (-want +got):\n%s", diff)
	}
}

func TestResolveFieldLeafNodeEmptyFields(t *testing.T) {
	mockGetDescriptorProto := func(name protoreflect.FullName) *descriptorpb.DescriptorProto {
		if name == "pkg.Message" {
			return &descriptorpb.DescriptorProto{
				Name:  proto.String("Message"),
				Field: []*descriptorpb.FieldDescriptorProto{},
			}
		}
		return nil
	}

	mockGetSourceMessageName := func(sourceName string) (protoreflect.FullName, error) {
		if sourceName == "source_name" {
			return protoreflect.FullName("pkg.Message"), nil
		}
		return "", fmt.Errorf("unknown source: %s", sourceName)
	}

	er := NewExpressionResolver(
		[]*pb.Node{
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					SourceName: "source_name",
					FieldNames: []string{},
				}.Build(),
			}.Build(),
		},
		mockGetSourceMessageName,
		mockGetDescriptorProto,
	)

	desc, err := er.Resolve(0)
	if err != nil {
		t.Fatalf("unexpected error resolving empty field path: %v", err)
	}
	wantDesc := &descriptorpb.FieldDescriptorProto{
		Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
		TypeName: proto.String(".pkg.Message"),
	}
	if diff := cmp.Diff(wantDesc, desc, protocmp.Transform()); diff != "" {
		t.Errorf("Resolve() returned unexpected descriptor diff (-want +got):\n%s", diff)
	}
}

func TestResolveFieldLeafNodeNonMessageNestedAccess(t *testing.T) {
	mockGetDescriptorProto := func(name protoreflect.FullName) *descriptorpb.DescriptorProto {
		if name == "pkg.Message" {
			return &descriptorpb.DescriptorProto{
				Name: proto.String("Message"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name: proto.String("int_field"),
						Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
					},
				},
			}
		}
		return nil
	}

	mockGetSourceMessageName := func(sourceName string) (protoreflect.FullName, error) {
		if sourceName == "source_name" {
			return protoreflect.FullName("pkg.Message"), nil
		}
		return "", fmt.Errorf("unknown source: %s", sourceName)
	}

	er := NewExpressionResolver(
		[]*pb.Node{
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					SourceName: "source_name",
					FieldNames: []string{"int_field", "nested_int"},
				}.Build(),
			}.Build(),
		},
		mockGetSourceMessageName,
		mockGetDescriptorProto,
	)

	_, err := er.Resolve(0)
	if err == nil {
		t.Fatal("expected error resolving nested access on non-message field, got nil")
	}
	expectedErrMsg := "non-message type \"pkg.Message\" cannot have nested type \"nested_int\""
	if !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("got error %v, want it to contain %q", err, expectedErrMsg)
	}
}

func TestResolveFieldLeafNodeLabelCoercion(t *testing.T) {
	mockGetDescriptorProto := func(name protoreflect.FullName) *descriptorpb.DescriptorProto {
		if name == "pkg.Message" {
			return &descriptorpb.DescriptorProto{
				Name: proto.String("Message"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:  proto.String("proto2_required"),
						Type:  descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
						Label: descriptorpb.FieldDescriptorProto_LABEL_REQUIRED.Enum(),
					},
					{
						Name:  proto.String("proto2_optional"),
						Type:  descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
						Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
					{
						Name:           proto.String("proto3_optional"),
						Type:           descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
						Label:          descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Proto3Optional: proto.Bool(true),
					},
					{
						Name: proto.String("proto3_non_optional"),
						Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
					},
				},
			}
		}
		return nil
	}

	mockGetSourceMessageName := func(sourceName string) (protoreflect.FullName, error) {
		if sourceName == "source_name" {
			return protoreflect.FullName("pkg.Message"), nil
		}
		return "", fmt.Errorf("unknown source: %s", sourceName)
	}

	tests := []struct {
		name      string
		fieldName string
	}{
		{
			name:      "proto2 required",
			fieldName: "proto2_required",
		},
		{
			name:      "proto2 optional",
			fieldName: "proto2_optional",
		},
		{
			name:      "proto3 optional",
			fieldName: "proto3_optional",
		},
		{
			name:      "proto3 non-optional",
			fieldName: "proto3_non_optional",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := NewExpressionResolver(
				[]*pb.Node{
					pb.Node_builder{
						FieldLeafNode: pb.FieldLeafNode_builder{
							SourceName: "source_name",
							FieldNames: []string{tt.fieldName},
						}.Build(),
					}.Build(),
				},
				mockGetSourceMessageName,
				mockGetDescriptorProto,
			)

			desc, err := er.Resolve(0)
			if err != nil {
				t.Fatalf("unexpected error resolving field path: %v", err)
			}

			wantDesc := &descriptorpb.FieldDescriptorProto{Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()}
			if diff := cmp.Diff(wantDesc, desc, protocmp.Transform()); diff != "" {
				t.Errorf("Resolve() returned unexpected descriptor diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolveListExpressions(t *testing.T) {
	mockGetDescriptorProto := func(name protoreflect.FullName) *descriptorpb.DescriptorProto {
		if name == "pkg.Message" {
			return &descriptorpb.DescriptorProto{
				Name: proto.String("Message"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:  proto.String("repeated_int"),
						Type:  descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
						Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
					},
					{
						Name:  proto.String("singular_int"),
						Type:  descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
						Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
					{
						Name:     proto.String("repeated_msg"),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".pkg.NestedMessage"),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
					},
				},
			}
		}
		return nil
	}

	mockGetSourceMessageName := func(sourceName string) (protoreflect.FullName, error) {
		if sourceName == "my_source" {
			return protoreflect.FullName("pkg.Message"), nil
		}
		return "", fmt.Errorf("unknown source: %s", sourceName)
	}

	config := pb.MetricsConfig_builder{
		ExpressionNodes: []*pb.Node{
			// Index 0: repeated_int
			pb.Node_builder{FieldLeafNode: pb.FieldLeafNode_builder{SourceName: "my_source", FieldNames: []string{"repeated_int"}}.Build()}.Build(),
			// Index 1: singular_int
			pb.Node_builder{FieldLeafNode: pb.FieldLeafNode_builder{SourceName: "my_source", FieldNames: []string{"singular_int"}}.Build()}.Build(),
			// Index 2: 0 (int32)
			pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{Int32Value: proto.Int32(0)}.Build()}.Build(),
			// Index 3: -1 (int32)
			pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{Int32Value: proto.Int32(-1)}.Build()}.Build(),
			// Index 4: 1.5 (double)
			pb.Node_builder{ConstantLeafNode: pb.ConstantLeafNode_builder{DoubleValue: proto.Float64(1.5)}.Build()}.Build(),
			// Index 5: repeated_msg
			pb.Node_builder{FieldLeafNode: pb.FieldLeafNode_builder{SourceName: "my_source", FieldNames: []string{"repeated_msg"}}.Build()}.Build(),
		},
	}.Build()

	er := NewExpressionResolver(config.GetExpressionNodes(), mockGetSourceMessageName, mockGetDescriptorProto)

	tests := []struct {
		name     string
		node     *pb.CombinationNode
		wantDesc *descriptorpb.FieldDescriptorProto
		wantErr  bool
	}{
		{
			name: "length(repeated_int) -> INT32",
			node: pb.CombinationNode_builder{
				ListOperator: pb.CombinationNode_LENGTH.Enum(),
				LeftIndex:    proto.Uint32(0),
			}.Build(),
			wantDesc: &descriptorpb.FieldDescriptorProto{Type: descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()},
		},
		{
			name: "length(singular_int) -> error",
			node: pb.CombinationNode_builder{
				ListOperator: pb.CombinationNode_LENGTH.Enum(),
				LeftIndex:    proto.Uint32(1),
			}.Build(),
			wantErr: true,
		},
		{
			name: "repeated_int[0] -> INT32 (singular)",
			node: pb.CombinationNode_builder{
				ListOperator: pb.CombinationNode_SUBSCRIPT.Enum(),
				LeftIndex:    proto.Uint32(0),
				RightIndex:   proto.Uint32(2),
			}.Build(),
			wantDesc: &descriptorpb.FieldDescriptorProto{
				Type:  descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
				Label: nil,
			},
		},
		{
			name: "repeated_int[-1] -> INT32 (singular)",
			node: pb.CombinationNode_builder{
				ListOperator: pb.CombinationNode_SUBSCRIPT.Enum(),
				LeftIndex:    proto.Uint32(0),
				RightIndex:   proto.Uint32(3),
			}.Build(),
			wantDesc: &descriptorpb.FieldDescriptorProto{
				Type:  descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
				Label: nil,
			},
		},
		{
			name: "repeated_msg[0] -> MESSAGE (singular)",
			node: pb.CombinationNode_builder{
				ListOperator: pb.CombinationNode_SUBSCRIPT.Enum(),
				LeftIndex:    proto.Uint32(5),
				RightIndex:   proto.Uint32(2),
			}.Build(),
			wantDesc: &descriptorpb.FieldDescriptorProto{
				Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
				TypeName: proto.String(".pkg.NestedMessage"),
				Label:    nil,
			},
		},
		{
			name: "repeated_int[1.5] -> error (invalid index type)",
			node: pb.CombinationNode_builder{
				ListOperator: pb.CombinationNode_SUBSCRIPT.Enum(),
				LeftIndex:    proto.Uint32(0),
				RightIndex:   proto.Uint32(4),
			}.Build(),
			wantErr: true,
		},
		{
			name: "singular_int[0] -> error (not repeated)",
			node: pb.CombinationNode_builder{
				ListOperator: pb.CombinationNode_SUBSCRIPT.Enum(),
				LeftIndex:    proto.Uint32(1),
				RightIndex:   proto.Uint32(2),
			}.Build(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc, err := er.resolveListExpressions(tt.node)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got error %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if diff := cmp.Diff(tt.wantDesc, desc, protocmp.Transform()); diff != "" {
					t.Errorf("Resolve() returned unexpected descriptor diff (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestResolveFieldLeafNodeWithExpressionIndex(t *testing.T) {
	mockGetDescriptorProto := func(name protoreflect.FullName) *descriptorpb.DescriptorProto {
		if name == "pkg.Message" {
			return &descriptorpb.DescriptorProto{
				Name: proto.String("Message"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("msg_field"),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".pkg.NestedMessage"),
					},
					{
						Name: proto.String("int_field"),
						Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
					},
					{
						Name:     proto.String("repeated_msg_field"),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".pkg.NestedMessage"),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
					},
				},
			}
		}
		if name == "pkg.NestedMessage" {
			return &descriptorpb.DescriptorProto{
				Name: proto.String("NestedMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name: proto.String("nested_int"),
						Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
					},
				},
			}
		}
		return nil
	}

	mockGetSourceMessageName := func(sourceName string) (protoreflect.FullName, error) {
		if sourceName == "my_data_source" {
			return protoreflect.FullName("pkg.Message"), nil
		}
		return "", fmt.Errorf("unknown source: %s", sourceName)
	}

	er := NewExpressionResolver(
		[]*pb.Node{
			// Index 0: Base FieldLeafNode (yields pkg.Message)
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					SourceName: "my_data_source",
				}.Build(),
			}.Build(),
			// Index 1: Postfix FieldLeafNode pointing to Index 0, accessing msg_field.nested_int
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					ExpressionNodeIndex: proto.Uint32(0),
					FieldNames:          []string{"msg_field", "nested_int"},
				}.Build(),
			}.Build(),
			// Index 2: Postfix FieldLeafNode pointing to Index 0, accessing int_field (non-message)
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					ExpressionNodeIndex: proto.Uint32(0),
					FieldNames:          []string{"int_field"},
				}.Build(),
			}.Build(),
			// Index 3: Invalid Postfix FieldLeafNode pointing to Index 2 (which is INT32, not MESSAGE)
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					ExpressionNodeIndex: proto.Uint32(2),
					FieldNames:          []string{"some_field"},
				}.Build(),
			}.Build(),
			// Index 4: Postfix FieldLeafNode pointing to Index 0, accessing repeated_msg_field
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					ExpressionNodeIndex: proto.Uint32(0),
					FieldNames:          []string{"repeated_msg_field"},
				}.Build(),
			}.Build(),
			// Index 5: Invalid Postfix FieldLeafNode pointing to Index 4 (which is REPEATED MESSAGE)
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					ExpressionNodeIndex: proto.Uint32(4),
					FieldNames:          []string{"nested_int"},
				}.Build(),
			}.Build(),
			// Index 6: Invalid: Both source_name and expression_node_index set
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					SourceName:          "my_data_source",
					ExpressionNodeIndex: proto.Uint32(0),
				}.Build(),
			}.Build(),
			// Index 7: Invalid: Neither set
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{}.Build(),
			}.Build(),
		},
		mockGetSourceMessageName,
		mockGetDescriptorProto,
	)

	tests := []struct {
		name    string
		nodeIdx uint32
		want    *descriptorpb.FieldDescriptorProto
		wantErr bool
	}{
		{
			name:    "valid postfix access",
			nodeIdx: 1,
			want:    &descriptorpb.FieldDescriptorProto{Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()},
		},
		{
			name:    "valid postfix access yielding int32",
			nodeIdx: 2,
			want:    &descriptorpb.FieldDescriptorProto{Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()},
		},
		{
			name:    "error: pointing to non-message node",
			nodeIdx: 3,
			wantErr: true,
		},
		{
			name:    "error: pointing to repeated message node",
			nodeIdx: 5,
			wantErr: true,
		},
		{
			name:    "error: both source and expression index set",
			nodeIdx: 6,
			wantErr: true,
		},
		{
			name:    "error: neither source nor expression index set",
			nodeIdx: 7,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc, err := er.Resolve(tt.nodeIdx)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Resolve() got error %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if diff := cmp.Diff(tt.want, desc, protocmp.Transform()); diff != "" {
					t.Errorf("Resolve() returned unexpected descriptor diff (-want +got):\n%s", diff)
				}
			}
		})
	}
}
