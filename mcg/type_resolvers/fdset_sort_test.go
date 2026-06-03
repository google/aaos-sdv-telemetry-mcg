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
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/descriptorpb"

	"sdv.googlesource.com/mcg/mcg/testdata"
	"sdv.googlesource.com/mcg/mcg/testhelper"
	"sdv.googlesource.com/mcg/mcg/type_resolvers"
)

func TestSortFileDescriptorSet(t *testing.T) {
	sortFdSlices := cmpopts.SortSlices(func(a, b *descriptorpb.FileDescriptorProto) bool { return *a.Name < *b.Name })

	fdSet := testdata.VehicleSignalsSampleFileDescriptorSet

	gotFds, err := type_resolvers.SortFileDescriptorSet(fdSet)
	if err != nil {
		t.Fatalf("type_resolvers.SortFileDescriptorSet(%v) failed: %v", fdSet, err)
	}

	// Step 1: Check that all FDs were returned (regardless of their order).
	wantFds := fdSet.File
	if diff := cmp.Diff(wantFds, gotFds, protocmp.Transform(), sortFdSlices); diff != "" {
		t.Errorf("Unexpected difference (-want +got):\n%s", diff)
	}

	// Step 2: Check that all deps of every FD appear before the FD itself in
	// the sorted list.
	for selfIdx, fd := range gotFds {
		for _, depName := range fd.GetDependency() {
			depIdx := slices.IndexFunc(gotFds, func(fd *descriptorpb.FileDescriptorProto) bool {
				return *fd.Name == depName
			})
			if depIdx < 0 {
				// This is an external dependency, such as
				// `google/protobuf/descriptor.proto`, which we can safely
				// ignore.
			}
			if depIdx >= selfIdx {
				t.Errorf("Dependency %q of %q appears after it in sorted list.", depName, fd.GetName())
			}
		}
	}
}

func TestSortFileDescriptorSetIgnoreDuplicates(t *testing.T) {
	speed_list_proto := testhelper.MustGetFileDescriptorFromSet(testdata.VehicleSignalsSampleFileDescriptorSet, "mcg/testdata/vehicle_signals_sample/subpkg/sample_speed_list.proto")
	speed_proto := testhelper.MustGetFileDescriptorFromSet(testdata.VehicleSignalsSampleFileDescriptorSet, "mcg/testdata/vehicle_signals_sample/subpkg/sample_speed.proto")

	fdSetUnsorted := new(descriptorpb.FileDescriptorSet)
	fdSetUnsorted.File = []*descriptorpb.FileDescriptorProto{
		speed_list_proto,
		speed_list_proto,
		speed_proto,
	}

	expectedFdList := []*descriptorpb.FileDescriptorProto{
		speed_proto,
		speed_list_proto,
	}

	sortedFdList, err := type_resolvers.SortFileDescriptorSet(fdSetUnsorted)
	if err != nil {
		t.Error(err)
	}

	if len(sortedFdList) != len(expectedFdList) {
		t.Errorf(fmt.Sprintf("Sorted []Filedescriptor has length of: %v, wanted %v", len(sortedFdList), len(expectedFdList)))
	}
	for i, sortedEntry := range sortedFdList {
		if expectedFdList[i] == nil || expectedFdList[i].GetName() != sortedEntry.GetName() {
			t.Errorf("Sorted []Filedescriptor %v does not match expected list %v", sortedFdList, expectedFdList)
		}
	}
}
