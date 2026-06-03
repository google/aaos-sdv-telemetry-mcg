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
	"maps"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestGetRootMessage(t *testing.T) {
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test.proto"),
		Package: proto.String("test"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Root"),
				NestedType: []*descriptorpb.DescriptorProto{
					{
						Name: proto.String("Nested"),
						NestedType: []*descriptorpb.DescriptorProto{
							{
								Name: proto.String("DeeplyNested"),
							},
						},
					},
				},
			},
		},
	}

	files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fd},
	})
	if err != nil {
		t.Fatalf("protodesc.NewFiles() failed: %v", err)
	}

	rootMsgDesc, err := files.FindDescriptorByName("test.Root")
	if err != nil {
		t.Fatalf("files.FindDescriptorByName(\"test.Root\") failed: %v", err)
	}
	rootMsg := rootMsgDesc.(protoreflect.MessageDescriptor)

	nestedMsgDesc, err := files.FindDescriptorByName("test.Root.Nested")
	if err != nil {
		t.Fatalf("files.FindDescriptorByName(\"test.Root.Nested\") failed: %v", err)
	}
	nestedMsg := nestedMsgDesc.(protoreflect.MessageDescriptor)

	deeplyNestedMsgDesc, err := files.FindDescriptorByName("test.Root.Nested.DeeplyNested")
	if err != nil {
		t.Fatalf("files.FindDescriptorByName(\"test.Root.Nested.DeeplyNested\") failed: %v", err)
	}
	deeplyNestedMsg := deeplyNestedMsgDesc.(protoreflect.MessageDescriptor)

	tests := []struct {
		name string
		msg  protoreflect.MessageDescriptor
		want protoreflect.MessageDescriptor
	}{
		{"root", rootMsg, rootMsg},
		{"nested", nestedMsg, rootMsg},
		{"deeply nested", deeplyNestedMsg, rootMsg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getRootMessage(tt.msg)
			if got.FullName() != tt.want.FullName() {
				t.Errorf("getRootMessage() = %v, want %v", got.FullName(), tt.want.FullName())
			}
		})
	}
}

func TestGetMessageDeps(t *testing.T) {
	dep1 := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("dep1.proto"),
		Package: proto.String("dep1"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Dep1Message")},
		},
	}
	dep2 := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("dep2.proto"),
		Package: proto.String("dep2"),
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: proto.String("Dep2Enum"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("VAL"), Number: proto.Int32(0)},
				},
			},
		},
	}
	dep3 := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("dep3.proto"),
		Package: proto.String("dep3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Dep3Message")},
		},
	}
	dep4 := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("dep4.proto"),
		Package: proto.String("dep4"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Dep4GroupMessage")},
		},
	}

	mainFd := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("main.proto"),
		Package:    proto.String("main"),
		Dependency: []string{"dep1.proto", "dep2.proto", "dep3.proto", "dep4.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("MainMessage"),
				NestedType: []*descriptorpb.DescriptorProto{
					{
						Name: proto.String("NestedMessage"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:     proto.String("nested_enum"),
								Number:   proto.Int32(1),
								Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
								TypeName: proto.String(".dep2.Dep2Enum"),
							},
						},
					},
					{
						Name: proto.String("GroupMessage"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:     proto.String("group_field"),
								Number:   proto.Int32(1),
								Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
								TypeName: proto.String(".dep4.Dep4GroupMessage"),
							},
						},
					},
				},
				OneofDecl: []*descriptorpb.OneofDescriptorProto{
					{Name: proto.String("my_oneof")},
				},
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("normal_msg"),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".dep1.Dep1Message"),
					},
					{
						Name:     proto.String("nested_msg"),
						Number:   proto.Int32(2),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".main.MainMessage.NestedMessage"),
					},
					{
						Name:       proto.String("oneof_msg"),
						Number:     proto.Int32(3),
						Type:       descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName:   proto.String(".dep3.Dep3Message"),
						OneofIndex: proto.Int32(0),
					},
					{
						Name:     proto.String("groupmessage"),
						Number:   proto.Int32(4),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_GROUP.Enum(),
						TypeName: proto.String(".main.MainMessage.GroupMessage"),
					},
				},
			},
		},
	}

	files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{dep1, dep2, dep3, dep4, mainFd},
	})
	if err != nil {
		t.Fatalf("protodesc.NewFiles() failed: %v", err)
	}

	tests := []struct {
		name         string
		msgName      protoreflect.FullName
		expectedDeps []string
	}{
		{
			name:    "main message with all constructs",
			msgName: "main.MainMessage",
			expectedDeps: []string{
				"main.proto",
				"dep1.proto",
				"dep2.proto",
				"dep3.proto",
				"dep4.proto",
			},
		},
		{
			name:    "nested message",
			msgName: "main.MainMessage.NestedMessage",
			expectedDeps: []string{
				"main.proto",
				"dep2.proto",
			},
		},
		{
			name:    "group message",
			msgName: "main.MainMessage.GroupMessage",
			expectedDeps: []string{
				"main.proto",
				"dep4.proto",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc, err := files.FindDescriptorByName(tt.msgName)
			if err != nil {
				t.Fatalf("FindDescriptorByName failed: %v", err)
			}
			msgDesc := desc.(protoreflect.MessageDescriptor)

			deps := make(map[string]struct{})
			getMessageDeps(msgDesc, deps)
			gotDeps := slices.Collect(maps.Keys(deps))

			opts := []cmp.Option{
				cmpopts.SortSlices(func(a, b string) bool { return a < b }),
			}
			if diff := cmp.Diff(tt.expectedDeps, gotDeps, opts...); diff != "" {
				t.Errorf("getMessageDeps() returned unexpected diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSortedFileDescriptorsNoDependencies(t *testing.T) {
	fds := make(map[string]*descriptorpb.FileDescriptorProto)
	fds["a.proto"] = &descriptorpb.FileDescriptorProto{Name: proto.String("a.proto")}
	fds["b.proto"] = &descriptorpb.FileDescriptorProto{Name: proto.String("b.proto")}

	got, err := sortedFileDescriptors(fds)
	if err != nil {
		t.Fatalf("sortFileDescriptors() error = %v", err)
	}
	gotNames := make([]string, 0, len(got))
	for _, fd := range got {
		gotNames = append(gotNames, fd.GetName())
	}

	want := []string{"a.proto", "b.proto"}
	opts := []cmp.Option{
		cmpopts.SortSlices(func(a, b string) bool { return a < b }),
	}
	if diff := cmp.Diff(want, gotNames, opts...); diff != "" {
		t.Errorf("sortFileDescriptors() returned unexpected files (-want +got):\n%s", diff)
	}
}

func TestSortedFileDescriptors(t *testing.T) {
	tests := []struct {
		name    string
		fds     map[string]*descriptorpb.FileDescriptorProto
		want    []string
		wantErr bool
	}{
		{
			name: "linear dependencies",
			fds: map[string]*descriptorpb.FileDescriptorProto{
				"a.proto": {Name: proto.String("a.proto"), Dependency: []string{"b.proto"}},
				"b.proto": {Name: proto.String("b.proto"), Dependency: []string{"c.proto"}},
				"c.proto": {Name: proto.String("c.proto")},
			},
			// C has no dependencies, B depends on C, A depends on B.
			want: []string{"c.proto", "b.proto", "a.proto"},
		},
		{
			name: "complex DAG",
			fds: map[string]*descriptorpb.FileDescriptorProto{
				"a.proto": {Name: proto.String("a.proto"), Dependency: []string{"b.proto", "c.proto"}},
				"b.proto": {Name: proto.String("b.proto"), Dependency: []string{"c.proto"}},
				"c.proto": {Name: proto.String("c.proto")},
			},
			// C has no dependencies. B depends on C. A depends on B and C.
			want: []string{"c.proto", "b.proto", "a.proto"},
		},
		{
			name: "cyclic dependencies",
			fds: map[string]*descriptorpb.FileDescriptorProto{
				"a.proto": {Name: proto.String("a.proto"), Dependency: []string{"b.proto"}},
				"b.proto": {Name: proto.String("b.proto"), Dependency: []string{"a.proto"}},
			},
			wantErr: true,
		},
		{
			name: "self cyclic dependency",
			fds: map[string]*descriptorpb.FileDescriptorProto{
				"a.proto": {Name: proto.String("a.proto"), Dependency: []string{"a.proto"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sortedFileDescriptors(tt.fds)
			if (err != nil) != tt.wantErr {
				t.Fatalf("sortFileDescriptors() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			gotNames := make([]string, 0, len(got))
			for _, fd := range got {
				gotNames = append(gotNames, fd.GetName())
			}
			if diff := cmp.Diff(tt.want, gotNames); diff != "" {
				t.Errorf("sortFileDescriptors() returned unexpected order (-want +got):\n%s", diff)
			}
		})
	}
}
