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

package requests_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"sdv.googlesource.com/mcg/mcg/requests"
	"sdv.googlesource.com/mcg/mcg/testhelper"
)

func TestParseDataSourceConfigurationFromDescriptorProtos(t *testing.T) {
	var jsonValue any
	if err := json.Unmarshal([]byte(`{"speed": 100}`), &jsonValue); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	textprotoValue := `speed: 100`
	binaryValue := []byte{0x0d, 0x00, 0x00, 0xc8, 0x42} // speed: 100

	testCases := []struct {
		name          string
		configuration *requests.DataSourceConfigurationRequest
	}{
		{
			name: "ValueJson",
			configuration: &requests.DataSourceConfigurationRequest{
				TypeUrl:   "type.googleapis.com/android.sdv.telemetry.mcg.testdata.VehicleSpeed",
				ValueJson: &jsonValue,
			},
		},
		{
			name: "ValueTextproto",
			configuration: &requests.DataSourceConfigurationRequest{
				TypeUrl:        "type.googleapis.com/android.sdv.telemetry.mcg.testdata.VehicleSpeed",
				ValueTextproto: &textprotoValue,
			},
		},
		{
			name: "Value",
			configuration: &requests.DataSourceConfigurationRequest{
				TypeUrl: "type.googleapis.com/android.sdv.telemetry.mcg.testdata.VehicleSpeed",
				Value:   &binaryValue, // speed: 100
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fd := testhelper.GetSpeedFdWithDependencies()
			fdBytes, err := proto.Marshal(fd)
			if err != nil {
				t.Fatalf("failed to marshal file descriptor set: %v", err)
			}

			req := &requests.MetricsConfigRequest{
				DataSources: []requests.DataSourceRequest{
					{
						Name:             "speed_source",
						SourceIdentifier: "android.sdv.telemetry.mcg.testdata.VehicleSpeed",
						Configuration:    tc.configuration,
					},
				},
				DescriptorProtos: [][]byte{fdBytes},
			}

			sess, errs := req.ToSession(context.Background())
			if len(errs) > 0 {
				t.Fatalf("req.ToSession() failed with errors: %v", errs)
			}

			if sess == nil {
				t.Fatal("req.ToSession() returned a nil session")
			}

			pub, ok := sess.Sources["speed_source"]
			if !ok {
				t.Fatal("Source 'speed_source' not found in session")
			}

			dataSource := pub.GetDataSource()
			if dataSource == nil {
				t.Fatal("Source is not a data source")
			}

			if dataSource.GetSourceIdentifier() != "android.sdv.telemetry.mcg.testdata.VehicleSpeed" {
				t.Errorf("Unexpected source identifier: got %q, want %q", dataSource.GetSourceIdentifier(), "android.sdv.telemetry.mcg.testdata.VehicleSpeed")
			}
		})
	}
}

func TestMetricsConfigWithDataSources(t *testing.T) {
	req := &requests.MetricsConfigRequest{
		DataSources: []requests.DataSourceRequest{
			{
				Name:             "ds1",
				SourceIdentifier: "source1",
			},
		},
	}

	sess, errs := req.ToSession(context.Background())
	if len(errs) > 0 {
		t.Fatalf("req.ToSession() failed with errors: %v", errs)
	}

	if _, ok := sess.Sources["ds1"]; !ok {
		t.Error("Source 'ds1' not found in session")
	}
}

func TestMetricsConfigWithAggregators(t *testing.T) {
	req := &requests.MetricsConfigRequest{
		Aggregators: []requests.AggregatorRequest{
			{
				Name:     "agg1",
				Triggers: []string{"t1"},
				MessageBuilder: requests.MessageBuilderRequest{
					MessageType: ".mt1",
				},
			},
		},
	}

	sess, errs := req.ToSession(context.Background())
	if len(errs) > 0 {
		t.Fatalf("req.ToSession() failed with errors: %v", errs)
	}

	if _, ok := sess.Sources["agg1"]; !ok {
		t.Error("Source 'agg1' not found in session")
	}
}

func TestMetricsConfigTriggerAliases(t *testing.T) {
	req := &requests.MetricsConfigRequest{
		StopTrigger:       "stop_trigger",
		DeactivateTrigger: "deactivate_trigger",
	}

	sess, errs := req.ToSession(context.Background())
	if len(errs) > 0 {
		t.Fatalf("req.ToSession() failed with errors: %v", errs)
	}

	if sess.StopTrigger != "stop_trigger" {
		t.Errorf("sess.StopTrigger = %q, want %q", sess.StopTrigger, "stop_trigger")
	}
	if sess.DeactivateTrigger != "deactivate_trigger" {
		t.Errorf("sess.DeactivateTrigger = %q, want %q", sess.DeactivateTrigger, "deactivate_trigger")
	}
}

func TestMetricsConfigDuplicateNames(t *testing.T) {
	req := &requests.MetricsConfigRequest{
		DataSources: []requests.DataSourceRequest{
			{Name: "p1", SourceIdentifier: "s1"},
			{Name: "p1", SourceIdentifier: "s2"},
		},
	}

	_, errs := req.ToSession(context.Background())
	if len(errs) == 0 {
		t.Fatal("Expected error but got none")
	}
	found := false
	for _, err := range errs {
		if err != nil && strings.Contains(err.Error(), "Name \"p1\" is used twice") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error about duplicate source named \"p1\", got %v", errs)
	}
}

func TestMetricsConfigRetainAggregators(t *testing.T) {
	testCases := []struct {
		name  string
		value bool
	}{
		{"RetainTrue", true},
		{"RetainFalse", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &requests.MetricsConfigRequest{
				RetainAggregationsOnStop: tc.value,
			}

			sess, errs := req.ToSession(context.Background())
			if len(errs) > 0 {
				t.Fatalf("req.ToSession() failed with errors: %v", errs)
			}

			if sess.RetainAggregationsOnStop != tc.value {
				t.Errorf("sess.RetainAggregationsOnStop = %v, want %v", sess.RetainAggregationsOnStop, tc.value)
			}
		})
	}
}
