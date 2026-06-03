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
	"maps"
	"slices"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// These should match with the Rust side:
// Path in Android: system/software_defined_vehicle/telemetry/src/utils/descriptor_utils.rs
var wellKnownTypesPaths = map[string]struct{}{
	"google/protobuf/any.proto":            {},
	"google/protobuf/api.proto":            {},
	"google/protobuf/duration.proto":       {},
	"google/protobuf/empty.proto":          {},
	"google/protobuf/field_mask.proto":     {},
	"google/protobuf/source_context.proto": {},
	"google/protobuf/struct.proto":         {},
	"google/protobuf/timestamp.proto":      {},
	"google/protobuf/type.proto":           {},
	"google/protobuf/wrappers.proto":       {},
	"google/protobuf/descriptor.proto":     {},
}

type file struct {
	// fdProto is the generated synthetic file descriptor proto.
	fdProto *descriptorpb.FileDescriptorProto
	// messages are the messages included in fdProto.
	messages map[protoreflect.FullName]protoreflect.MessageDescriptor
	// enums are the enums included in fdProto.
	enums map[protoreflect.FullName]protoreflect.EnumDescriptor
}

// files maps from file paths to `file`s.
type files map[string]*file

func (m files) getOrInit(fd protoreflect.FileDescriptor) *file {
	path := fd.Path()
	f, ok := m[path]
	if !ok {
		fdProto := &descriptorpb.FileDescriptorProto{
			Package: proto.String(string(fd.Package())),
			Name:    &path,
			Syntax:  proto.String(fd.Syntax().String()),
		}
		f = &file{
			fdProto:  fdProto,
			messages: make(map[protoreflect.FullName]protoreflect.MessageDescriptor),
			enums:    make(map[protoreflect.FullName]protoreflect.EnumDescriptor),
		}
		m[path] = f

		for i := 0; i < fd.Imports().Len(); i++ {
			imp := fd.Imports().Get(i)
			fdProto.Dependency = append(fdProto.Dependency, imp.Path())
			if imp.IsPublic {
				fdProto.PublicDependency = append(fdProto.PublicDependency, int32(i))
			}
			m.getOrInit(imp)
		}
	}
	return f
}

type treeShaker struct{ files files }

func TreeShake(
	msgs map[protoreflect.FullName]protoreflect.MessageDescriptor,
	enums map[protoreflect.FullName]protoreflect.EnumDescriptor,
) ([]*descriptorpb.FileDescriptorProto, error) {
	ts := treeShaker{files: make(files)}
	return ts.treeShake(msgs, enums)
}

func (ts *treeShaker) treeShake(
	msgs map[protoreflect.FullName]protoreflect.MessageDescriptor,
	enums map[protoreflect.FullName]protoreflect.EnumDescriptor,
) ([]*descriptorpb.FileDescriptorProto, error) {
	// 1. Walk and harvest each referenced message in deterministic order.
	msgNames := slices.Collect(maps.Keys(msgs))
	slices.Sort(msgNames)
	for _, msgName := range msgNames {
		msg := msgs[msgName]
		if err := ts.harvestMessage(msg); err != nil {
			return nil, fmt.Errorf("failed to harvest message %v: %w", msg, err)
		}
	}

	// 2. Walk and harvest each referenced enum in deterministic order.
	enumNames := slices.Collect(maps.Keys(enums))
	slices.Sort(enumNames)
	for _, enumName := range enumNames {
		enum := enums[enumName]
		if err := ts.harvestEnum(enum); err != nil {
			return nil, fmt.Errorf("failed to harvest enum %v: %w", enum, err)
		}
	}

	// 3. Prune unused imports in file descriptors.
	ts.pruneUnusedImports()

	// 4. Prune unused file descriptors (i.e., file descriptors that are no
	// longer imported from anywhere).
	ts.pruneUnusedAndWellKnownFileDescriptors()

	// 5. Topologically sort the file descriptors based on their dependencies.
	fdProtosMap := make(map[string]*descriptorpb.FileDescriptorProto)
	for path, file := range ts.files {
		fdProtosMap[path] = file.fdProto
	}
	fdProtos, err := sortedFileDescriptors(fdProtosMap)
	if err != nil {
		return nil, fmt.Errorf("failed to sort file descriptors: %w", err)
	}

	return fdProtos, nil
}

func (ts *treeShaker) harvestMessage(msg protoreflect.MessageDescriptor) error {
	topAncestor := getRootMessage(msg)
	fd := topAncestor.ParentFile()
	if fd == nil {
		// This _should_ never happen.
		return fmt.Errorf("failed to get file descriptor of %v. This is a bug; please report it", topAncestor)
	}

	file := ts.files.getOrInit(fd)
	file.messages[msg.FullName()] = msg
	file.messages[topAncestor.FullName()] = topAncestor

	// FIXME: Technically, we should prune everything inside `topAncestor` and
	// all intermediate messages down towards `msg`. (Once we do, we need to
	// be careful not to break the call to `harvestMessage` from `harvestEnum`).
	//
	// For now, when we encounter a nested message, we simply put its top-level
	// message and all of its nested messages into the `fdProto`.
	if !slices.ContainsFunc(file.fdProto.MessageType, func(m *descriptorpb.DescriptorProto) bool {
		return m.GetName() == string(topAncestor.Name())
	}) {
		file.fdProto.MessageType = append(file.fdProto.MessageType, protodesc.ToDescriptorProto(topAncestor))
		ts.harvestMessageDeps(topAncestor)
	}

	return nil
}

func (ts *treeShaker) harvestEnum(enum protoreflect.EnumDescriptor) error {
	// If the enum is nested inside of a message, we simply harvest the message,
	// which includes enums inside of it.
	if msg, ok := enum.Parent().(protoreflect.MessageDescriptor); ok {
		return ts.harvestMessage(msg)
	}

	fd := enum.ParentFile()
	if fd == nil {
		// This _should_ never happen.
		return fmt.Errorf("failed to get file descriptor of %v. This is a bug; please report it", enum)
	}

	file := ts.files.getOrInit(fd)
	file.enums[enum.FullName()] = enum

	if !slices.ContainsFunc(file.fdProto.EnumType, func(e *descriptorpb.EnumDescriptorProto) bool {
		return e.GetName() == string(enum.Name())
	}) {
		file.fdProto.EnumType = append(file.fdProto.EnumType, protodesc.ToEnumDescriptorProto(enum))
		// Enums do not have dependencies we need to harvest.
	}

	return nil
}

func (ts *treeShaker) harvestMessageDeps(msg protoreflect.MessageDescriptor) {
	fields := msg.Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if msg := field.Message(); msg != nil {
			ts.harvestMessage(msg)
		} else if enum := field.Enum(); enum != nil {
			ts.harvestEnum(enum)
		}
	}
	msgs := msg.Messages()
	for i := 0; i < msgs.Len(); i++ {
		msg := msgs.Get(i)
		ts.harvestMessageDeps(msg)
	}
}

// grantsVisibilityTo determines if importing the file descriptor proto at
// `path` grants visibility into the file descriptor proto at `targetPath`. In
// Protobuf, importing a file provides visibility to types defined within that
// file, as well as types from files it explicitly re-exports via `import
// public`.
func (ts *treeShaker) grantsVisibilityTo(path, targetPath string) bool {
	// A file trivially grants visibility to itself.
	if path == targetPath {
		return true
	}
	file, ok := ts.files[path]
	if !ok {
		return false
	}
	// Recursively check if any of its `import public` statements reach the target.
	for _, idx := range file.fdProto.PublicDependency {
		if idx >= 0 && int(idx) < len(file.fdProto.Dependency) {
			if ts.grantsVisibilityTo(file.fdProto.Dependency[idx], targetPath) {
				return true
			}
		}
	}
	return false
}

func (ts *treeShaker) pruneUnusedImports() {
	for _, file := range ts.files {
		// requiredDeps holds the paths of files descriptor protos that
		// contain types we *must* have visibility into for this file to
		// compile.
		requiredDeps := make(map[string]struct{})

		// 1. Gather files containing types used by the messages defined in this
		// file.
		for _, msgDesc := range file.messages {
			getMessageDeps(msgDesc, requiredDeps)
		}

		// Enums cannot depend on other files, hence we do not need to
		// iterate through `file.enums` here.

		// 2. We must never prune our own `import public` dependencies.
		// Downstream files importing us might rely on them. By adding them to
		// requiredTargets, we ensure we maintain visibility to them.
		for _, idx := range file.fdProto.PublicDependency {
			if idx >= 0 && int(idx) < len(file.fdProto.Dependency) {
				requiredDeps[file.fdProto.Dependency[idx]] = struct{}{}
			}
		}

		var keptDeps []string
		oldToNew := make(map[int32]int32) // Tracks how indices shift during pruning

		// 3. Evaluate each existing imports. We only keep it if it serves a
		// purpose (i.e. it provides visibility to at least one required
		// target).
		for i, dep := range file.fdProto.Dependency {
			keep := false
			if _, ok := requiredDeps[dep]; ok {
				keep = true
			} else {
				for requiredTarget := range requiredDeps {
					if ts.grantsVisibilityTo(dep, requiredTarget) {
						keep = true
						break
					}
				}
			}
			if keep {
				keptDeps = append(keptDeps, dep)
				oldToNew[int32(i)] = int32(len(keptDeps) - 1)
			}
		}

		file.fdProto.Dependency = keptDeps

		// 4. Update the `public_dependency` array to point to the shifted
		// indices.
		var newPubDepIdxs []int32
		for _, oldIdx := range file.fdProto.PublicDependency {
			if newIdx, ok := oldToNew[oldIdx]; ok {
				newPubDepIdxs = append(newPubDepIdxs, newIdx)
			}
		}
		file.fdProto.PublicDependency = newPubDepIdxs
	}
}

func (ts *treeShaker) pruneUnusedAndWellKnownFileDescriptors() {
	keepPaths := make(map[string]struct{})
	var keep func(string)
	keep = func(path string) {
		if _, ok := keepPaths[path]; ok {
			return
		}
		keepPaths[path] = struct{}{}
		if file, ok := ts.files[path]; ok {
			for _, dep := range file.fdProto.GetDependency() {
				keep(dep)
			}
		}
	}

	for path, file := range ts.files {
		if len(file.messages) > 0 || len(file.enums) > 0 {
			keep(path)
		}
	}

	// Prune the file descriptors that are not referenced or well-known types.
	maps.DeleteFunc(
		ts.files,
		func(path string, file *file) bool {
			if _, ok := wellKnownTypesPaths[file.fdProto.GetName()]; ok {
				return true
			}

			_, ok := keepPaths[path]
			return !ok
		},
	)
}

