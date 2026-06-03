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
	"slices"
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
		Name:       proto.String("main.proto"),
		Package:    proto.String("my.main.package"),
		Syntax:     proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("MyMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("msg_field"),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum(),
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
