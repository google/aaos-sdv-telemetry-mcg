// Copyright 2025 Google LLC
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

package testhelper

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/descriptorpb"

	"sdv.googlesource.com/mcg/mcg/testdata"
	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

func LoadJSON(fName string, vsReq any) error {
	reqBytes, err := os.ReadFile(fName)
	if err != nil {
		return err
	}
	err = json.Unmarshal(reqBytes, &vsReq)
	if err != nil {
		return err
	}
	return nil
}

func MergeFileDescriptorSets(fdSets ...*descriptorpb.FileDescriptorSet) *descriptorpb.FileDescriptorSet {
	result := new(descriptorpb.FileDescriptorSet)
	for _, fdSet := range fdSets {
		result.File = append(result.File, fdSet.File...)
	}
	return result
}

func MustGetFileDescriptorFromSet(fdSet *descriptorpb.FileDescriptorSet, name string) *descriptorpb.FileDescriptorProto {
	idx := slices.IndexFunc(fdSet.File, func(fd *descriptorpb.FileDescriptorProto) bool {
		return *fd.Name == name
	})
	if idx < 0 {
		fileNames := make([]string, len(fdSet.File))
		for i, fd := range fdSet.File {
			fileNames[i] = *fd.Name
		}
		panic(fmt.Sprintf("Failed to find file descriptor named %q in file descriptor containing files named: %v", name, fileNames))
	}

	return fdSet.File[idx]
}

func GetSpeedFdWithDependencies() *descriptorpb.FileDescriptorSet {
	fdSet := new(descriptorpb.FileDescriptorSet)
	fdSet.File = []*descriptorpb.FileDescriptorProto{
		MustGetFileDescriptorFromSet(testdata.SpeedFileDescriptorSet, "mcg/testdata/speed/vehicle_speed.proto"),
		MustGetFileDescriptorFromSet(testdata.VehicleSignalsSampleFileDescriptorSet, "mcg/testdata/vehicle_signals_sample/sdv_protos/syntax/vsidl.proto"),
		MustGetFileDescriptorFromSet(testdata.VehicleSignalsSampleFileDescriptorSet, "mcg/testdata/vehicle_signals_sample/sdv_protos/syntax/someip.proto"),
		MustGetFileDescriptorFromSet(testdata.VehicleSignalsSampleFileDescriptorSet, "mcg/testdata/vehicle_signals_sample/sdv_protos/syntax/someip_type.proto"),
		MustGetFileDescriptorFromSet(testdata.VehicleSignalsSampleFileDescriptorSet, "mcg/testdata/vehicle_signals_sample/sdv_protos/syntax/diagnostics.proto"),
	}
	return fdSet
}

func DefaultMetricsConfigCmpOptions() []cmp.Option {
	return []cmp.Option{
		protocmp.Transform(),
		protocmp.SortRepeated(func(a, b *pb.Trigger) bool {
			return a.GetName() < b.GetName()
		}),
		protocmp.SortRepeated(func(a, b *pb.Source) bool {
			return a.GetName() < b.GetName()
		}),
		protocmp.SortRepeated(func(a, b *pb.MetricsReportConfig) bool {
			return a.GetName() < b.GetName()
		}),
		protocmp.SortRepeated(func(a, b *descriptorpb.FileDescriptorProto) bool {
			return a.GetName() < b.GetName()
		}),
		protocmp.SortRepeated(func(a, b *descriptorpb.DescriptorProto) bool {
			return a.GetName() < b.GetName()
		}),
		protocmp.IgnoreFields(new(pb.MetricsConfig), protoreflect.Name("uuid")),
	}
}

func AssertMetricsConfigEqual(t *testing.T, want, got *pb.MetricsConfig, opts ...cmp.Option) {
	t.Helper()
	opts = append(DefaultMetricsConfigCmpOptions(), opts...)
	if diff := cmp.Diff(want, got, opts...); diff != "" {
		t.Errorf("MetricsConfig mismatch (-want +got):\n%s", diff)
	}
}
