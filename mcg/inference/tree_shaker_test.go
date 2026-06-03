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
	"testing"

	"github.com/google/go-cmp/cmp"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestTreeShake(t *testing.T) {
	dep1 := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("dep1.proto"),
		Package: proto.String("dep1"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("UsedMessage")},
			{Name: proto.String("UnusedMessage")},
		},
	}
	dep2 := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("dep2.proto"),
		Package: proto.String("dep2"),
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: proto.String("UsedEnum"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("VAL"), Number: proto.Int32(0)},
				},
			},
			{
				Name: proto.String("UnusedEnum"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("UNVAL"), Number: proto.Int32(0)},
				},
			},
		},
	}
	wkt := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("google/protobuf/timestamp.proto"),
		Package: proto.String("google.protobuf"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Timestamp")},
		},
	}
	unusedDep := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("unused_dep.proto"),
		Package: proto.String("unused_dep"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("CompletelyUnused")},
		},
	}
	mainFd := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("main.proto"),
		Package:    proto.String("main"),
		Dependency: []string{"dep1.proto", "dep2.proto", "google/protobuf/timestamp.proto", "unused_dep.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("MainMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("used_msg"),
						Number:   proto.Int32(1),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".dep1.UsedMessage"),
					},
					{
						Name:     proto.String("used_enum"),
						Number:   proto.Int32(2),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
						TypeName: proto.String(".dep2.UsedEnum"),
					},
					{
						Name:     proto.String("ts"),
						Number:   proto.Int32(3),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".google.protobuf.Timestamp"),
					},
				},
			},
		},
	}

	files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{dep1, dep2, wkt, unusedDep, mainFd},
	})
	if err != nil {
		t.Fatalf("protodesc.NewFiles() failed: %v", err)
	}

	mainDesc, err := files.FindDescriptorByName("main.MainMessage")
	if err != nil {
		t.Fatalf("FindDescriptorByName failed: %v", err)
	}

	msgs := map[protoreflect.FullName]protoreflect.MessageDescriptor{
		mainDesc.FullName(): mainDesc.(protoreflect.MessageDescriptor),
	}
	enums := map[protoreflect.FullName]protoreflect.EnumDescriptor{}

	got, err := TreeShake(msgs, enums)
	if err != nil {
		t.Fatalf("TreeShake() failed: %v", err)
	}

	wantDep1 := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("dep1.proto"),
		Package: proto.String("dep1"),
		Syntax:  proto.String("proto2"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("UsedMessage")},
		},
	}
	wantDep2 := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("dep2.proto"),
		Package: proto.String("dep2"),
		Syntax:  proto.String("proto2"),
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: proto.String("UsedEnum"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("VAL"), Number: proto.Int32(0)},
				},
			},
		},
	}
	wantMain := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("main.proto"),
		Package:    proto.String("main"),
		Syntax:     proto.String("proto2"),
		Dependency: []string{"dep1.proto", "dep2.proto", "google/protobuf/timestamp.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("MainMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("used_msg"),
						Number:   proto.Int32(1),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".dep1.UsedMessage"),
					},
					{
						Name:     proto.String("used_enum"),
						Number:   proto.Int32(2),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
						TypeName: proto.String(".dep2.UsedEnum"),
					},
					{
						Name:     proto.String("ts"),
						Number:   proto.Int32(3),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".google.protobuf.Timestamp"),
					},
				},
			},
		},
	}

	want := []*descriptorpb.FileDescriptorProto{wantDep2, wantDep1, wantMain}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("TreeShake() topological order mismatch (-want +got):\n%s", diff)
	}
}

func TestTreeShake_HarvestEnum(t *testing.T) {
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("main.proto"),
		Package: proto.String("main"),
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: proto.String("MyEnum"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("VAL"), Number: proto.Int32(0)},
				},
			},
			{
				Name: proto.String("UnusedEnum"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("UNVAL"), Number: proto.Int32(0)},
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

	enumDesc, err := files.FindDescriptorByName("main.MyEnum")
	if err != nil {
		t.Fatalf("FindDescriptorByName failed: %v", err)
	}

	msgs := map[protoreflect.FullName]protoreflect.MessageDescriptor{}
	enums := map[protoreflect.FullName]protoreflect.EnumDescriptor{
		enumDesc.FullName(): enumDesc.(protoreflect.EnumDescriptor),
	}

	got, err := TreeShake(msgs, enums)
	if err != nil {
		t.Fatalf("TreeShake() failed: %v", err)
	}

	want := []*descriptorpb.FileDescriptorProto{
		{
			Name:    proto.String("main.proto"),
			Package: proto.String("main"),
			Syntax:  proto.String("proto2"),
			EnumType: []*descriptorpb.EnumDescriptorProto{
				{
					Name: proto.String("MyEnum"),
					Value: []*descriptorpb.EnumValueDescriptorProto{
						{Name: proto.String("VAL"), Number: proto.Int32(0)},
					},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("TreeShake() mismatch (-want +got):\n%s", diff)
	}
}

func TestTreeShake_NestedMessage(t *testing.T) {
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("main.proto"),
		Package: proto.String("main"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("RootMessage"),
				NestedType: []*descriptorpb.DescriptorProto{
					{
						Name: proto.String("NestedMessage"),
					},
					{
						Name: proto.String("UnusedNestedMessage"),
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

	msgDesc, err := files.FindDescriptorByName("main.RootMessage.NestedMessage")
	if err != nil {
		t.Fatalf("FindDescriptorByName failed: %v", err)
	}

	msgs := map[protoreflect.FullName]protoreflect.MessageDescriptor{
		msgDesc.FullName(): msgDesc.(protoreflect.MessageDescriptor),
	}
	enums := map[protoreflect.FullName]protoreflect.EnumDescriptor{}

	got, err := TreeShake(msgs, enums)
	if err != nil {
		t.Fatalf("TreeShake() failed: %v", err)
	}

	want := []*descriptorpb.FileDescriptorProto{
		{
			Name:    proto.String("main.proto"),
			Package: proto.String("main"),
			Syntax:  proto.String("proto2"),
			MessageType: []*descriptorpb.DescriptorProto{
				{
					Name: proto.String("RootMessage"),
					NestedType: []*descriptorpb.DescriptorProto{
						{
							Name: proto.String("NestedMessage"),
						},
						{
							Name: proto.String("UnusedNestedMessage"),
						},
					},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("TreeShake() mismatch (-want +got):\n%s", diff)
	}
}

func TestTreeShake_NestedEnum(t *testing.T) {
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("main.proto"),
		Package: proto.String("main"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("RootMessage"),
				EnumType: []*descriptorpb.EnumDescriptorProto{
					{
						Name: proto.String("NestedEnum"),
						Value: []*descriptorpb.EnumValueDescriptorProto{
							{Name: proto.String("VAL"), Number: proto.Int32(0)},
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

	enumDesc, err := files.FindDescriptorByName("main.RootMessage.NestedEnum")
	if err != nil {
		t.Fatalf("FindDescriptorByName failed: %v", err)
	}

	msgs := map[protoreflect.FullName]protoreflect.MessageDescriptor{}
	enums := map[protoreflect.FullName]protoreflect.EnumDescriptor{
		enumDesc.FullName(): enumDesc.(protoreflect.EnumDescriptor),
	}

	got, err := TreeShake(msgs, enums)
	if err != nil {
		t.Fatalf("TreeShake() failed: %v", err)
	}

	want := []*descriptorpb.FileDescriptorProto{
		{
			Name:    proto.String("main.proto"),
			Package: proto.String("main"),
			Syntax:  proto.String("proto2"),
			MessageType: []*descriptorpb.DescriptorProto{
				{
					Name: proto.String("RootMessage"),
					EnumType: []*descriptorpb.EnumDescriptorProto{
						{
							Name: proto.String("NestedEnum"),
							Value: []*descriptorpb.EnumValueDescriptorProto{
								{Name: proto.String("VAL"), Number: proto.Int32(0)},
							},
						},
					},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("TreeShake() mismatch (-want +got):\n%s", diff)
	}
}

func TestTreeShake_PublicImports(t *testing.T) {
	// file_c defines the message.
	fdC := &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String("c.proto"),
		Package: proto.String("pkg.c"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("MessageC")},
		},
	}
	// file_b public-imports file_c, and defines an unused message.
	fdB := &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String("b.proto"),
		Package: proto.String("pkg.b"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("UnusedMessageB")},
		},
		Dependency: []string{"c.proto"},
		PublicDependency: []int32{0},
	}
	// file_a imports file_b, and uses MessageC (available via public import).
	fdA := &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String("a.proto"),
		Package: proto.String("pkg.a"),
		Dependency: []string{"b.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("MessageA"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("c_msg"),
						Number:   proto.Int32(1),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".pkg.c.MessageC"),
					},
				},
			},
		},
	}

	files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fdC, fdB, fdA},
	})
	if err != nil {
		t.Fatalf("protodesc.NewFiles() failed: %v", err)
	}

	msgDesc, err := files.FindDescriptorByName("pkg.a.MessageA")
	if err != nil {
		t.Fatalf("FindDescriptorByName failed: %v", err)
	}

	got, err := TreeShake(
		map[protoreflect.FullName]protoreflect.MessageDescriptor{
			msgDesc.FullName(): msgDesc.(protoreflect.MessageDescriptor),
		},
		nil,
	)
	if err != nil {
		t.Fatalf("TreeShake() failed: %v", err)
	}

	// We expect c.proto, b.proto, a.proto to all be retained, and their
	// dependencies and public dependencies correctly preserved.
	wantC := fdC
	wantB := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("b.proto"),
		Package: proto.String("pkg.b"),
		Syntax:  proto.String("proto3"),
		Dependency: []string{"c.proto"},
		PublicDependency: []int32{0},
		// The unused message has been removed
	}
	wantA := fdA
	want := []*descriptorpb.FileDescriptorProto{wantC, wantB, wantA}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("TreeShake() mismatch with public dependencies (-want +got):\n%s", diff)
	}
}

func TestTreeShake_PublicDependencyIndexShift(t *testing.T) {
	// target.proto defines a message that we'll actually use.
	fdTarget := &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String("target.proto"),
		Package: proto.String("pkg.target"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("TargetMessage")},
		},
	}
	// public.proto defines a message and will be a public dependency.
	fdPublic := &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String("public.proto"),
		Package: proto.String("pkg.public"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("PublicMessage")},
		},
	}
	// unused.proto will be completely ignored.
	fdUnused := &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String("unused.proto"),
		Package: proto.String("pkg.unused"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("UnusedMessage")},
		},
	}
	// main.proto has dependencies: [unused.proto, public.proto, target.proto]
	// It public-imports public.proto (index 1).
	// It privately imports unused.proto (index 0) and target.proto (index 2).
	// It uses target.proto's message, so target.proto is retained.
	// unused.proto is unused and should be pruned.
	fdMain := &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String("main.proto"),
		Package: proto.String("pkg.main"),
		Dependency: []string{"unused.proto", "public.proto", "target.proto"},
		PublicDependency: []int32{1},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("MainMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("target_msg"),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".pkg.target.TargetMessage"),
					},
				},
			},
		},
	}

	files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fdTarget, fdPublic, fdUnused, fdMain},
	})
	if err != nil {
		t.Fatalf("protodesc.NewFiles() failed: %v", err)
	}

	msgDesc, err := files.FindDescriptorByName("pkg.main.MainMessage")
	if err != nil {
		t.Fatalf("FindDescriptorByName failed: %v", err)
	}

	got, err := TreeShake(
		map[protoreflect.FullName]protoreflect.MessageDescriptor{
			msgDesc.FullName(): msgDesc.(protoreflect.MessageDescriptor),
		},
		nil,
	)
	if err != nil {
		t.Fatalf("TreeShake() failed: %v", err)
	}

	// Since unused.proto is pruned, the new dependency list for main.proto should be:
	// ["public.proto", "target.proto"]
	// Its PublicDependency index should shift from 1 to 0!
	wantTarget := fdTarget
	wantPublic := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("public.proto"),
		Package: proto.String("pkg.public"),
		Syntax:  proto.String("proto3"),
		// The unused message has been removed.
	}
	wantMain := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("main.proto"),
		Package: proto.String("pkg.main"),
		Syntax:  proto.String("proto3"),
		// Dependencies and public dependencies have been updated.
		Dependency: []string{"public.proto", "target.proto"},
		PublicDependency: []int32{0},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("MainMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("target_msg"),
						Number:   proto.Int32(1),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".pkg.target.TargetMessage"),
					},
				},
			},
		},
	}

	want := []*descriptorpb.FileDescriptorProto{wantTarget, wantPublic, wantMain}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("TreeShake() full output mismatch (-want +got):\n%s", diff)
	}
}

func TestTreeShake_Stability(t *testing.T) {
	// Define a file with 10 messages and 10 enums. The number of permutations
	// is 10! * 10! ≈ 1.3e13. If TreeShake were non-deterministic (e.g. relying
	// on map iteration order), the chance of it producing the exact sorted
	// output is extremely small (~1 in 13 trillion).
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test.proto"),
		Package: proto.String("test"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Msg1")},
			{Name: proto.String("Msg2")},
			{Name: proto.String("Msg3")},
			{Name: proto.String("Msg4")},
			{Name: proto.String("Msg5")},
			{Name: proto.String("Msg6")},
			{Name: proto.String("Msg7")},
			{Name: proto.String("Msg8")},
			{Name: proto.String("Msg9")},
			{Name: proto.String("Msg10")},
		},
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{Name: proto.String("Enum1"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E1"), Number: proto.Int32(0)}}},
			{Name: proto.String("Enum2"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E2"), Number: proto.Int32(0)}}},
			{Name: proto.String("Enum3"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E3"), Number: proto.Int32(0)}}},
			{Name: proto.String("Enum4"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E4"), Number: proto.Int32(0)}}},
			{Name: proto.String("Enum5"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E5"), Number: proto.Int32(0)}}},
			{Name: proto.String("Enum6"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E6"), Number: proto.Int32(0)}}},
			{Name: proto.String("Enum7"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E7"), Number: proto.Int32(0)}}},
			{Name: proto.String("Enum8"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E8"), Number: proto.Int32(0)}}},
			{Name: proto.String("Enum9"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E9"), Number: proto.Int32(0)}}},
			{Name: proto.String("Enum10"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E10"), Number: proto.Int32(0)}}},
		},
	}
	files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fd},
	})
	if err != nil {
		t.Fatalf("protodesc.NewFiles() failed: %v", err)
	}

	indices := []int{5, 3, 8, 1, 10, 2, 7, 9, 4, 6}

	msgs := make(map[protoreflect.FullName]protoreflect.MessageDescriptor)
	for _, i := range indices {
		name := protoreflect.FullName(fmt.Sprintf("test.Msg%d", i))
		desc, err := files.FindDescriptorByName(name)
		if err != nil {
			t.Fatalf("FindDescriptorByName(%q) failed: %v", name, err)
		}
		msgs[desc.FullName()] = desc.(protoreflect.MessageDescriptor)
	}

	enums := make(map[protoreflect.FullName]protoreflect.EnumDescriptor)
	for _, i := range indices {
		name := protoreflect.FullName(fmt.Sprintf("test.Enum%d", i))
		desc, err := files.FindDescriptorByName(name)
		if err != nil {
			t.Fatalf("FindDescriptorByName(%q) failed: %v", name, err)
		}
		enums[desc.FullName()] = desc.(protoreflect.EnumDescriptor)
	}

	got, err := TreeShake(msgs, enums)
	if err != nil {
		t.Fatalf("TreeShake() failed: %v", err)
	}

	// Expected output must be sorted lexicographically by FullName:
	// Msg1, Msg10, Msg2, Msg3, Msg4, Msg5, Msg6, Msg7, Msg8, Msg9
	// Enum1, Enum10, Enum2, Enum3, Enum4, Enum5, Enum6, Enum7, Enum8, Enum9
	want := []*descriptorpb.FileDescriptorProto{
		{
			Name:    proto.String("test.proto"),
			Package: proto.String("test"),
			Syntax:  proto.String("proto3"),
			MessageType: []*descriptorpb.DescriptorProto{
				{Name: proto.String("Msg1")},
				{Name: proto.String("Msg10")},
				{Name: proto.String("Msg2")},
				{Name: proto.String("Msg3")},
				{Name: proto.String("Msg4")},
				{Name: proto.String("Msg5")},
				{Name: proto.String("Msg6")},
				{Name: proto.String("Msg7")},
				{Name: proto.String("Msg8")},
				{Name: proto.String("Msg9")},
			},
			EnumType: []*descriptorpb.EnumDescriptorProto{
				{Name: proto.String("Enum1"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E1"), Number: proto.Int32(0)}}},
				{Name: proto.String("Enum10"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E10"), Number: proto.Int32(0)}}},
				{Name: proto.String("Enum2"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E2"), Number: proto.Int32(0)}}},
				{Name: proto.String("Enum3"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E3"), Number: proto.Int32(0)}}},
				{Name: proto.String("Enum4"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E4"), Number: proto.Int32(0)}}},
				{Name: proto.String("Enum5"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E5"), Number: proto.Int32(0)}}},
				{Name: proto.String("Enum6"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E6"), Number: proto.Int32(0)}}},
				{Name: proto.String("Enum7"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E7"), Number: proto.Int32(0)}}},
				{Name: proto.String("Enum8"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E8"), Number: proto.Int32(0)}}},
				{Name: proto.String("Enum9"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("E9"), Number: proto.Int32(0)}}},
			},
		},
	}

	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("TreeShake() output mismatch (-want +got):\n%s", diff)
	}
}
