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

package requests

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	proto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"sdv.googlesource.com/mcg/mcg/type_resolvers"
)

type DataSourceConfigurationRequest struct {
	TypeUrl string `json:"type_url,omitempty"`

	// Must be a valid serialized protocol buffer of the above specified type.
	// Mutually exclusive with `ValueTextproto` and `ValueJson`.
	Value *[]byte `json:"value,omitempty"`

	// Must be a valid textproto instance of the above specified type.
	// Mutually exclusive with `Value` and `ValueJson`.
	ValueTextproto *string `json:"value_textproto,omitempty"`

	// Must be a valid json representation of the above specified type.
	// Mutually exclusive with `Value` and `ValueTextproto`.
	ValueJson *interface{} `json:"value_json,omitempty"`
}

func (r *DataSourceConfigurationRequest) ToProto(resolver type_resolvers.EnrichedTypeResolver) (*anypb.Any, error) {
	// Check that exactly one of `Value`, `ValueTextproto`, or `ValueJson` is set.
	c := 0
	if r.Value != nil {
		c++
	}
	if r.ValueTextproto != nil {
		c++
	}
	if r.ValueJson != nil {
		c++
	}
	if c != 1 {
		return nil, fmt.Errorf("exactly one of `value`, `value_textproto`, or `value_json` must be set.")
	}

	if r.Value != nil {
		return &anypb.Any{
			TypeUrl: r.TypeUrl,
			Value:   *r.Value,
		}, nil
	} else if r.ValueTextproto != nil {
		return handleTextprotoConfiguration(resolver, r.TypeUrl, *r.ValueTextproto)
	} else if r.ValueJson != nil {
		return handleJsonConfiguration(resolver, r.TypeUrl, *r.ValueJson)
	} else {
		panic("unreachable")
	}
}

func handleTextprotoConfiguration(resolver type_resolvers.EnrichedTypeResolver, typeUrl string, textproto string) (*anypb.Any, error) {
	message, err := resolver.FindMessageByURL(typeUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to find message %q by URL: %w", typeUrl, err)
	}

	// Create an (empty) instance of the protobuf message referred to in `typeUrl`.
	configurationProto := message.New().Interface()

	// Populate the instance with the data from the textproto.
	unmarshalOptions := prototext.UnmarshalOptions{
		Resolver: &resolver,
	}
	if err := unmarshalOptions.Unmarshal([]byte(textproto), configurationProto); err != nil {
		return nil, fmt.Errorf("failed to parse textproto into proto: %w", err)
	}

	return intoAny(configurationProto)
}

func handleJsonConfiguration(resolver type_resolvers.EnrichedTypeResolver, typeUrl string, jsonValue interface{}) (*anypb.Any, error) {
	// First, we need to convert the parsed JSON back into a string, because we need to parse it using the
	// `protojson` parser, which only accepts strings.
	jsonString, err := json.Marshal(jsonValue)
	if err != nil {
		// This error should never be possible, because `jsonValue` was just parsed from JSON.
		return nil, fmt.Errorf("unexpectedly failed to convert json back into string: %w", err)
	}

	messageType, err := resolver.FindMessageByURL(typeUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to find message %q by URL: %w", typeUrl, err)
	}

	// Create an (empty) instance of the protobuf message referred to in `typeUrl`.
	configurationProto := messageType.New().Interface()

	// Populate the instance with the data from JSON.
	unmarshalOptions := protojson.UnmarshalOptions{
		Resolver: &resolver,
	}
	if err := unmarshalOptions.Unmarshal(jsonString, configurationProto); err != nil {
		return nil, fmt.Errorf("failed to parse JSON into proto: %w", err)
	}

	return intoAny(configurationProto)
}

func intoAny(m proto.Message) (*anypb.Any, error) {
	a, err := anypb.New(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal configuration proto into Any proto: %w", err)
	}
	return a, nil
}
