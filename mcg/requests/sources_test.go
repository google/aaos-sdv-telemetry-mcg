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
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"

	"sdv.googlesource.com/mcg/mcg/requests"
	"sdv.googlesource.com/mcg/mcg/session"
)

func getDefaultDeprecatedDataSourceRequest() *requests.DeprecatedDataSourceRequest {
	return &requests.DeprecatedDataSourceRequest{ServiceName: "ServiceName"}
}

func getDefaultDeprecatedAggregatorRequest() *requests.DeprecatedAggregatorRequest {
	return &requests.DeprecatedAggregatorRequest{
		Triggers:   []string{"TriggerName"},
		ResetOnGet: true,
		MessageBuilder: requests.MessageBuilderRequest{
			MessageType: "MessageType",
		},
	}
}

func TestCheckDoesNotHaveMultipleSourcesWithOneSourcePasses(t *testing.T) {
	for _, req := range []*requests.DeprecatedSourceRequest{
		{
			Name:    "DataSource",
			Service: getDefaultDeprecatedDataSourceRequest(),
		},
		{
			Name:        "Aggregator",
			Aggregation: getDefaultDeprecatedAggregatorRequest(),
		},
	} {
		if err := requests.CheckDoesNotHaveMultipleSources(req); err != nil {
			t.Fatal("Defining a single source should be allowed.")
		}
	}
}

func TestDeprecatedSourceRequestWithMultipleSourcesFails(t *testing.T) {
	req := &requests.DeprecatedSourceRequest{
		Name:        "string",
		Service:     getDefaultDeprecatedDataSourceRequest(),
		Aggregation: getDefaultDeprecatedAggregatorRequest(),
	}

	err := requests.CheckDoesNotHaveMultipleSources(req)
	if err == nil {
		t.Fatal("Defining multiple sources should not be allowed.")
	}
}

func TestInvalidConnectionTypeFails(t *testing.T) {
	_, err := requests.GetConnectionType("nonsense")
	if err == nil {
		t.Fatal("String that is not an existing enum type should return an error.")
	}
}

func TestOnDemandConnectionType(t *testing.T) {
	ct, err := requests.GetConnectionType("ON_DEMAND")
	if err != nil {
		t.Fatalf("GetConnectionType(\"ON_DEMAND\") failed: %v", err)
	}
	if ct.String() != "ON_DEMAND" {
		t.Errorf("GetConnectionType(\"ON_DEMAND\") = %v, want ON_DEMAND", ct)
	}
}

func TestValidSubSamplingIntervalSucceeds(t *testing.T) {
	for _, d := range []struct {
		name                  string
		subSamplingIntervalMs float64
		want                  *durationpb.Duration
	}{
		{name: "zero", subSamplingIntervalMs: 0, want: nil},
		{name: "positive", subSamplingIntervalMs: 1234, want: durationpb.New(1234 * time.Millisecond)},
	} {
		t.Run(d.name, func(t *testing.T) {
			var s *session.Session
			req := requests.DataSourceRequest{
				Name:                  "Name",
				SourceIdentifier:      "ServiceName",
				SubSamplingIntervalMs: d.subSamplingIntervalMs,
			}

			proto, err := req.ToProto(s)
			if err != nil {
				t.Fatalf("req.ToProto(%v) = %v, %v, want _, nil", s, proto, err)
			}
			got := proto.GetDataSource().GetSubSamplingInterval()
			if diff := cmp.Diff(d.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("proto.GetDataSource().SubSamplingInterval is unexpectedly different (-want +got):\n%s", diff)
			}
		})
	}
}

func TestInvalidSubSamplingIntervalFails(t *testing.T) {
	const EXPECTED_ERROR = "does not map to a valid sub-sampling interval"

	for _, d := range []struct {
		name                  string
		subSamplingIntervalMs float64
	}{
		{name: "negative", subSamplingIntervalMs: -3},
		{name: "too_large", subSamplingIntervalMs: 4389438943893348943},
	} {
		t.Run(d.name, func(t *testing.T) {
			var s *session.Session
			req := requests.DataSourceRequest{
				Name:                  "Name",
				SourceIdentifier:      "ServiceName",
				SubSamplingIntervalMs: d.subSamplingIntervalMs,
			}

			proto, err := req.ToProto(s)
			if proto != nil || err == nil {
				t.Fatalf("req.ToProto(%v) = %v, %v, want nil, _", s, proto, err)
			}
			if !strings.Contains(err.Status.Message, EXPECTED_ERROR) {
				t.Errorf("err.Status.Message = %q, want a string containing %q", err.Status.Message, EXPECTED_ERROR)
			}
		})
	}
}

func TestDataSourceFetchLastMessage(t *testing.T) {
	testCases := []struct {
		name             string
		fetchLastMessage bool
	}{
		{
			name:             "FetchLastMessage_True",
			fetchLastMessage: true,
		},
		{
			name:             "FetchLastMessage_False",
			fetchLastMessage: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var s *session.Session
			req := requests.DataSourceRequest{
				Name:             "TestSource",
				SourceIdentifier: "TestService",
				FetchLastMessage: tc.fetchLastMessage,
			}

			proto, err := req.ToProto(s)
			if err != nil {
				t.Fatalf("req.ToProto(%v) returned error %v, want nil", s, err)
			}

			if got := proto.GetDataSource().GetFetchLastMessage(); got != tc.fetchLastMessage {
				t.Errorf("proto.GetDataSource().GetFetchLastMessage() = %v, want %v", got, tc.fetchLastMessage)
			}
		})
	}
}

func TestDataSourceRequestValidateStandard(t *testing.T) {
	testCases := []struct {
		name        string
		req         requests.DataSourceRequest
		expectError bool
	}{
		{
			name:        "Valid",
			req:         requests.DataSourceRequest{SourceIdentifier: "sid"},
			expectError: false,
		},
		{
			name:        "MissingSourceIdentifier",
			req:         requests.DataSourceRequest{},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.ValidateCanonical()
			if tc.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestAggregatorRequestValidateStandard(t *testing.T) {
	testCases := []struct {
		name        string
		req         requests.AggregatorRequest
		expectError bool
	}{
		{
			name:        "Valid",
			req:         requests.AggregatorRequest{Triggers: []string{"t1"}},
			expectError: false,
		},
		{
			name:        "MissingTriggers",
			req:         requests.AggregatorRequest{},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.ValidateCanonical()
			if tc.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestDeprecatedSourceRequestValidateDeprecated(t *testing.T) {
	testCases := []struct {
		name        string
		req         requests.DeprecatedSourceRequest
		expectError bool
	}{
		{
			name:        "ValidService",
			req:         requests.DeprecatedSourceRequest{Service: &requests.DeprecatedDataSourceRequest{}},
			expectError: false,
		},
		{
			name:        "ValidAggregation",
			req:         requests.DeprecatedSourceRequest{Aggregation: &requests.DeprecatedAggregatorRequest{}},
			expectError: false,
		},
		{
			name:        "BothSet",
			req:         requests.DeprecatedSourceRequest{Service: &requests.DeprecatedDataSourceRequest{}, Aggregation: &requests.DeprecatedAggregatorRequest{}},
			expectError: true,
		},
		{
			name:        "NeitherSet",
			req:         requests.DeprecatedSourceRequest{},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.ValidateDeprecated()
			if tc.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
