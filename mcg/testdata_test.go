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

package mcg

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"sdv.googlesource.com/mcg/mcg/requests"
	"sdv.googlesource.com/mcg/mcg/testdata"
	"sdv.googlesource.com/mcg/mcg/testhelper"
)

// The point of this test is to make sure that the vehicle signals and/or
// descriptor protos specified in certain test files match the proto definitions
// and don't get out of sync with each other.
func TestDataJsonFileDescriptorsMatchWithExpectedFileDescriptorSet(t *testing.T) {
	// A `cmp.Transformer` to make the diff look a bit nicer and matches how the
	// bytes are represented in the request JSON.`
	base64Transformer := cmp.Transformer("AsBase64", func(in []byte) string {
		return base64.StdEncoding.EncodeToString(in)
	})

	// Maps the provided file descriptor set into a slice of strings, where each
	// string is the quoted path of a file descriptor contained in the set.
	getProtoPaths := func(fdset *descriptorpb.FileDescriptorSet) []string {
		var protoPaths []string
		for _, fd := range fdset.File {
			protoPaths = append(protoPaths, fmt.Sprintf("%q", fd.GetName()))
		}
		return protoPaths
	}

	testCases := []struct {
		file              string
		vehicle_signals   *descriptorpb.FileDescriptorSet
		descriptor_protos []*descriptorpb.FileDescriptorSet
	}{
		{
			file:              "testdata/eipf_b.json",
			vehicle_signals:   testhelper.MergeFileDescriptorSets(testdata.MaxavgcurFileDescriptorSet, testdata.Proto2ExampleFileDescriptorSet),
			descriptor_protos: []*descriptorpb.FileDescriptorSet{testhelper.MergeFileDescriptorSets(testdata.MaxavgcurFileDescriptorSet, testdata.Proto2ExampleFileDescriptorSet)},
		},
		{
			file:            "testdata/source_configuration.json",
			vehicle_signals: testdata.MaxavgcurFileDescriptorSet,
			descriptor_protos: []*descriptorpb.FileDescriptorSet{
				testdata.Proto2ExampleFileDescriptorSet,
			},
		},
		{
			file:            "testdata/comprehensive_descriptor_optimization.json",
			vehicle_signals: testdata.VehicleSignalsSampleFileDescriptorSet,
			descriptor_protos: []*descriptorpb.FileDescriptorSet{
				testdata.InputDescriptorsSampleFileDescriptorSet,
				testdata.TimestampFileDescriptorSet,
				testdata.VehicleSignalsSampleFileDescriptorSet,
			},
		},
	}
	for _, tc := range testCases {
		var metricsConfigRequest requests.MetricsConfigRequest
		if err := testhelper.LoadJSON(tc.file, &metricsConfigRequest); err != nil {
			t.Fatalf("Failed to load JSON from %s: %v", tc.file, err)
		}

		t.Run(fmt.Sprintf("%s_vehicle_signals", tc.file), func(t *testing.T) {
			want, err := proto.Marshal(tc.vehicle_signals)
			if err != nil {
				t.Fatalf("proto.Marshal(%v) failed: %v", tc.vehicle_signals, err)
			}
			got := metricsConfigRequest.VehicleSignals

			if diff := cmp.Diff(want, got, base64Transformer); diff != "" {
				protoPaths := getProtoPaths(tc.vehicle_signals)
				t.Errorf(`
The "vehicle_signals" field in %q has gone out of sync with %s (-want +got):

%s

Run the following command to retrieve the correct field content:

protoc --include_imports --descriptor_set_out=/dev/stdout %s | base64 --wrap 0
`, tc.file, strings.Join(protoPaths, ", "), diff, strings.Join(protoPaths, " "))
			}
		})
		t.Run(fmt.Sprintf("%s_descriptor_protos", tc.file), func(t *testing.T) {
			var want [][]byte
			for _, fdset := range tc.descriptor_protos {
				bytes, err := proto.Marshal(fdset)
				if err != nil {
					t.Fatalf("proto.Marshal(%v) failed: %v", fdset, err)
				}
				want = append(want, bytes)
			}
			got := metricsConfigRequest.DescriptorProtos

			if diff := cmp.Diff(want, got, base64Transformer); diff != "" {
				var protoPaths []string
				var commands []string
				for _, fdset := range tc.descriptor_protos {
					paths := getProtoPaths(fdset)
					protoPaths = append(protoPaths, strings.Join(paths, ":"))

					command := fmt.Sprintf("protoc --include_imports --descriptor_set_out=/dev/stdout %s | base64 --wrap 0", strings.Join(paths, " "))
					commands = append(commands, command)
				}
				t.Errorf(`
The "descriptor_protos" field in %q has gone out of sync with %s (-want +got):

%s

Run the following commands to retrieve the correct field contents:

%s
`, tc.file, strings.Join(protoPaths, ", "), diff, strings.Join(commands, "\n"))
			}
		})
	}
}
