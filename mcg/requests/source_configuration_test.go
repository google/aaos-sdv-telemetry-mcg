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

package requests_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/anypb"

	"sdv.googlesource.com/mcg/mcg/requests"
	"sdv.googlesource.com/mcg/mcg/testdata"
	"sdv.googlesource.com/mcg/mcg/type_resolvers"
)

func makeResolver(t *testing.T, fileDescriptors []*descriptorpb.FileDescriptorProto) type_resolvers.EnrichedTypeResolver {
	resolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorProtos(fileDescriptors)
	if err != nil {
		t.Fatalf("NewEnrichedTypeResolverFromFileDescriptorProtos(_) failed: %v", err)
	}
	return *resolver
}

func TestErrorsWhenNotExactlyOneMethodToSpecifyValueIsUsed(t *testing.T) {
	resolver := makeResolver(t, []*descriptorpb.FileDescriptorProto{})

	STRING := "a string"
	var JSON interface{}
	JSON = STRING

	for _, req := range []*requests.DataSourceConfigurationRequest{
		{
			TypeUrl: "type.googleapis.com/com.example.Proto",
		},
		{
			TypeUrl:        "type.googleapis.com/com.example.Proto",
			Value:          &[]byte{1, 2, 3},
			ValueTextproto: &STRING,
		},
		{
			TypeUrl:   "type.googleapis.com/com.example.Proto",
			Value:     &[]byte{1, 2, 3},
			ValueJson: &JSON,
		},
		{
			TypeUrl:        "type.googleapis.com/com.example.Proto",
			ValueTextproto: &STRING,
			ValueJson:      &JSON,
		},
		{
			TypeUrl:        "type.googleapis.com/com.example.Proto",
			Value:          &[]byte{1, 2, 3},
			ValueTextproto: &STRING,
			ValueJson:      &JSON,
		},
	} {
		if _, err := req.ToProto(resolver); err == nil {
			t.Error("req.ToProto(resolver) = _, nil, want _, err")
		}
	}
}

func TestConvertsBinaryValue(t *testing.T) {
	resolver := makeResolver(t, []*descriptorpb.FileDescriptorProto{})

	for _, tc := range []*struct {
		req  requests.DataSourceConfigurationRequest
		want *anypb.Any
	}{
		// Use type URLs that does not exist in the `resolver` to verify that this works even without the type being known.
		{
			req: requests.DataSourceConfigurationRequest{
				TypeUrl: "type.googleapis.com/com.example.Proto",
				Value:   &[]byte{},
			},
			want: &anypb.Any{
				TypeUrl: "type.googleapis.com/com.example.Proto",
				Value:   []byte{},
			},
		},
		{
			req: requests.DataSourceConfigurationRequest{
				TypeUrl: "type.googleapis.com/com.example.Proto",
				Value:   &[]byte{1, 2, 3},
			},
			want: &anypb.Any{
				TypeUrl: "type.googleapis.com/com.example.Proto",
				Value:   []byte{1, 2, 3},
			},
		},
	} {
		got, err := tc.req.ToProto(resolver)
		if err != nil {
			t.Errorf("tc.req.ToProto(*resolver) failed: %v", err)
		}

		diff := cmp.Diff(tc.want, got, protocmp.Transform())
		if diff != "" {
			t.Errorf("tc.req.ToProto(*resolver) is unexpectedly different (-want +got):\n%s", diff)
		}
	}
}

func TestConvertsTextprotoValue(t *testing.T) {
	resolver := makeResolver(t, testdata.Proto2ExampleFileDescriptorSet.File)

	empty := ""
	value123 := "value: 123"
	lalala7 := "lalala: 7"

	for _, tc := range []*struct {
		req  requests.DataSourceConfigurationRequest
		want *anypb.Any
	}{
		{
			req: requests.DataSourceConfigurationRequest{
				TypeUrl:        "type.googleapis.com/google.protobuf.UInt32Value",
				ValueTextproto: &empty,
			},
			want: &anypb.Any{
				TypeUrl: "type.googleapis.com/google.protobuf.UInt32Value",
				Value:   []byte{},
			},
		},
		{
			req: requests.DataSourceConfigurationRequest{
				TypeUrl:        "type.googleapis.com/google.protobuf.UInt32Value",
				ValueTextproto: &value123,
			},
			want: &anypb.Any{
				TypeUrl: "type.googleapis.com/google.protobuf.UInt32Value",
				Value: []byte{
					// first field (00001) + varint (000)
					0b00001000,
					// int value
					123,
				},
			},
		},
		{
			req: requests.DataSourceConfigurationRequest{
				TypeUrl:        "type.googleapis.com/android.sdv.telemetry.mcg.testdata.SomeProto2Message",
				ValueTextproto: &lalala7,
			},
			want: &anypb.Any{
				TypeUrl: "type.googleapis.com/android.sdv.telemetry.mcg.testdata.SomeProto2Message",
				Value: []byte{
					// first field (00001) + float (101)
					0b00001101,
					// float value
					0b00000000, 0b00000000, 0b11100000, 0b01000000,
				},
			},
		},
	} {
		got, err := tc.req.ToProto(resolver)
		if err != nil {
			t.Errorf("tc.req.ToProto(*resolver) failed: %v", err)
		}

		diff := cmp.Diff(tc.want, got, protocmp.Transform())
		if diff != "" {
			t.Errorf("tc.req.ToProto(*resolver) is unexpectedly different (-want +got):\n%s", diff)
		}
	}
}

func TestConvertsJsonValue(t *testing.T) {
	resolver := makeResolver(t, testdata.Proto2ExampleFileDescriptorSet.File)

	var value0 interface{} // 0
	if err := json.Unmarshal([]byte("0"), &value0); err != nil {
		t.Fatalf("json.Unmarshal(_, _) failed: %v", err)
	}

	var value123 interface{} // 123
	if err := json.Unmarshal([]byte("123"), &value123); err != nil {
		t.Fatalf("json.Unmarshal(_, _) failed: %v", err)
	}

	var lalala7 interface{} // { "lalala": 7 }
	if err := json.Unmarshal([]byte("{ \"lalala\": 7 }"), &lalala7); err != nil {
		t.Fatalf("json.Unmarshal(_, _) failed: %v", err)
	}

	for _, tc := range []*struct {
		req  requests.DataSourceConfigurationRequest
		want *anypb.Any
	}{
		{
			req: requests.DataSourceConfigurationRequest{
				TypeUrl:   "type.googleapis.com/google.protobuf.UInt32Value",
				ValueJson: &value0,
			},
			want: &anypb.Any{
				TypeUrl: "type.googleapis.com/google.protobuf.UInt32Value",
				Value:   []byte{},
			},
		},
		{
			req: requests.DataSourceConfigurationRequest{
				TypeUrl:   "type.googleapis.com/google.protobuf.UInt32Value",
				ValueJson: &value123,
			},
			want: &anypb.Any{
				TypeUrl: "type.googleapis.com/google.protobuf.UInt32Value",
				Value: []byte{
					// first field (00001) + varint (000)
					0b00001000,
					// int value
					123,
				},
			},
		},
		{
			req: requests.DataSourceConfigurationRequest{
				TypeUrl:   "type.googleapis.com/android.sdv.telemetry.mcg.testdata.SomeProto2Message",
				ValueJson: &lalala7,
			},
			want: &anypb.Any{
				TypeUrl: "type.googleapis.com/android.sdv.telemetry.mcg.testdata.SomeProto2Message",
				Value: []byte{
					// first field (00001) + float (101)
					0b00001101,
					// float value
					0b00000000, 0b00000000, 0b11100000, 0b01000000,
				},
			},
		},
	} {
		got, err := tc.req.ToProto(resolver)
		if err != nil {
			t.Errorf("tc.req.ToProto(*resolver) failed: %v", err)
		}

		diff := cmp.Diff(tc.want, got, protocmp.Transform())
		if diff != "" {
			t.Errorf("tc.req.ToProto(*resolver) is unexpectedly different (-want +got):\n%s", diff)
		}
	}
}
