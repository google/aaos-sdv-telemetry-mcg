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

package type_resolvers_test

import (
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"sdv.googlesource.com/mcg/mcg/testdata"
	"sdv.googlesource.com/mcg/mcg/testhelper"
	"sdv.googlesource.com/mcg/mcg/type_resolvers"
)

const (
	// Should match a message in /mcg/testdata/vehicle_speed.proto.
	SPEED_MSG_FULL_NAME = protoreflect.FullName("android.sdv.telemetry.mcg.testdata.VehicleSpeed")

	// Should match a message in /mcg/testdata/maxavgcur.proto.
	MAXAVGCUR_MSG_FULL_NAME = protoreflect.FullName("android.sdv.telemetry.mcg.testdata.MaxAvgCur")
)

func NewTypeResolver(t *testing.T, fdSet *descriptorpb.FileDescriptorSet) *type_resolvers.EnrichedTypeResolver {
	resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(fdSet)
	if err != nil {
		t.Fatal(err)
	}
	return resolver
}

func TestStoredLocalTypeCanBeFoundViaResolver(t *testing.T) {
	speedFd := testhelper.GetSpeedFdWithDependencies()
	resolver := NewTypeResolver(t, speedFd)

	// Can be accessed through the EnrichedTypeResolver.
	msgTypeFromLocalTypes, err := resolver.FindMessageByName(SPEED_MSG_FULL_NAME)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := SPEED_MSG_FULL_NAME, msgTypeFromLocalTypes.Descriptor().FullName(); want != got {
		t.Errorf("Type was not properly saved into the resolver's types: %s != %s", want, got)
	}
}

func TestStoredLocalTypeCanBeFoundViaResolversLocalTypes(t *testing.T) {
	speedFd := testhelper.GetSpeedFdWithDependencies()
	resolver := NewTypeResolver(t, speedFd)

	// Will be saved as part of the local types (not global) of the EnrichedTypeResolver.
	msgTypeFromLocalTypes, err := resolver.Local.FindMessageByName(SPEED_MSG_FULL_NAME)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := SPEED_MSG_FULL_NAME, msgTypeFromLocalTypes.Descriptor().FullName(); want != got {
		t.Errorf("Type was not properly saved into the resolver's local types: %s != %s", want, got)
	}
}

func TestNewTypeResolverFromFileDescriptorProtosContainsGivenMessages(t *testing.T) {
	fdSet := testhelper.MergeFileDescriptorSets(
		testdata.MaxavgcurFileDescriptorSet,
		testdata.SpeedFileDescriptorSet,
		testdata.VehicleSignalsSampleFileDescriptorSet,
	)

	resolver := NewTypeResolver(t, fdSet)

	// Without dots
	if _, err := resolver.FindMessageByName(SPEED_MSG_FULL_NAME); err != nil {
		t.Fatalf("%s message should be found.", SPEED_MSG_FULL_NAME)
	}
	if _, err := resolver.FindMessageByName(MAXAVGCUR_MSG_FULL_NAME); err != nil {
		t.Fatalf("%s message should be found.", MAXAVGCUR_MSG_FULL_NAME)
	}

	// With dots
	speedFullNameWithDot := protoreflect.FullName(fmt.Sprintf(".%s", string(SPEED_MSG_FULL_NAME)))
	maxavgcurFullNameWithDot := protoreflect.FullName(fmt.Sprintf(".%s", string(MAXAVGCUR_MSG_FULL_NAME)))

	if _, err := resolver.FindMessageByName(speedFullNameWithDot); err != nil {
		t.Fatalf("%s message should be found.", speedFullNameWithDot)
	}
	if _, err := resolver.FindMessageByName(maxavgcurFullNameWithDot); err != nil {
		t.Fatalf("%s message should be found.", maxavgcurFullNameWithDot)
	}
}

func TestExtendLocalTypesWithNewMessage(t *testing.T) {
	speedFd := testhelper.GetSpeedFdWithDependencies()
	resolver := NewTypeResolver(t, speedFd)

	if err := resolver.ExtendLocalTypes(testdata.MaxavgcurFileDescriptorSet); err != nil {
		t.Fatalf("ExtendLocalTypes failed unexpectedly: %v", err)
	}

	if _, err := resolver.Local.FindMessageByName(MAXAVGCUR_MSG_FULL_NAME); err != nil {
		t.Fatalf("%s message should be found after extending local types.", MAXAVGCUR_MSG_FULL_NAME)
	}
}

func TestExtendLocalTypesWithDuplicateMessageDoesNotFail(t *testing.T) {
	resolver := NewTypeResolver(t, &descriptorpb.FileDescriptorSet{})

	fdSet := testhelper.GetSpeedFdWithDependencies()
	// Call ExtendLocalTypes twice with the same fdSet to simulate a duplicate.
	if err := resolver.ExtendLocalTypes(fdSet); err != nil {
		t.Fatalf("First call to ExtendLocalTypes failed unexpectedly: %v", err)
	}
	if err := resolver.ExtendLocalTypes(fdSet); err != nil {
		t.Fatalf("Second call to ExtendLocalTypes should not fail on duplicate message: %v", err)
	}
}

func TestExtendLocalTypesWithConflictingMessageFails(t *testing.T) {
	resolver := NewTypeResolver(t, &descriptorpb.FileDescriptorSet{})

	speedFd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("mcg/testdata/speed/vehicle_speed.proto"),
		Package: proto.String("android.sdv.telemetry.mcg.testdata"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("VehicleSpeed"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("speed"),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_FLOAT.Enum(),
						JsonName: proto.String("speed"),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
	}

	conflictingFdProto := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("mcg/testdata/speed/vehicle_speed.proto"),
		Package: proto.String("android.sdv.telemetry.mcg.testdata"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("VehicleSpeed"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("speed"),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_FLOAT.Enum(),
						JsonName: proto.String("wrong :("),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
	}

	if err := resolver.ExtendLocalTypes(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{speedFd}}); err != nil {
		t.Fatalf("First call to ExtendLocalTypes failed unexpectedly: %v", err)
	}

	if err := resolver.ExtendLocalTypes(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{conflictingFdProto}}); err == nil {
		t.Fatal("Second call to ExtendLocalTypes with a conflicting message should have failed, but it did not.")
	} else {
		if !strings.Contains(err.Error(), "with a different definition is already registered") {
			t.Fatalf("Expected error to contain 'with a different definition is already registered', but got: %v", err)
		}
	}
}

func TestNewEnrichedTypeResolverCollectsNestedMessages(t *testing.T) {
	inputSchemaText := `
name: "my_package.proto"
package: "my.package"
syntax: "proto2"
message_type: {
  name: "Outer"
  nested_type: {
    name: "Inner"
    field: {
      name: "nested_value"
      number: 1
      label: LABEL_OPTIONAL
      type: TYPE_INT32
    }
  }
}
`
	var fdProto descriptorpb.FileDescriptorProto
	if err := prototext.Unmarshal([]byte(inputSchemaText), &fdProto); err != nil {
		t.Fatalf("Failed to unmarshal input schema textproto: %v", err)
	}

	resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(&descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{&fdProto},
	})
	if err != nil {
		t.Fatalf("Failed to construct EnrichedTypeResolver: %v", err)
	}

	// Verify the top-level Outer message is successfully resolved.
	outerFullName := protoreflect.FullName("my.package.Outer")
	if _, err := resolver.FindMessageByName(outerFullName); err != nil {
		t.Errorf("Expected message %q to be resolved, but got: %v", outerFullName, err)
	}

	// Verify the nested Inner message is successfully resolved!
	innerFullName := protoreflect.FullName("my.package.Outer.Inner")
	if _, err := resolver.FindMessageByName(innerFullName); err != nil {
		t.Errorf("Expected nested message %q to be resolved (resolver fix!), but got: %v", innerFullName, err)
	}
}

func TestNewEnrichedTypeResolverCollectsEnums(t *testing.T) {
	inputSchemaText := `
name: "my_package.proto"
package: "my.package"
syntax: "proto2"
enum_type: {
  name: "TopLevelEnum"
  value: {
    name: "A"
    number: 1
  }
}
message_type: {
  name: "Outer"
  enum_type: {
    name: "NestedEnum"
    value: {
      name: "B"
      number: 1
    }
  }
}
`
	var fdProto descriptorpb.FileDescriptorProto
	if err := prototext.Unmarshal([]byte(inputSchemaText), &fdProto); err != nil {
		t.Fatalf("Failed to unmarshal input schema textproto: %v", err)
	}

	resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(&descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{&fdProto},
	})
	if err != nil {
		t.Fatalf("Failed to construct EnrichedTypeResolver: %v", err)
	}

	// Verify the top-level enum is successfully resolved.
	topLevelEnumName := protoreflect.FullName("my.package.TopLevelEnum")
	if _, err := resolver.FindEnumByName(topLevelEnumName); err != nil {
		t.Errorf("Expected top-level enum %q to be resolved, but got: %v", topLevelEnumName, err)
	}

	// Verify the nested enum is successfully resolved.
	nestedEnumName := protoreflect.FullName("my.package.Outer.NestedEnum")
	if _, err := resolver.FindEnumByName(nestedEnumName); err != nil {
		t.Errorf("Expected nested enum %q to be resolved, but got: %v", nestedEnumName, err)
	}
}

func TestExtendLocalTypesWithEnumMessageCollision(t *testing.T) {
	msgSchemaText := `
name: "my_package_msg.proto"
package: "my.package"
syntax: "proto2"
message_type: {
  name: "Collision"
}
`
	enumSchemaText := `
name: "my_package_enum.proto"
package: "my.package"
syntax: "proto2"
enum_type: {
  name: "Collision"
  value: {
    name: "A"
    number: 1
  }
}
`

	var msgProto, enumProto descriptorpb.FileDescriptorProto
	if err := prototext.Unmarshal([]byte(msgSchemaText), &msgProto); err != nil {
		t.Fatalf("Failed to unmarshal msg schema textproto: %v", err)
	}
	if err := prototext.Unmarshal([]byte(enumSchemaText), &enumProto); err != nil {
		t.Fatalf("Failed to unmarshal enum schema textproto: %v", err)
	}

	tests := []struct {
		name       string
		first      *descriptorpb.FileDescriptorProto
		second     *descriptorpb.FileDescriptorProto
		wantErrMsg string
	}{
		{
			name:       "Register message then enum",
			first:      &msgProto,
			second:     &enumProto,
			wantErrMsg: "is already registered, but not as an enum type",
		},
		{
			name:       "Register enum then message",
			first:      &enumProto,
			second:     &msgProto,
			wantErrMsg: "is already registered, but not as a message type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(&descriptorpb.FileDescriptorSet{
				File: []*descriptorpb.FileDescriptorProto{tt.first},
			})
			if err != nil {
				t.Fatalf("Failed to construct EnrichedTypeResolver: %v", err)
			}

			err = resolver.ExtendLocalTypes(&descriptorpb.FileDescriptorSet{
				File: []*descriptorpb.FileDescriptorProto{tt.second},
			})
			if err == nil {
				t.Fatal("Expected error when registering conflicting type, but got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("Expected error to mention %q, but got: %v", tt.wantErrMsg, err)
			}
		})
	}
}
