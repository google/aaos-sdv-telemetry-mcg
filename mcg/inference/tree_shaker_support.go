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

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"sdv.googlesource.com/mcg/mcg/graph"
)

// If `msg` is a nested message, returns the top-level message. Otherwise
// returns `msg` itself.
func getRootMessage(msg protoreflect.MessageDescriptor) protoreflect.MessageDescriptor {
	if parent, ok := msg.Parent().(protoreflect.MessageDescriptor); ok {
		return getRootMessage(parent)
	}
	return msg
}

// Populate `deps` with all file descriptor paths of all (recursive) dependencies of `desc`.
func getMessageDeps(desc protoreflect.MessageDescriptor, deps map[string]struct{}) {
	// Always add the message's own file as a dependency. This happens anyways
	// as soon as a field is a message/enum defined in the same file. Thus, we
	// add it here for consistency.
	deps[desc.ParentFile().Path()] = struct{}{}

	fields := desc.Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if msg := field.Message(); msg != nil {
			deps[msg.ParentFile().Path()] = struct{}{}
		} else if enum := field.Enum(); enum != nil {
			deps[enum.ParentFile().Path()] = struct{}{}
		}
	}

	messages := desc.Messages()
	for i := 0; i < messages.Len(); i++ {
		message := messages.Get(i)
		getMessageDeps(message, deps)
	}
}

// Returns a slice of the file descriptors from `fds` sorted topologically based on their dependencies.
func sortedFileDescriptors(fds map[string]*descriptorpb.FileDescriptorProto) ([]*descriptorpb.FileDescriptorProto, error) {
	g := graph.NewGraph[string]()
	for path, fd := range fds {
		g.AddNode(path)
		for _, depPath := range fd.GetDependency() {
			if _, ok := fds[depPath]; ok {
				g.AddEdge(depPath, path)
			}
		}
	}

	paths, err := g.StableTopologicalOrdering()
	if err != nil {
		return nil, fmt.Errorf("failed to topologically sort file descriptors: %w", err)
	}

	var sorted []*descriptorpb.FileDescriptorProto
	for _, path := range paths {
		sorted = append(sorted, fds[path])
	}
	return sorted, nil
}
