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

package inference_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/descriptorpb"

	"sdv.googlesource.com/mcg/mcg/inference"
	"sdv.googlesource.com/mcg/mcg/type_resolvers"
	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

func TestIsInt(t *testing.T) {
	for _, testCase := range []struct {
		wantMain *descriptorpb.FieldDescriptorProto_Type
		wantBool bool
	}{
		{wantMain: descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum(), wantBool: false},
		{wantMain: descriptorpb.FieldDescriptorProto_TYPE_FLOAT.Enum(), wantBool: false},
		{wantMain: descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(), wantBool: true},
		{wantMain: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(), wantBool: true},
	} {
		if got := inference.IsInt(testCase.wantMain); got != testCase.wantBool {
			t.Errorf("inference.IsInt(%v) = %v, want %v", testCase.wantMain, got, testCase.wantBool)
		}
	}
}

func TestIsNumeric(t *testing.T) {
	for _, testCase := range []struct {
		wantMain *descriptorpb.FieldDescriptorProto_Type
		wantBool bool
	}{
		{wantMain: descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum(), wantBool: true},
		{wantMain: descriptorpb.FieldDescriptorProto_TYPE_FLOAT.Enum(), wantBool: true},
		{wantMain: descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(), wantBool: true},
		{wantMain: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(), wantBool: true},
		{wantMain: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(), wantBool: false},
		{wantMain: descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum(), wantBool: false},
	} {
		if got := inference.IsNumeric(testCase.wantMain); got != testCase.wantBool {
			t.Errorf("inference.IsNumeric(%v) = %v, want %v", testCase.wantMain, got, testCase.wantBool)
		}
	}
}

func TestInferAggregatorNestedFieldTraversal(t *testing.T) {
	depFd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("dep.proto"),
		Package: proto.String("my.dep.package"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("NestedMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("nested_int"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
					},
				},
			},
		},
	}
	mainFd := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("main.proto"),
		Package:    proto.String("my.main.package"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"dep.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("MyMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("msg_field"),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".my.dep.package.NestedMessage"),
					},
				},
			},
		},
	}

	fdSet := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{depFd, mainFd},
	}
	resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(fdSet)
	if err != nil {
		t.Fatalf("Failed to create type resolver: %v", err)
	}

	config := pb.MetricsConfig_builder{
		DescriptorProtos: []*descriptorpb.FileDescriptorProto{depFd},
		ExpressionNodes: []*pb.Node{
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					SourceName: "my_data_source",
					FieldNames: []string{"msg_field"},
				}.Build(),
			}.Build(),
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					SourceName: "my_aggregator",
					FieldNames: []string{"nested_int"},
				}.Build(),
			}.Build(),
		},
		Sources: []*pb.Source{
			pb.Source_builder{
				Name: "my_data_source",
				DataSource: pb.DataSource_builder{
					SourceIdentifier: "my_data_source_identifier",
				}.Build(),
			}.Build(),
			pb.Source_builder{
				Name: "my_aggregator",
				Aggregator: pb.Aggregator_builder{
					MessageBuilder: pb.ProtoMessageBuilder_builder{
						MessageType: ".my.dep.package.NestedMessage",
					}.Build(),
				}.Build(),
			}.Build(),
			pb.Source_builder{
				Name: "my_second_aggregator",
				Aggregator: pb.Aggregator_builder{
					MessageBuilder: pb.ProtoMessageBuilder_builder{
						FieldAssignments: []*pb.ProtoMessageBuilder_FieldAssignment{
							pb.ProtoMessageBuilder_FieldAssignment_builder{
								FieldName: "nested_value",
								NoAggregation: pb.ProtoMessageBuilder_FieldAssignment_NoAggregation_builder{
									ExpressionNodeIndex: proto.Uint32(1),
								}.Build(),
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
		},
	}.Build()

	errs := inference.Infer(config, *resolver, map[string]string{
		"my_data_source_identifier": ".my.main.package.MyMessage",
	})
	if len(errs) > 0 {
		t.Fatalf("Infer() returned unexpected errors: %v", errs)
	}

	// We simply assert it succeeded without errors. If nested field traversal on the aggregator was unsupported,
	// Infer would have returned an error during ExpressionResolver.Resolve().
}

func TestInferExtractsProto3Syntax(t *testing.T) {
	tests := []struct {
		name       string
		mainSyntax string
		depSyntax  string
		wantErr    bool
	}{
		{
			name:       "proto3 main, proto3 dep",
			mainSyntax: "proto3",
			depSyntax:  "proto3",
		},
		{
			name:       "proto3 main, proto2 dep",
			mainSyntax: "proto3",
			depSyntax:  "proto2",
			// proto3 messages cannot have fields of proto2 enum types. This is
			// a protobuf limitation, not an MCG limitation.
			wantErr: true,
		},
		{
			name:       "proto2 main, proto3 dep",
			mainSyntax: "proto2",
			depSyntax:  "proto3",
		},
		{
			name:       "proto2 main, proto2 dep",
			mainSyntax: "proto2",
			depSyntax:  "proto2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			depFd := &descriptorpb.FileDescriptorProto{
				Name:    proto.String("dep.proto"),
				Package: proto.String("my.dep.package"),
				Syntax:  proto.String(tc.depSyntax),
				EnumType: []*descriptorpb.EnumDescriptorProto{
					{
						Name: proto.String("MyEnum"),
						Value: []*descriptorpb.EnumValueDescriptorProto{
							{Name: proto.String("UNKNOWN"), Number: proto.Int32(0)},
						},
					},
				},
			}
			sourceFd := &descriptorpb.FileDescriptorProto{
				Name:       proto.String("source.proto"),
				Package:    proto.String("my.source.package"),
				Syntax:     proto.String(tc.mainSyntax),
				Dependency: []string{"dep.proto"},
				MessageType: []*descriptorpb.DescriptorProto{
					{
						Name: proto.String("MyMessage"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:     proto.String("my_field"),
								Number:   proto.Int32(1),
								Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
								TypeName: proto.String(".my.dep.package.MyEnum"),
								Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
							},
						},
					},
				},
			}

			fdSet := &descriptorpb.FileDescriptorSet{
				File: []*descriptorpb.FileDescriptorProto{depFd, sourceFd},
			}
			resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(fdSet)
			if gotErr := err != nil; gotErr != tc.wantErr {
				t.Fatalf("NewEnrichedTypeResolverFromFileDescriptorSet() error = %v, want error presence = %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}

			config := pb.MetricsConfig_builder{
				ExpressionNodes: []*pb.Node{
					pb.Node_builder{
						FieldLeafNode: pb.FieldLeafNode_builder{
							SourceName: "my_data_source",
							FieldNames: []string{},
						}.Build(),
					}.Build(),
					pb.Node_builder{
						FieldLeafNode: pb.FieldLeafNode_builder{
							SourceName: "my_data_source",
							FieldNames: []string{"my_field"},
						}.Build(),
					}.Build(),
				},
				Sources: []*pb.Source{
					pb.Source_builder{
						Name: "my_data_source",
						DataSource: pb.DataSource_builder{
							SourceIdentifier: "my_data_source_identifier",
						}.Build(),
					}.Build(),
					pb.Source_builder{
						Name: "my_aggregator",
						Aggregator: pb.Aggregator_builder{
							MessageBuilder: pb.ProtoMessageBuilder_builder{
								FieldAssignments: []*pb.ProtoMessageBuilder_FieldAssignment{
									pb.ProtoMessageBuilder_FieldAssignment_builder{
										FieldName: "message",
										NoAggregation: pb.ProtoMessageBuilder_FieldAssignment_NoAggregation_builder{
											ExpressionNodeIndex: proto.Uint32(0),
										}.Build(),
									}.Build(),
									pb.ProtoMessageBuilder_FieldAssignment_builder{
										FieldName: "nested_enum",
										NoAggregation: pb.ProtoMessageBuilder_FieldAssignment_NoAggregation_builder{
											ExpressionNodeIndex: proto.Uint32(1),
										}.Build(),
									}.Build(),
								},
							}.Build(),
						}.Build(),
					}.Build(),
				},
			}.Build()

			errs := inference.Infer(config, *resolver, map[string]string{
				"my_data_source_identifier": ".my.source.package.MyMessage",
			})
			if len(errs) > 0 {
				t.Fatalf("Infer() returned unexpected errors: %v", errs)
			}

			wantProtos := []*descriptorpb.FileDescriptorProto{
				{
					Name:    proto.String("adhoc.proto"),
					Package: proto.String("aaos.sdv.telemetry.adhoc"),
					Dependency: []string{
						"dep.proto",
						"google/protobuf/any.proto",
					},
					MessageType: []*descriptorpb.DescriptorProto{
						{
							Name: proto.String("my_aggregator"),
							Field: []*descriptorpb.FieldDescriptorProto{
								{
									Name:     proto.String("message"),
									Number:   proto.Int32(1),
									Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
									TypeName: proto.String(".google.protobuf.Any"),
								},
								{
									Name:     proto.String("nested_enum"),
									Number:   proto.Int32(2),
									Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
									TypeName: proto.String(".my.dep.package.MyEnum"),
								},
							},
						},
					},
					// The adhoc descriptor should always be proto2.
					Syntax: proto.String("proto2"),
				},
				{
					Name:    proto.String("dep.proto"),
					Package: proto.String("my.dep.package"),
					EnumType: []*descriptorpb.EnumDescriptorProto{
						{
							Name: proto.String("MyEnum"),
							Value: []*descriptorpb.EnumValueDescriptorProto{
								{Name: proto.String("UNKNOWN"), Number: proto.Int32(0)},
							},
						},
					},
					// Other descriptors should use whatever syntax they were
					// using originally.
					Syntax: proto.String(tc.depSyntax),
				},
			}

			opts := []cmp.Option{
				cmpopts.SortSlices(func(a, b *descriptorpb.FileDescriptorProto) bool {
					return *a.Name < *b.Name
				}),
				protocmp.Transform(),
			}
			if diff := cmp.Diff(wantProtos, config.GetDescriptorProtos(), opts...); diff != "" {
				t.Errorf("Infer() returned unexpected DescriptorProtos diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNoDuplicateFileDescriptorsForTopLevelEnumAndMessage(t *testing.T) {
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("shared.proto"),
		Package: proto.String("my.package"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("MyMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("my_enum_field"),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
						TypeName: proto.String(".my.package.MyEnum"),
					},
				},
			},
		},
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: proto.String("MyEnum"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("UNKNOWN"), Number: proto.Int32(0)},
				},
			},
		},
	}

	fdSet := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fd},
	}
	resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(fdSet)
	if err != nil {
		t.Fatalf("Failed to create type resolver: %v", err)
	}

	config := pb.MetricsConfig_builder{
		ExpressionNodes: []*pb.Node{
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					SourceName: "my_data_source",
					FieldNames: []string{"my_enum_field"},
				}.Build(),
			}.Build(),
		},
		Sources: []*pb.Source{
			pb.Source_builder{
				Name: "my_data_source",
				DataSource: pb.DataSource_builder{
					SourceIdentifier: "my_data_source_identifier",
				}.Build(),
			}.Build(),
			pb.Source_builder{
				Name: "my_aggregator",
				Aggregator: pb.Aggregator_builder{
					MessageBuilder: pb.ProtoMessageBuilder_builder{
						FieldAssignments: []*pb.ProtoMessageBuilder_FieldAssignment{
							pb.ProtoMessageBuilder_FieldAssignment_builder{
								FieldName: "enum_val",
								NoAggregation: pb.ProtoMessageBuilder_FieldAssignment_NoAggregation_builder{
									ExpressionNodeIndex: proto.Uint32(0),
								}.Build(),
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
		},
	}.Build()

	errs := inference.Infer(config, *resolver, map[string]string{
		"my_data_source_identifier": ".my.package.MyMessage",
	})
	if len(errs) > 0 {
		t.Fatalf("Infer() returned unexpected errors: %v", errs)
	}

	count := 0
	for _, dp := range config.GetDescriptorProtos() {
		if dp.GetName() == "shared.proto" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Expected exactly 1 file descriptor for 'shared.proto', got %d", count)
	}
}

func TestExternalDependencyIsRetained(t *testing.T) {
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("main.proto"),
		Package: proto.String("my.main.package"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("MyMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("msg_field"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum(),
					},
				},
			},
		},
	}

	fdSet := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{fd}}
	resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(fdSet)
	if err != nil {
		t.Fatalf("Failed to create type resolver: %v", err)
	}

	config := pb.MetricsConfig_builder{
		ExpressionNodes: []*pb.Node{
			pb.Node_builder{
				ConstantLeafNode: pb.ConstantLeafNode_builder{
					BoolValue: proto.Bool(false),
				}.Build(),
			}.Build(),
		},
		Sources: []*pb.Source{
			pb.Source_builder{
				Name: "my_aggregator",
				Aggregator: pb.Aggregator_builder{
					MessageBuilder: pb.ProtoMessageBuilder_builder{
						MessageType: ".my.main.package.MyMessage",
					}.Build(),
				}.Build(),
			}.Build(),
		},
	}.Build()

	errs := inference.Infer(config, *resolver, make(map[string]string))
	if len(errs) > 0 {
		t.Fatalf("Infer() returned unexpected errors: %v", errs)
	}

	if !slices.ContainsFunc(config.GetDescriptorProtos(), func(fd *descriptorpb.FileDescriptorProto) bool {
		return fd.GetName() == "main.proto"
	}) {
		t.Errorf("Expected 'main.proto' to be retained because it depends on 'dep.proto', but it was pruned")
	}
}

func TestNestedMessageExternalDependencyIsRetained(t *testing.T) {
	depFd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("dep.proto"),
		Package: proto.String("my.dep.package"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("ExternalMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("external_int"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
					},
				},
			},
		},
	}
	mainFd := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("main.proto"),
		Package:    proto.String("my.main.package"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"dep.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("MyMessage"),
				NestedType: []*descriptorpb.DescriptorProto{
					{
						Name: proto.String("NestedMessage"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:     proto.String("external_field"),
								Number:   proto.Int32(1),
								Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
								TypeName: proto.String(".my.dep.package.ExternalMessage"),
							},
						},
					},
				},
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("nested_field"),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".my.main.package.MyMessage.NestedMessage"),
					},
				},
			},
		},
	}

	fdSet := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{depFd, mainFd},
	}
	resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(fdSet)
	if err != nil {
		t.Fatalf("Failed to create type resolver: %v", err)
	}

	config := pb.MetricsConfig_builder{
		ExpressionNodes: []*pb.Node{
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					SourceName: "my_data_source",
					FieldNames: []string{"msg_field"},
				}.Build(),
			}.Build(),
		},
		Sources: []*pb.Source{
			pb.Source_builder{
				Name: "my_data_source",
				DataSource: pb.DataSource_builder{
					SourceIdentifier: "my_data_source_identifier",
				}.Build(),
			}.Build(),
			pb.Source_builder{
				Name: "my_aggregator",
				Aggregator: pb.Aggregator_builder{
					MessageBuilder: pb.ProtoMessageBuilder_builder{
						MessageType: ".my.main.package.MyMessage",
					}.Build(),
				}.Build(),
			}.Build(),
		},
	}.Build()

	errs := inference.Infer(config, *resolver, map[string]string{
		"my_data_source_identifier": ".my.main.package.MyMessage",
	})
	if len(errs) > 0 {
		t.Fatalf("Infer() returned unexpected errors: %v", errs)
	}

	var gotNames []string
	for _, dp := range config.GetDescriptorProtos() {
		gotNames = append(gotNames, dp.GetName())
	}
	wantNames := []string{"dep.proto", "main.proto"}
	if diff := cmp.Diff(wantNames, gotNames); diff != "" {
		t.Errorf("Retained descriptors mismatch (-want +got):\n%s", diff)
	}
}

func TestAdhocMessageDeduplication(t *testing.T) {
	fdSet := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String("shared.proto"),
				Package: proto.String("my.package"),
				Syntax:  proto.String("proto3"),
				MessageType: []*descriptorpb.DescriptorProto{
					{
						Name: proto.String("MyMessage"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   proto.String("my_int_field"),
								Number: proto.Int32(1),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
							},
						},
					},
				},
			},
		},
	}
	resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(fdSet)
	if err != nil {
		t.Fatalf("Failed to create type resolver: %v", err)
	}

	config := pb.MetricsConfig_builder{
		ExpressionNodes: []*pb.Node{
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					SourceName: "my_data_source",
					FieldNames: []string{"my_int_field"},
				}.Build(),
			}.Build(),
		},
		Sources: []*pb.Source{
			pb.Source_builder{
				Name: "my_data_source",
				DataSource: pb.DataSource_builder{
					SourceIdentifier: "my_data_source_identifier",
				}.Build(),
			}.Build(),
			pb.Source_builder{
				Name: "agg1",
				Aggregator: pb.Aggregator_builder{
					MessageBuilder: pb.ProtoMessageBuilder_builder{
						FieldAssignments: []*pb.ProtoMessageBuilder_FieldAssignment{
							pb.ProtoMessageBuilder_FieldAssignment_builder{
								FieldName: "int_val",
								NoAggregation: pb.ProtoMessageBuilder_FieldAssignment_NoAggregation_builder{
									ExpressionNodeIndex: proto.Uint32(0),
								}.Build(),
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
			pb.Source_builder{
				Name: "agg2",
				Aggregator: pb.Aggregator_builder{
					MessageBuilder: pb.ProtoMessageBuilder_builder{
						FieldAssignments: []*pb.ProtoMessageBuilder_FieldAssignment{
							pb.ProtoMessageBuilder_FieldAssignment_builder{
								FieldName: "int_val",
								NoAggregation: pb.ProtoMessageBuilder_FieldAssignment_NoAggregation_builder{
									ExpressionNodeIndex: proto.Uint32(0),
								}.Build(),
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
		},
	}.Build()

	errs := inference.Infer(config, *resolver, map[string]string{
		"my_data_source_identifier": ".my.package.MyMessage",
	})
	if len(errs) > 0 {
		t.Fatalf("Infer() returned unexpected errors: %v", errs)
	}

	// agg1 and agg2 have identical message builders, so their schemas should be deduplicated.
	msgType1 := config.GetSources()[1].GetAggregator().GetMessageBuilder().GetMessageType()
	msgType2 := config.GetSources()[2].GetAggregator().GetMessageBuilder().GetMessageType()

	if msgType1 != msgType2 {
		t.Errorf("Expected aggregators to use the same deduplicated message type, got %q and %q", msgType1, msgType2)
	}
	if msgType1 != ".aaos.sdv.telemetry.adhoc.agg1" {
		t.Errorf("Expected message type to be .aaos.sdv.telemetry.adhoc.agg1, got %q", msgType1)
	}

	// Verify the adhoc descriptor only contains one message
	var adhocDp *descriptorpb.FileDescriptorProto
	for _, dp := range config.GetDescriptorProtos() {
		if dp.GetName() == "adhoc.proto" {
			adhocDp = dp
		}
	}
	if adhocDp == nil {
		t.Fatalf("adhoc.proto not found in output descriptors")
	}

	wantAdhocDp := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("adhoc.proto"),
		Package: proto.String("aaos.sdv.telemetry.adhoc"),
		Syntax:  proto.String("proto2"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("agg1"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("int_val"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
					},
				},
			},
		},
	}

	if diff := cmp.Diff(wantAdhocDp, adhocDp, protocmp.Transform()); diff != "" {
		t.Errorf("Adhoc FileDescriptorProto mismatch (-want +got):\n%s", diff)
	}
}

func TestInfer_MsgBuilderNodeAdhoc(t *testing.T) {
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("main.proto"),
		Package: proto.String("my.package"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("MyMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("speed"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_FLOAT.Enum(),
					},
				},
			},
		},
	}
	fdSet := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{fd}}
	resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(fdSet)
	if err != nil {
		t.Fatalf("Failed to create type resolver: %v", err)
	}

	config := pb.MetricsConfig_builder{
		ExpressionNodes: []*pb.Node{
			// Node 0: FieldLeafNode accessing speed (FLOAT)
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					SourceName: "my_data_source",
					FieldNames: []string{"speed"},
				}.Build(),
			}.Build(),
			// Node 1: ConstantLeafNode (INT32)
			pb.Node_builder{
				ConstantLeafNode: pb.ConstantLeafNode_builder{
					Int32Value: proto.Int32(100),
				}.Build(),
			}.Build(),
			// Node 2: Untyped MessageBuilderNode
			pb.Node_builder{
				MessageBuilderNode: pb.MessageBuilderNode_builder{
					FieldAssignments: []*pb.MessageBuilderNode_FieldAssignment{
						pb.MessageBuilderNode_FieldAssignment_builder{
							FieldName:           proto.String("inferred_speed"),
							ExpressionNodeIndex: proto.Uint32(0),
						}.Build(),
						pb.MessageBuilderNode_FieldAssignment_builder{
							FieldName:           proto.String("limit"),
							ExpressionNodeIndex: proto.Uint32(1),
						}.Build(),
					},
				}.Build(),
			}.Build(),
		},
		Sources: []*pb.Source{
			pb.Source_builder{
				Name: "my_data_source",
				DataSource: pb.DataSource_builder{
					SourceIdentifier: "my_data_source_identifier",
				}.Build(),
			}.Build(),
		},
	}.Build()

	errs := inference.Infer(config, *resolver, map[string]string{
		"my_data_source_identifier": ".my.package.MyMessage",
	})
	if len(errs) > 0 {
		t.Fatalf("Infer() returned unexpected errors: %v", errs)
	}

	mbNode := config.GetExpressionNodes()[2].GetMessageBuilderNode()
	wantMsgType := ".aaos.sdv.telemetry.adhoc.MsgBuilderNode2"
	if mbNode.GetMessageType() != wantMsgType {
		t.Errorf("Expected MessageType %q, got %q", wantMsgType, mbNode.GetMessageType())
	}

	var adhocDp *descriptorpb.FileDescriptorProto
	for _, dp := range config.GetDescriptorProtos() {
		if dp.GetName() == "adhoc.proto" {
			adhocDp = dp
			break
		}
	}
	if adhocDp == nil {
		t.Fatalf("adhoc.proto not found in output descriptor protos")
	}

	var foundMsg *descriptorpb.DescriptorProto
	for _, msg := range adhocDp.GetMessageType() {
		if msg.GetName() == "MsgBuilderNode2" {
			foundMsg = msg
			break
		}
	}
	if foundMsg == nil {
		t.Fatalf("MsgBuilderNode2 not found in adhoc.proto")
	}

	wantFields := []*descriptorpb.FieldDescriptorProto{
		{
			Name:   proto.String("inferred_speed"),
			Number: proto.Int32(1),
			Type:   descriptorpb.FieldDescriptorProto_TYPE_FLOAT.Enum(),
		},
		{
			Name:   proto.String("limit"),
			Number: proto.Int32(2),
			Type:   descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
		},
	}
	if diff := cmp.Diff(wantFields, foundMsg.GetField(), protocmp.Transform()); diff != "" {
		t.Errorf("Adhoc fields mismatch (-want +got):\n%s", diff)
	}
}

func TestInfer_MsgBuilderNodeDeduplication(t *testing.T) {
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("main.proto"),
		Package: proto.String("my.package"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("MyMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("val"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
					},
				},
			},
		},
	}
	fdSet := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{fd}}
	resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(fdSet)
	if err != nil {
		t.Fatalf("Failed to create type resolver: %v", err)
	}

	config := pb.MetricsConfig_builder{
		ExpressionNodes: []*pb.Node{
			// Node 0: ConstantLeafNode
			pb.Node_builder{
				ConstantLeafNode: pb.ConstantLeafNode_builder{
					Int32Value: proto.Int32(10),
				}.Build(),
			}.Build(),
			// Node 1: MessageBuilderNode
			pb.Node_builder{
				MessageBuilderNode: pb.MessageBuilderNode_builder{
					FieldAssignments: []*pb.MessageBuilderNode_FieldAssignment{
						pb.MessageBuilderNode_FieldAssignment_builder{
							FieldName:           proto.String("count"),
							ExpressionNodeIndex: proto.Uint32(0),
						}.Build(),
					},
				}.Build(),
			}.Build(),
			// Node 2: Another identical MessageBuilderNode
			pb.Node_builder{
				MessageBuilderNode: pb.MessageBuilderNode_builder{
					FieldAssignments: []*pb.MessageBuilderNode_FieldAssignment{
						pb.MessageBuilderNode_FieldAssignment_builder{
							FieldName:           proto.String("count"),
							ExpressionNodeIndex: proto.Uint32(0),
						}.Build(),
					},
				}.Build(),
			}.Build(),
		},
	}.Build()

	errs := inference.Infer(config, *resolver, make(map[string]string))
	if len(errs) > 0 {
		t.Fatalf("Infer() returned unexpected errors: %v", errs)
	}

	mb1 := config.GetExpressionNodes()[1].GetMessageBuilderNode().GetMessageType()
	mb2 := config.GetExpressionNodes()[2].GetMessageBuilderNode().GetMessageType()
	if mb1 != mb2 {
		t.Errorf("Expected identical MessageBuilderNodes to share deduplicated message type, got %q and %q", mb1, mb2)
	}
}

func TestInfer_MsgBuilderNodePredefined(t *testing.T) {
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("main.proto"),
		Package: proto.String("my.package"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("TargetMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("val"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
					},
				},
			},
		},
	}
	fdSet := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{fd}}
	resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(fdSet)
	if err != nil {
		t.Fatalf("Failed to create type resolver: %v", err)
	}

	config := pb.MetricsConfig_builder{
		ExpressionNodes: []*pb.Node{
			pb.Node_builder{
				MessageBuilderNode: pb.MessageBuilderNode_builder{
					MessageType: proto.String(".my.package.TargetMessage"),
				}.Build(),
			}.Build(),
		},
	}.Build()

	errs := inference.Infer(config, *resolver, make(map[string]string))
	if len(errs) > 0 {
		t.Fatalf("Infer() returned unexpected errors: %v", errs)
	}

	// Verify TargetMessage descriptor is retained in output descriptors
	found := false
	for _, dp := range config.GetDescriptorProtos() {
		for _, msg := range dp.GetMessageType() {
			if msg.GetName() == "TargetMessage" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("Expected TargetMessage to be retained in output descriptors")
	}
}

func TestInfer_ExpressionMessageBuilderPredefinedUnknownFails(t *testing.T) {
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("main.proto"),
		Package: proto.String("my.package"),
		Syntax:  proto.String("proto3"),
	}
	fdSet := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{fd}}
	resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(fdSet)
	if err != nil {
		t.Fatalf("type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet() error = %v, want nil", err)
	}

	config := pb.MetricsConfig_builder{
		ExpressionNodes: []*pb.Node{
			pb.Node_builder{
				MessageBuilderNode: pb.MessageBuilderNode_builder{
					MessageType: proto.String(".my.package.NonExistent"),
				}.Build(),
			}.Build(),
		},
	}.Build()

	errs := inference.Infer(config, *resolver, make(map[string]string))
	if len(errs) == 0 {
		t.Fatal("Infer() succeeded, want error")
	}
	const wantSubstr = `no definition found for message type ".my.package.NonExistent"`
	found := false
	for _, err := range errs {
		if strings.Contains(err.Error(), wantSubstr) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Infer() errors = %v, want error containing %q", errs, wantSubstr)
	}
}

func TestInfer_ExpressionMessageBuilderOutOfOrder(t *testing.T) {
	// Node 0: Parent MessageBuilder referencing child at Node 1
	// Node 1: Child MessageBuilder referencing leaf at Node 2
	// Node 2: Constant Leaf
	config := pb.MetricsConfig_builder{
		ExpressionNodes: []*pb.Node{
			pb.Node_builder{
				MessageBuilderNode: pb.MessageBuilderNode_builder{
					FieldAssignments: []*pb.MessageBuilderNode_FieldAssignment{
						pb.MessageBuilderNode_FieldAssignment_builder{
							FieldName:           proto.String("child"),
							ExpressionNodeIndex: proto.Uint32(1),
						}.Build(),
					},
				}.Build(),
			}.Build(),
			pb.Node_builder{
				MessageBuilderNode: pb.MessageBuilderNode_builder{
					FieldAssignments: []*pb.MessageBuilderNode_FieldAssignment{
						pb.MessageBuilderNode_FieldAssignment_builder{
							FieldName:           proto.String("val"),
							ExpressionNodeIndex: proto.Uint32(2),
						}.Build(),
					},
				}.Build(),
			}.Build(),
			pb.Node_builder{
				ConstantLeafNode: pb.ConstantLeafNode_builder{
					Int32Value: proto.Int32(123),
				}.Build(),
			}.Build(),
		},
	}.Build()

	resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(&descriptorpb.FileDescriptorSet{})
	if err != nil {
		t.Fatalf("type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet() error = %v, want nil", err)
	}
	errs := inference.Infer(config, *resolver, make(map[string]string))
	if len(errs) > 0 {
		t.Fatalf("Infer() error = %v, want none", errs)
	}

	node0Type := config.GetExpressionNodes()[0].GetMessageBuilderNode().GetMessageType()
	node1Type := config.GetExpressionNodes()[1].GetMessageBuilderNode().GetMessageType()
	if node0Type == "" || node1Type == "" {
		t.Errorf("MessageTypes = %q, %q, want both non-empty", node0Type, node1Type)
	}
}

func TestInfer_ExpressionMessageBuilderPreexistingAdhocName(t *testing.T) {
	// Create config where adhoc descriptor already exists with MsgBuilderNode0
	existingAdhoc := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("adhoc.proto"),
		Package: proto.String(inference.AdhocPackage),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("MsgBuilderNode0"),
			},
		},
	}

	config := pb.MetricsConfig_builder{
		DescriptorProtos: []*descriptorpb.FileDescriptorProto{existingAdhoc},
		ExpressionNodes: []*pb.Node{
			pb.Node_builder{
				MessageBuilderNode: pb.MessageBuilderNode_builder{
					FieldAssignments: []*pb.MessageBuilderNode_FieldAssignment{
						pb.MessageBuilderNode_FieldAssignment_builder{
							FieldName:           proto.String("x"),
							ExpressionNodeIndex: proto.Uint32(1),
						}.Build(),
					},
				}.Build(),
			}.Build(),
			pb.Node_builder{
				ConstantLeafNode: pb.ConstantLeafNode_builder{
					Int32Value: proto.Int32(10),
				}.Build(),
			}.Build(),
		},
	}.Build()

	resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(&descriptorpb.FileDescriptorSet{})
	if err != nil {
		t.Fatalf("type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet() error = %v, want nil", err)
	}
	errs := inference.Infer(config, *resolver, make(map[string]string))
	if len(errs) > 0 {
		t.Fatalf("Infer() error = %v, want none", errs)
	}

	mbType := config.GetExpressionNodes()[0].GetMessageBuilderNode().GetMessageType()
	expectedType := fmt.Sprintf(".%s.MsgBuilderNode0", inference.AdhocPackage)
	if mbType != expectedType {
		t.Errorf("MessageType = %q, want %q", mbType, expectedType)
	}
}

func TestInfer_MsgBuilderNodeInterleavedWithAggregators(t *testing.T) {
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("main.proto"),
		Package: proto.String("my.package"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("RawData"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("speed"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_FLOAT.Enum(),
					},
				},
			},
		},
	}
	fdSet := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{fd}}
	resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(fdSet)
	if err != nil {
		t.Fatalf("Failed to create type resolver: %v", err)
	}

	// Chain: agg_2 -> MB(1) -> agg_1 -> raw_source
	config := pb.MetricsConfig_builder{
		Sources: []*pb.Source{
			// raw data source
			pb.Source_builder{
				Name: "raw_source",
				DataSource: pb.DataSource_builder{
					SourceIdentifier: "raw_ident",
				}.Build(),
			}.Build(),
			// agg_1: aggregates raw_source.speed (expression node 2)
			pb.Source_builder{
				Name: "agg_1",
				Aggregator: pb.Aggregator_builder{
					TriggerNames: []string{"trig_1"},
					MessageBuilder: pb.ProtoMessageBuilder_builder{
						FieldAssignments: []*pb.ProtoMessageBuilder_FieldAssignment{
							pb.ProtoMessageBuilder_FieldAssignment_builder{
								FieldName: "agg1_speed",
								NoAggregation: pb.ProtoMessageBuilder_FieldAssignment_NoAggregation_builder{
									ExpressionNodeIndex: proto.Uint32(2),
								}.Build(),
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
			// agg_2: aggregates MB(1)
			pb.Source_builder{
				Name: "agg_2",
				Aggregator: pb.Aggregator_builder{
					TriggerNames: []string{"trig_2"},
					MessageBuilder: pb.ProtoMessageBuilder_builder{
						FieldAssignments: []*pb.ProtoMessageBuilder_FieldAssignment{
							pb.ProtoMessageBuilder_FieldAssignment_builder{
								FieldName: "agg2_msg",
								NoAggregation: pb.ProtoMessageBuilder_FieldAssignment_NoAggregation_builder{
									ExpressionNodeIndex: proto.Uint32(1),
								}.Build(),
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
		},
		ExpressionNodes: []*pb.Node{
			// Node 0: FieldLeafNode accessing agg_1.agg1_speed
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					SourceName: "agg_1",
					FieldNames: []string{"agg1_speed"},
				}.Build(),
			}.Build(),
			// Node 1: MessageBuilderNode assigning inner_speed = Node 0 (agg_1.agg1_speed)
			pb.Node_builder{
				MessageBuilderNode: pb.MessageBuilderNode_builder{
					FieldAssignments: []*pb.MessageBuilderNode_FieldAssignment{
						pb.MessageBuilderNode_FieldAssignment_builder{
							FieldName:           proto.String("inner_speed"),
							ExpressionNodeIndex: proto.Uint32(0),
						}.Build(),
					},
				}.Build(),
			}.Build(),
			// Node 2: FieldLeafNode accessing raw_source.speed
			pb.Node_builder{
				FieldLeafNode: pb.FieldLeafNode_builder{
					SourceName: "raw_source",
					FieldNames: []string{"speed"},
				}.Build(),
			}.Build(),
		},
	}.Build()

	errs := inference.Infer(config, *resolver, map[string]string{
		"raw_ident": ".my.package.RawData",
	})
	if len(errs) > 0 {
		t.Fatalf("Infer() returned unexpected errors: %v", errs)
	}

	// Verify agg_1 schema was inferred
	agg1Type := config.GetSources()[1].GetAggregator().GetMessageBuilder().GetMessageType()
	if agg1Type == "" {
		t.Error("agg_1 MessageType was not inferred")
	}

	// Verify MB(1) schema was inferred
	mb1Type := config.GetExpressionNodes()[1].GetMessageBuilderNode().GetMessageType()
	if mb1Type == "" {
		t.Error("MB(1) MessageType was not inferred")
	}

	// Verify agg_2 schema was inferred
	agg2Type := config.GetSources()[2].GetAggregator().GetMessageBuilder().GetMessageType()
	if agg2Type == "" {
		t.Error("agg_2 MessageType was not inferred")
	}
}

func TestInfer_MsgBuilderNodeCycleDetected(t *testing.T) {
	// Node 0: MB referencing Node 1
	// Node 1: MB referencing Node 0
	config := pb.MetricsConfig_builder{
		ExpressionNodes: []*pb.Node{
			pb.Node_builder{
				MessageBuilderNode: pb.MessageBuilderNode_builder{
					FieldAssignments: []*pb.MessageBuilderNode_FieldAssignment{
						pb.MessageBuilderNode_FieldAssignment_builder{
							FieldName:           proto.String("b"),
							ExpressionNodeIndex: proto.Uint32(1),
						}.Build(),
					},
				}.Build(),
			}.Build(),
			pb.Node_builder{
				MessageBuilderNode: pb.MessageBuilderNode_builder{
					FieldAssignments: []*pb.MessageBuilderNode_FieldAssignment{
						pb.MessageBuilderNode_FieldAssignment_builder{
							FieldName:           proto.String("a"),
							ExpressionNodeIndex: proto.Uint32(0),
						}.Build(),
					},
				}.Build(),
			}.Build(),
		},
	}.Build()

	resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(&descriptorpb.FileDescriptorSet{})
	if err != nil {
		t.Fatalf("Failed to create type resolver: %v", err)
	}

	errs := inference.Infer(config, *resolver, make(map[string]string))
	if len(errs) == 0 {
		t.Fatal("Infer() succeeded, want error for cycle")
	}
	const wantErr = "Cyclic dependency between sources and/or message builders detected"
	found := false
	for _, err := range errs {
		if strings.Contains(err.Error(), wantErr) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Infer() errors = %v, want error containing %q", errs, wantErr)
	}
}
