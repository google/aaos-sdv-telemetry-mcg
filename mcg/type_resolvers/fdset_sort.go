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

package type_resolvers

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"

	"sdv.googlesource.com/mcg/mcg/graph"
)

func SortFileDescriptorSet(fdSet *descriptorpb.FileDescriptorSet) ([]*descriptorpb.FileDescriptorProto, error) {
	fileProtoLookup := make(map[string]*descriptorpb.FileDescriptorProto)
	g := graph.NewGraph[string]()

	// Build dependency graph
	for _, fd := range fdSet.File {
		fileName := fd.GetName()
		if IgnoreDependency(fileName) {
			continue
		}

		// Ignore duplicate file descriptors
		if _, ok := fileProtoLookup[fileName]; ok {
			continue
		}

		g.AddNode(fileName)
		fileProtoLookup[fileName] = fd
		for _, dep := range fd.GetDependency() {
			if IgnoreDependency(dep) {
				continue
			}
			g.AddEdge(fileName, dep)
		}
	}

	orderedFileNames, err := g.ReverseTopologicalOrdering()
	if err != nil {
		return nil, fmt.Errorf("Missing or circular dependency in File Descriptor Set: %w", err)
	}

	orderedFileDescriptors := make([]*descriptorpb.FileDescriptorProto, len(orderedFileNames))
	for i, fileName := range orderedFileNames {
		orderedFileDescriptors[i] = fileProtoLookup[fileName]
	}
	return orderedFileDescriptors, nil
}

func IgnoreDependency(dep string) bool {
	if strings.HasPrefix(dep, "google/protobuf") {
		return true
	}
	return false
}
