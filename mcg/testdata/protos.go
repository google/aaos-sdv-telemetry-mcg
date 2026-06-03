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

package testdata

import (
	_ "embed"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

var (
	//go:embed input_descriptors_sample_file_descriptor_set.bin
	inputDescriptorsSampleFileDescriptorSetBytes []byte
	//go:embed proto2_example_file_descriptor_set.bin
	proto2ExampleFileDescriptorSetBytes []byte
	//go:embed vehicle_signals_sample_file_descriptor_set.bin
	vehicleSignalsSampleFileDescriptorSetBytes []byte
	//go:embed speed_file_descriptor_set.bin
	speedFileDescriptorSetBytes []byte
	//go:embed maxavgcur_file_descriptor_set.bin
	maxavgcurFileDescriptorSetBytes []byte
	//go:embed timestamp_file_descriptor_set.bin
	timestampFileDescriptorSetBytes []byte
)

var (
	InputDescriptorsSampleFileDescriptorSet *descriptorpb.FileDescriptorSet = mustUnmarshalFileDescriptorSet(inputDescriptorsSampleFileDescriptorSetBytes)
	Proto2ExampleFileDescriptorSet          *descriptorpb.FileDescriptorSet = mustUnmarshalFileDescriptorSet(proto2ExampleFileDescriptorSetBytes)
	VehicleSignalsSampleFileDescriptorSet   *descriptorpb.FileDescriptorSet = mustUnmarshalFileDescriptorSet(vehicleSignalsSampleFileDescriptorSetBytes)
	SpeedFileDescriptorSet                  *descriptorpb.FileDescriptorSet = mustUnmarshalFileDescriptorSet(speedFileDescriptorSetBytes)
	MaxavgcurFileDescriptorSet              *descriptorpb.FileDescriptorSet = mustUnmarshalFileDescriptorSet(maxavgcurFileDescriptorSetBytes)
	TimestampFileDescriptorSet              *descriptorpb.FileDescriptorSet = mustUnmarshalFileDescriptorSet(timestampFileDescriptorSetBytes)
)

func mustUnmarshalFileDescriptorSet(bytes []byte) *descriptorpb.FileDescriptorSet {
	fdSet := new(descriptorpb.FileDescriptorSet)
	if err := proto.Unmarshal(bytes, fdSet); err != nil {
		panic(fmt.Sprintf("proto.Unmarshal(%v, _) failed: %v", bytes, err))
	}
	return fdSet
}
