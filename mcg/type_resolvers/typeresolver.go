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

// Contains a metrics-configs adjusted wrapper for `protoregistry.MessageTypeResolver` which is for
// example aware of the `supported_protobuf_deps` and can be enriched with new types parsed from for
// example Vehicle Signals (VSIDL).
package type_resolvers

import (
	"errors"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	_ "sdv.googlesource.com/mcg/mcg/supported_protobuf_deps"
)

// Assert that `EnrichedTypeResolver` implements both `MessageTypeResolver` and `ExtensionTypeResolver`.
var (
	_ protoregistry.MessageTypeResolver   = (*EnrichedTypeResolver)(nil)
	_ protoregistry.ExtensionTypeResolver = (*EnrichedTypeResolver)(nil)
)

type EnrichedTypeResolver struct {
	Local *protoregistry.Types
}

// NewEnrichedTypeResolverFromBytes is a constructor collecting the types from bytes array
// containing `descriptorpb.FileDescriptorSet` data (such as vehicle signals) as bytes.
func NewEnrichedTypeResolverFromBytes(bts []byte) (*EnrichedTypeResolver, error) {
	fdSet, err := UnmarshalFileDescriptorSet(bts)
	if err != nil {
		return nil, err
	}
	return NewEnrichedTypeResolverFromFileDescriptorSet(fdSet)
}

// NewEnrichedTypeResolverFromFileDescriptorProtos is a constructor collecting the types from
// `descriptorpb.FileDescriptorProtos` into an `EnrichedTypeResolver`. It always returns a resolver
// even if there are no local types. Then it just knows about the global types.
func NewEnrichedTypeResolverFromFileDescriptorProtos(fdProtos []*descriptorpb.FileDescriptorProto) (*EnrichedTypeResolver, error) {
	fdSet := new(descriptorpb.FileDescriptorSet)
	for _, fdProto := range fdProtos {
		fdSet.File = append(fdSet.File, fdProto)
	}
	return NewEnrichedTypeResolverFromFileDescriptorSet(fdSet)
}

func NewEnrichedTypeResolverFromFileDescriptorSet(fdSet *descriptorpb.FileDescriptorSet) (*EnrichedTypeResolver, error) {
	localTypes := new(protoregistry.Types)
	resolver := &EnrichedTypeResolver{Local: localTypes}

	err := resolver.ExtendLocalTypes(fdSet)
	if err != nil {
		return nil, err
	}
	return &EnrichedTypeResolver{Local: localTypes}, nil
}

func (fr *EnrichedTypeResolver) FindMessageByName(message protoreflect.FullName) (protoreflect.MessageType, error) {
	// TODO(b/350777804): Adjust accordingly depending on whether the dot should or shouldn't be there
	msgWithoutDot := protoreflect.FullName(strings.TrimPrefix(string(message), "."))

	mt, err := fr.Local.FindMessageByName(msgWithoutDot)
	if err == nil {
		return mt, err
	}
	return protoregistry.GlobalTypes.FindMessageByName(msgWithoutDot)
}

// FindMessageByURL is needed to implement the interface but it doesn't currently support Rust-style
// dot-prefixed messages.
func (fr *EnrichedTypeResolver) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	mt, err := fr.Local.FindMessageByURL(url)
	if err == nil {
		return mt, err
	}
	return protoregistry.GlobalTypes.FindMessageByURL(url)
}

func (fr *EnrichedTypeResolver) FindEnumByName(name protoreflect.FullName) (protoreflect.EnumType, error) {
	name = protoreflect.FullName(strings.TrimPrefix(string(name), "."))

	et, err := fr.Local.FindEnumByName(name)
	if err == nil {
		return et, err
	}
	return protoregistry.GlobalTypes.FindEnumByName(name)
}

// FindExtensionByName looks up a extension field by the field's full name.
// Note that this is the full name of the field as determined by
// where the extension is declared and is unrelated to the full name of the
// message being extended.
//
// This returns (nil, NotFound) if not found.
func (fr *EnrichedTypeResolver) FindExtensionByName(field protoreflect.FullName) (protoreflect.ExtensionType, error) {
	return protoregistry.GlobalTypes.FindExtensionByName(field)
}

// FindExtensionByNumber looks up a extension field by the field number
// within some parent message, identified by full name.
//
// This returns (nil, NotFound) if not found.
func (fr *EnrichedTypeResolver) FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	return protoregistry.GlobalTypes.FindExtensionByNumber(message, field)
}

func collectTypesFromFileDescriptorSet(fdSet *descriptorpb.FileDescriptorSet) ([]protoreflect.MessageType, []protoreflect.EnumType, error) {
	var msgTypes []protoreflect.MessageType
	var enumTypes []protoreflect.EnumType

	sorted, err := SortFileDescriptorSet(fdSet)
	if err != nil {
		return nil, nil, err
	}

	var localFiles protoregistry.Files = *new(protoregistry.Files)

	// Register 'supported_protobuf_deps' from the global registry
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		localFiles.RegisterFile(fd)
		return true
	})

	for _, fdProto := range sorted {
		unnamedFdProto := false

		// If the file descriptor proto does not define a file name, assign a
		// temporary name to allow conversion to `protoreflect.FileDescriptor`.
		if fdProto.Name == nil {
			tmp := "tmp.proto"
			fdProto.Name = &tmp
			unnamedFdProto = true
		}

		file, err := protodesc.NewFile(fdProto, &localFiles)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to create FileDescriptor from fdProto for %v: %w", fdProto.GetName(), err)
		}

		fdMsgTypes, fdNestedEnumTypes := collectMessagesAndEnumsRecursively(file.Messages())
		msgTypes = append(msgTypes, fdMsgTypes...)
		// Enums can be nested inside messages...
		enumTypes = append(enumTypes, fdNestedEnumTypes...)
		// ...or defined at the top-level of the file.
		enumTypes = append(enumTypes, collectEnums(file.Enums())...)

		if unnamedFdProto {
			// Remove temporary name and skip registering file in the
			// localFiles since it cannot have dependents without a file name.
			fdProto.Name = nil
		} else {
			/* Add file to localFiles, so it is available in the resolver of protodesc.NewFile
			when generating FileDescriptors of dependent files. */
			_, err = localFiles.FindFileByPath(file.Path())
			if errors.Is(err, protoregistry.NotFound) {
				localFiles.RegisterFile(file)
			} else if err != nil {
				return nil, nil, fmt.Errorf("Failed to look up file by path %v: %w", file.Path(), err)
			}
		}
	}

	return msgTypes, enumTypes, nil
}

// UnmarshalFileDescriptorSet is a convenience function for proto.Unmarshal.
func UnmarshalFileDescriptorSet(bytes []byte) (*descriptorpb.FileDescriptorSet, error) {
	fdSet := new(descriptorpb.FileDescriptorSet)
	if err := proto.Unmarshal(bytes, fdSet); err != nil {
		return nil, err
	}
	return fdSet, nil
}

func (fr *EnrichedTypeResolver) ExtendLocalTypes(fdSet *descriptorpb.FileDescriptorSet) error {
	msgTypes, enumTypes, err := collectTypesFromFileDescriptorSet(fdSet)
	if err != nil {
		return err
	}

	for _, msgType := range msgTypes {
		if err := fr.Local.RegisterMessage(msgType); err != nil {
			// We have a naming conflict - a message or enum with the same name
			// is already registered.

			existingMsgType, err := fr.Local.FindMessageByName(msgType.Descriptor().FullName())
			if err != nil {
				// This may fail if the an enum, not a message, is registered
				// for the name.
				return fmt.Errorf("type %q is already registered, but not as a message type: %w", msgType.Descriptor().FullName(), err)
			}

			// Compare descriptors to see if the definitions are identical.
			existingDesc := protodesc.ToDescriptorProto(existingMsgType.Descriptor())
			newDesc := protodesc.ToDescriptorProto(msgType.Descriptor())
			if !proto.Equal(existingDesc, newDesc) {
				return fmt.Errorf("message type %q with a different definition is already registered", msgType.Descriptor().FullName())
			}
		}
	}

	for _, enumType := range enumTypes {
		if err := fr.Local.RegisterEnum(enumType); err != nil {
			// We have a naming conflict - a message or enum with the same name
			// is already registered.

			existingEnumType, err := fr.Local.FindEnumByName(enumType.Descriptor().FullName())
			if err != nil {
				// This may fail if the a message, not an enum, is registered
				// for the name.
				return fmt.Errorf("type %q is already registered, but not as an enum type: %w", enumType.Descriptor().FullName(), err)
			}

			// Compare descriptors to see if the definitions are identical.
			existingDesc := protodesc.ToEnumDescriptorProto(existingEnumType.Descriptor())
			newDesc := protodesc.ToEnumDescriptorProto(enumType.Descriptor())
			if !proto.Equal(existingDesc, newDesc) {
				return fmt.Errorf("enum type %q with a different definition is already registered", enumType.Descriptor().FullName())
			}
		}
	}

	return nil
}

func collectMessagesAndEnumsRecursively(messageDescriptors protoreflect.MessageDescriptors) ([]protoreflect.MessageType, []protoreflect.EnumType) {
	var messages []protoreflect.MessageType
	var enums []protoreflect.EnumType
	for i := 0; i < messageDescriptors.Len(); i++ {
		messageDescriptor := messageDescriptors.Get(i)
		messages = append(messages, dynamicpb.NewMessageType(messageDescriptor))
		enums = append(enums, collectEnums(messageDescriptor.Enums())...)

		nestedMessages, nestedEnums := collectMessagesAndEnumsRecursively(messageDescriptor.Messages())
		messages = append(messages, nestedMessages...)
		enums = append(enums, nestedEnums...)
	}
	return messages, enums
}

func collectEnums(enumDescriptors protoreflect.EnumDescriptors) []protoreflect.EnumType {
	var enums []protoreflect.EnumType
	for i := 0; i < enumDescriptors.Len(); i++ {
		enums = append(enums, dynamicpb.NewEnumType(enumDescriptors.Get(i)))
	}
	return enums
}
