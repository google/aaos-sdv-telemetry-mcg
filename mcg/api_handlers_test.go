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

package mcg_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/descriptorpb"

	"sdv.googlesource.com/mcg/mcg"
	"sdv.googlesource.com/mcg/mcg/constants"
	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
	"sdv.googlesource.com/mcg/mcg/requests"
	"sdv.googlesource.com/mcg/mcg/testhelper"
	"sdv.googlesource.com/mcg/mcg/type_resolvers"
	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

const (
	CONTENT_TYPE_HEADER_NAME = "Content-Type"
)

func performPostRequest(router *gin.Engine, path string, contentType string, accept string, body []byte) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set(CONTENT_TYPE_HEADER_NAME, contentType)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	router.ServeHTTP(w, req)
	return w
}

type uuidTestCase struct {
	name              string
	uuid              string
	wantErrContaining string
}

func getInvalidUuidTestCases() []uuidTestCase {
	return []uuidTestCase{
		{
			name:              "invalid_uuid",
			uuid:              "invalid",
			wantErrContaining: "invalid UUID length: 7",
		},
		{
			name:              "nil_uuid",
			uuid:              uuid.Nil.String(),
			wantErrContaining: "must not be the Nil UUID",
		},
		{
			name:              "max_uuid",
			uuid:              uuid.Max.String(),
			wantErrContaining: "must not be the Max UUID",
		},
		{
			name:              "no_hyphens",
			uuid:              "a1a2a3a4b1b2c1c2d1d2d3d4d5d6d7d8",
			wantErrContaining: "must be a valid UUID in the standard lowercase hyphenated format",
		},
		{
			name:              "urn",
			uuid:              "urn:uuid:A1A2A3A4-B1B2-C1C2-D1D2-D3D4D5D6D7D8",
			wantErrContaining: "must be a valid UUID in the standard lowercase hyphenated format",
		},
		{
			name:              "braced",
			uuid:              "{a1a2a3a4-b1b2-c1c2-d1d2-d3d4d5d6d7d8}",
			wantErrContaining: "must be a valid UUID in the standard lowercase hyphenated format",
		},
		// We deliberately do not allow non-lowercase UUIDs, even though RFC 9562 technically allows
		// them. This is because we internally store the UUID in numeric format, which loses
		// information about the UUID's casing. However, our Binder and Client APIs operate on
		// strings, and callers may not be aware that they have to compare UUIDs case-insensitively.
		// By enforcing lowercase UUIDs, we remove this potential pitfall for our clients.
		{
			name:              "mixed_case",
			uuid:              "A1a2a3a4-b1B2-c1c2-d1d2-d3d4d5d6d7d8",
			wantErrContaining: "must be a valid UUID in the standard lowercase hyphenated format",
		},
		{
			name:              "upper_case",
			uuid:              "A1A2A3A4-B1B2-C1C2-D1D2-D3D4D5D6D7D8",
			wantErrContaining: "must be a valid UUID in the standard lowercase hyphenated format",
		},
	}
}

func fixtureTestV1(t *testing.T, jsonReq, textproto string, opts ...cmp.Option) {
	fixtureTest(t, constants.APIVersionV1, jsonReq, textproto, opts...)
}

func fixtureTestCurrent(t *testing.T, jsonReq, textproto string, opts ...cmp.Option) {
	fixtureTest(t, constants.CurrentAPIVersion, jsonReq, textproto, opts...)
}

// Complete input-output test with the specified json and text proto.
//
// Make one call to this function per test or per subtest.
func fixtureTest(t *testing.T, apiVersion constants.APIVersion, jsonReq, textproto string, opts ...cmp.Option) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, false)

	w := performPostRequest(router,
		fmt.Sprintf("/api/%s/generate_metrics_config", apiVersion),
		"application/json", "application/x-protobuf", []byte(jsonReq))

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("Failure response: %d\n%v", w.Result().StatusCode, w.Body.String())
		return
	}
	if contentType := w.Result().Header.Get(CONTENT_TYPE_HEADER_NAME); contentType != mcg.ContentTypeBinaryProtoMetricsConfig {
		t.Errorf("Unexpected Content-Type header, want %q, got %q", mcg.ContentTypeBinaryProtoMetricsConfig, contentType)
		return
	}

	mcReq := new(requests.MetricsConfigRequest)
	if err := json.Unmarshal([]byte(jsonReq), mcReq); err != nil {
		t.Fatalf("json.Unmarshal(%q) failed: %v", jsonReq, err)
	}

	vehicleSignalsFdSet, err := type_resolvers.UnmarshalFileDescriptorSet(mcReq.VehicleSignals)
	if err != nil {
		t.Fatalf("type_resolvers.UnmarshalFileDescriptorSet(mcReq.VehicleSignals) failed: %v", err)
	}
	descriptorProtosFdSet := new(descriptorpb.FileDescriptorSet)
	for i, fdSetBytes := range mcReq.DescriptorProtos {
		fdSet, err := type_resolvers.UnmarshalFileDescriptorSet(fdSetBytes)
		if err != nil {
			t.Fatalf("type_resolvers.UnmarshalFileDescriptorSet(mcReq.DescriptorProtos[%d]) failed: %v", i, err)
		}
		descriptorProtosFdSet.File = append(descriptorProtosFdSet.File, fdSet.File...)
	}

	typeResolver, err := type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(
		testhelper.MergeFileDescriptorSets(vehicleSignalsFdSet, descriptorProtosFdSet),
	)
	if err != nil {
		t.Fatalf("type_resolvers.NewEnrichedTypeResolverFromFileDescriptorSet(_) failed: %v", err)
	}

	unmarshalOptions := prototext.UnmarshalOptions{
		Resolver: typeResolver,
	}
	var result, expected pb.MetricsConfig
	if err := unmarshalOptions.Unmarshal([]byte(textproto), &expected); err != nil {
		t.Fatalf("prototext.Unmarshal(%q, _) failed: %v", textproto, err)
	}
	if err := proto.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Errorf("proto.Unmarshal(%q) failed: %v", w.Body.String(), err)
	}

	testhelper.AssertMetricsConfigEqual(t, &expected, &result, opts...)
}

func fixtureTestNoExpressions(t *testing.T, json, textproto string) {
	fixtureTestCurrent(t, json, textproto,
		protocmp.IgnoreFields(&pb.MetricsConfig{}, protoreflect.Name("expression_nodes")),
	)
}

func TestBasicTriggers(t *testing.T) {
	fixtureTestNoExpressions(t, `{
		"triggers": [{
			"name": "trig3",
			"conditional": {
				"triggers": ["trig2"],
				"condition_type": {
					"is_true": {}
				},
				"expression": "pub.Value"
			}
		}, {
			"name": "trig1",
			"periodic": {
				"period_ms": 1200
			}
		}, {
			"name": "trig4",
			"periodic": {
				"period_ms": 12000
			}
		}, {
			"name": "trig2",
			"data": {
				"source_name": "pub"
			}
		}, {
			"name": "trig5",
			"periodic": {
				"triggers": ["trig2"],
				"period_ms": 20,
				"count": 5
			}
		}],
		"data_sources": [{
			"name": "pub",
			"source_identifier": "TripBegin",
			"connection_type": "SUBSCRIPTION"
		}]
	}`, `
	version: 3758096386
	triggers: {
		name: "trig1"
		periodic_trigger: {
			interval: {
				seconds: 1
				nanos: 200000000
			}
		}
	}
	triggers {
		name: "trig2"
		data_trigger: {
			source_name: "pub"
		}
	}
	triggers {
		name: "trig3"
		conditional_trigger {
			trigger_names: "trig2"
			is_true: {}
			selector_node_index: 0
		}
	}
	expression_nodes {
		field_leaf_node {
			source_name: "pub"
			field_names: "Value"
		}
	}
	triggers: {
		name: "trig4"
		periodic_trigger: {
			interval: { seconds: 12 }
		}
	}
	sources {
		name: "pub"
		data_source {
			source_identifier: "TripBegin"
			connection_type: SUBSCRIPTION
		}
	}
	triggers: {
		name: "trig5"
		periodic_trigger: {
			trigger_names: "trig2"
			interval: {
				nanos: 20000000
			}
			count: 5
		}
	}`)
}

func TestBasicSources(t *testing.T) {
	fixtureTestNoExpressions(t, `{
		"triggers": [{
			"name": "trig1",
			"periodic": {
				"period_ms": 5000
			}
		}],
		"data_sources": [{
			"name": "pub1",
			"source_identifier": "TripBegin"
		}, {
			"name": "pub2",
			"source_identifier": "TripEnd",
			"connection_type": "ON_DEMAND"
		}, {
			"name": "pub3",
			"source_identifier": "WithConfiguration",
			"configuration": {
				"type_url": "type.example.com/VideoConfiguration",
				"value": "CAE="
			}
		}],
		"aggregators": [{
			"name": "pub4",
			"trigger_names": ["trig1"],
			"message_builder": {"message_type": ".google.protobuf.Empty"}
		}, {
			"name": "pub5",
			"trigger_names": ["trig1"],
			"message_builder": {
				"message_type": ".google.protobuf.UInt32Value",
				"field_assignments": [{
					"field_name": "value",
					"aggregation": {
						"@type": "count"
					}
				}]
			}
		}, {
			"name": "pub6",
			"trigger_names": ["trig1"],
			"message_builder": {
				"message_type": ".google.protobuf.UInt32Value",
				"field_assignments": [{
					"field_name": "value",
					"aggregation": {
						"@type": "vector",
						"max_length": 5,
						"expression": "437918234"
					}
				}]
			}
		}]
	}`, `
	version: 3758096386
	triggers {
		name:"trig1", periodic_trigger:{interval: {seconds: 5}}
	}
	sources {
		name: "pub1"
		data_source {
			source_identifier: "TripBegin"
			connection_type: SUBSCRIPTION,
		}
	}
	sources {
		name: "pub2"
		data_source {
			source_identifier: "TripEnd"
			connection_type: ON_DEMAND,
		}
	}
	sources {
		name: "pub3"
		data_source {
			source_identifier: "WithConfiguration"
			connection_type: SUBSCRIPTION,
			configuration {
				type_url: "type.example.com/VideoConfiguration",
				value: "\x08\x01"
			}
		}
	}
	sources {
		name: "pub4"
		aggregator {
			trigger_names: "trig1"
			message_builder {
				message_type: ".google.protobuf.Empty"
			}
		}
	}
	sources {
		name: "pub5"
		aggregator {
			trigger_names: "trig1"
			message_builder {
				message_type: ".google.protobuf.UInt32Value"
				field_assignments {
					field_name: "value"
					count_aggregation {}
				}
			}
		}
	}
	sources {
		name: "pub6"
		aggregator {
			trigger_names: "trig1"
			message_builder {
				message_type: ".google.protobuf.UInt32Value"
				field_assignments {
					field_name: "value"
					vector_aggregation {
						expression_node_index: 0
						max_length: 5
					}
				}
			}
		}
	}`)
}

func TestBasicReportConfigs(t *testing.T) {
	fixtureTestNoExpressions(t, `{
		"triggers": [{
			"name": "trig1",
			"periodic": {
				"period_ms": 5000
			}
		},
		{
			"name": "trig2",
			"periodic": {
				"period_ms": 10000
			}
		}],
		"report_configs": [{
			"name": "report1",
			"report_incomplete": true,
			"message_builder": {"message_type": ".google.protobuf.Empty"}
		}, {
			"name": "report2",
			"trigger_names": ["trig1", "trig2"],
			"message_builder": {"message_type": ".google.protobuf.Empty"}
		}]
	}`, `
	version: 3758096386
	triggers {
		name:"trig1", periodic_trigger:{interval: {seconds: 5}}
	}
	triggers {
		name:"trig2", periodic_trigger:{interval: {seconds: 10}}
	}
	metrics_report_configs {
		name: "report1"
		report_incomplete: true
		message_builder { message_type: ".google.protobuf.Empty" }
	}
	metrics_report_configs {
		name: "report2"
		trigger_names: "trig1"
		trigger_names: "trig2"
		message_builder { message_type: ".google.protobuf.Empty" }
	}
	`)
}

func TestExpressionNodes(t *testing.T) {
	fixtureTestCurrent(t, `{
		"triggers": [{
			"name": "periodic_16s",
			"periodic": {"period_ms": 16000}
		}],
		"aggregators": [{
			"name": "expr1", ` /* const node */ +`
			"trigger_names": ["periodic_16s"],
			"message_builder": {
				"message_type": ".google.protobuf.UInt32Value",
				"field_assignments": [{
					"field_name": "value",
					"aggregation": {
						"@type": "none",
						"expression": "12345"
					}
				}]
			}
		}, {
			"name": "expr2", ` /* subtract a negative + source field access */ +`
			"trigger_names": ["periodic_16s"],
			"message_builder": {
				"message_type": ".google.protobuf.UInt32Value",
				"field_assignments": [{
					"field_name": "value",
					"aggregation": {
						"@type": "none",
						"expression": "expr1.value --4"
					}
				}]
			}
		}, {
			"name": "expr3", ` /* nested combination nodes + const reuse */ +`
			"trigger_names": ["periodic_16s"],
			"message_builder": {
				"message_type": ".google.protobuf.UInt32Value",
				"field_assignments": [{
					"field_name": "value",
					"aggregation": {
						"@type": "none",
						"expression": "(12345 + -4) / 100"
					}
				}]
			}
		}]
	}`, `
	version: 3758096386
	expression_nodes {
		constant_leaf_node {
			int32_value: 12345
		}
	}
	expression_nodes {
		field_leaf_node {
			source_name: "expr1"
			field_names: "value"
		}
	}
	expression_nodes {
		constant_leaf_node {
			int32_value: -4
		}
	}
	expression_nodes {
		combination_node {
			arithmetic_operator: SUBTRACT
			left_index: 1
			right_index: 2
		}
	}
	expression_nodes {
		combination_node {
			arithmetic_operator: ADD
			left_index: 0
			right_index: 2
		}
	}
	expression_nodes {
		constant_leaf_node {
			int32_value: 100
		}
	}
	expression_nodes {
		combination_node {
			arithmetic_operator: DIVIDE
			left_index: 4
			right_index: 5
		}
	}
	triggers {
		name: "periodic_16s"
		periodic_trigger {
			interval: { seconds: 16 }
		}
	}
	sources {
		name: "expr1"
		aggregator {
			trigger_names: "periodic_16s"
			message_builder {
				message_type: ".google.protobuf.UInt32Value"
				field_assignments {
					field_name: "value"
					no_aggregation {
						expression_node_index: 0
					}
				}
			}
		}
	}
	sources {
		name: "expr2"
		aggregator {
			trigger_names: "periodic_16s"
			message_builder {
				message_type: ".google.protobuf.UInt32Value"
				field_assignments {
					field_name: "value"
					no_aggregation {
						expression_node_index: 3
					}
				}
			}
		}
	}
	sources {
		name: "expr3"
		aggregator {
			trigger_names: "periodic_16s"
			message_builder {
				message_type: ".google.protobuf.UInt32Value"
				field_assignments {
					field_name: "value"
					no_aggregation {
						expression_node_index: 6
					}
				}
			}
		}
	}
	`)
}

const (
	BUG_378900418_JSON_FILENAME                              = "testdata/378900418.json"
	BUG_378900418_TEXTPROTO_FILENAME                         = "testdata/378900418.textproto"
	BUG_380905512_JSON_FILENAME                              = "testdata/380905512.json"
	BUG_380905512_TEXTPROTO_FILENAME                         = "testdata/380905512.textproto"
	COMPREHENSIVE_DESCRIPTOR_OPTIMIZATION_JSON_FILENAME      = "testdata/comprehensive_descriptor_optimization.json"
	COMPREHENSIVE_DESCRIPTOR_OPTIMIZATION_TEXTPROTO_FILENAME = "testdata/comprehensive_descriptor_optimization.textproto"
	CONDITIONAL_TRIGGER_JSON_FILENAME                        = "testdata/conditional_trigger.json"
	CONDITIONAL_TRIGGER_TEXTPROTO_FILENAME                   = "testdata/conditional_trigger.textproto"
	INIT_NODE_INDEX_JSON_FILENAME                            = "testdata/init_node_index.json"
	INIT_NODE_INDEX_TEXTPROTO_FILENAME                       = "testdata/init_node_index.textproto"
	CUSTOM_AGGREGATION_MESSAGE_TYPE_JSON_FILENAME            = "testdata/custom_aggregation_message_type.json"
	CUSTOM_AGGREGATION_MESSAGE_TYPE_TEXTPROTO_FILENAME       = "testdata/custom_aggregation_message_type.textproto"
	API_COMPATIBILITY_CANONICAL_JSON_FILENAME                = "testdata/api_version_compatibility/canonical_version.json"
	API_COMPATIBILITY_LEGACY_JSON_FILENAME                   = "testdata/api_version_compatibility/deprecated_version.json"
	API_COMPATIBILITY_OUTPUT_TEXTPROTO_FILENAME              = "testdata/api_version_compatibility/output.textproto"
	EIPF_B_JSON_FILENAME                                     = "testdata/eipf_b.json"
	EIPF_B_TEXTPROTO_FDS_FILENAME                            = "testdata/eipf_b_fds.textproto"
	EIPF_B_TEXTPROTO_FILENAME                                = "testdata/eipf_b.textproto"
	INFER_JSON_FILENAME                                      = "testdata/infer.json"
	INFER_TEXTPROTO_FILENAME                                 = "testdata/infer.textproto"
	DATA_SOURCE_MESSAGE_TYPES_JSON_FILENAME                  = "testdata/data_source_message_types.json"
	DATA_SOURCE_MESSAGE_TYPES_TEXTPROTO_FILENAME             = "testdata/data_source_message_types.textproto"
	SOURCE_CONFIGURATION_JSON_FILENAME                       = "testdata/source_configuration.json"
	SOURCE_CONFIGURATION_TEXTPROTO_FILENAME                  = "testdata/source_configuration.textproto"
)

func TestMilestoneEIPF_B(t *testing.T) {
	fixtureTestCurrent(t, string(FileAsBytes(EIPF_B_JSON_FILENAME)), string(FileAsBytes(EIPF_B_TEXTPROTO_FILENAME)))
}

func TestTextFormat(t *testing.T) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, false)

	w := performPostRequest(router,
		fmt.Sprintf("/api/%s/generate_metrics_config", constants.CurrentAPIVersion),
		"application/json", "text/x-protobuf", FileAsBytes(EIPF_B_JSON_FILENAME))

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("Failure response: %d\n%v", w.Result().StatusCode, w.Body.String())
		return
	}
	if contentType := w.Result().Header.Get(CONTENT_TYPE_HEADER_NAME); contentType != mcg.ContentTypeTextProtoMetricsConfig {
		t.Errorf("Unexpected Content-Type header, want %q, got %q", mcg.ContentTypeTextProtoMetricsConfig, contentType)
		return
	}

	strResp := w.Body.String()
	if !strings.Contains(strResp, "3758096386" /* version */) {
		t.Error("version not found in text format output")
	}
}

func TestValidateWithTextprotoPasses(t *testing.T) {
	textProtoAsBytes := []byte(FileAsBytes(EIPF_B_TEXTPROTO_FILENAME))
	var mc pb.MetricsConfig
	if err := prototext.Unmarshal(textProtoAsBytes, &mc); err != nil {
		t.Fatal(err)
	}
	// In order for the validation to pass, we have to parse, manually add the uuid and reparse the
	// textproto as in the other tests where it's used, the UUID is generated by the metrics config
	// generator.
	mc.SetUuid(uuid.New().String())

	t1, err := prototext.MarshalOptions{Indent: "\t"}.Marshal(&mc)
	if err != nil {
		t.Fatal(err)
	}
	t2, err := mcg.TextprotoMarshal(&mc, false)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		description string
		textproto   []byte
	}{
		{
			description: "text_proto_with_many_colons",
			textproto:   t1,
		},
		{
			description: "text_proto_with_fewer_colons",
			textproto:   t2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			ctx := context.Background()
			router, _ := setupServer(ctx, t, false)

			w := performPostRequest(router,
				fmt.Sprintf("/api/%s/validate_metrics_config", constants.CurrentAPIVersion),
				mcg.CONTENT_TYPE_TEXT_X_PROTOBUF, "", tc.textproto)

			if w.Result().StatusCode != http.StatusOK {
				t.Errorf("Failure response: %d\n%v", w.Result().StatusCode, w.Body.String())
			}
		})
	}
}

func TestValidateWithTextprotoFails(t *testing.T) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, false)

	textProtoAsBytes := []byte(FileAsBytes(EIPF_B_TEXTPROTO_FILENAME))
	w := performPostRequest(router,
		fmt.Sprintf("/api/%s/validate_metrics_config", constants.CurrentAPIVersion),
		mcg.CONTENT_TYPE_TEXT_X_PROTOBUF, "", textProtoAsBytes)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("Request should have failed and returned an error, but got StatusCode: %d", w.Result().StatusCode)
	}

	jsonErr, err := mcgerrors.JsonErrorResponseFromBytes(w.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	got, err := jsonErr.BadRequestFromDetail(0)
	if err != nil {
		t.Fatal(err)
	}
	wantFieldViol0 := mcgerrors.UuidMissing.Details[0].(*errdetails.BadRequest).FieldViolations[0]
	wantFormattedErrDescription := mcgerrors.FormatErrorMsg(mcgerrors.UuidMissing.Status.Message, wantFieldViol0.Description)

	if got.FieldViolations[0].Field != wantFieldViol0.Field || got.FieldViolations[0].Description != wantFormattedErrDescription {
		t.Errorf("Request should have returned an error complaining about the UUID missing but instead got: %s", w.Body.String())
	}
}

func TestValidateWithInvalidUuidFails(t *testing.T) {
	for _, tc := range getInvalidUuidTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			router, _ := setupServer(ctx, t, false)

			textProtoAsBytes := []byte(fmt.Sprintf(`uuid: %q`, tc.uuid))
			w := performPostRequest(router,
				fmt.Sprintf("/api/%s/validate_metrics_config", constants.CurrentAPIVersion),
				mcg.CONTENT_TYPE_TEXT_X_PROTOBUF, "", textProtoAsBytes)

			if w.Result().StatusCode != http.StatusBadRequest {
				t.Errorf("Request should have failed and returned an error, but got StatusCode: %d", w.Result().StatusCode)
			}

			if got := w.Body.String(); !strings.Contains(got, tc.wantErrContaining) {
				t.Errorf("w.Body.String() = %q, want containing %q", got, tc.wantErrContaining)
			}
		})
	}
}

func TestGenerateAndValidateWithProtobufBytesPasses(t *testing.T) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, false)

	w1 := performPostRequest(router,
		fmt.Sprintf("/api/%s/generate_metrics_config", constants.CurrentAPIVersion),
		"application/json", "application/x-protobuf", FileAsBytes(EIPF_B_JSON_FILENAME))

	if w1.Result().StatusCode != http.StatusOK {
		t.Fatalf("Failure response: %d\n%v", w1.Result().StatusCode, w1.Body.String())
	}
	if contentType := w1.Result().Header.Get(CONTENT_TYPE_HEADER_NAME); contentType != mcg.ContentTypeBinaryProtoMetricsConfig {
		t.Errorf("Unexpected Content-Type header, want %q, got %q", mcg.ContentTypeBinaryProtoMetricsConfig, contentType)
		return
	}

	body, err := io.ReadAll(w1.Body)
	if err != nil {
		t.Fatalf("Failure reading the response body: %#v", err)
	}

	w2 := performPostRequest(router,
		fmt.Sprintf("/api/%s/validate_metrics_config", constants.CurrentAPIVersion),
		mcg.CONTENT_TYPE_APP_X_PROTOBUF, "", body)

	if w2.Result().StatusCode != http.StatusOK {
		t.Errorf("Failure response: %d\n%v", w2.Result().StatusCode, w2.Body.String())
	}
}

func TestFieldType(t *testing.T) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, false)

	var mcr requests.MetricsConfigRequest

	json.Unmarshal([]byte(`{
		"report_configs": [{
			"name": "abcXYZ",
			"report_incomplete": true,
			"message_builder": {
				"field_assignments": [{
					"field_name": "field1",
					"field_type": ".google.protobuf.Int32Value",
					"aggregation": {
						"@type": "none",
						"expression": "4.2"
					}
				}]
			}
		}]
	}`), &mcr)

	w := httptest.NewRecorder()
	c := gin.CreateTestContextOnly(w, router)
	sess, errorList := mcr.ToSession(c)
	if len(errorList) > 0 {
		t.Fatal(errorList)
	}

	if len(sess.FieldTypes) == 0 {
		t.Error("no types stashed")
	}
}

func TestIgnoreValidations(t *testing.T) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, false)

	invalidPayload := `{
		"report_configs": [{
			"name": "report1",
			"report_incomplete": true,
			"message_builder": {"message_type": ".google.protobuf.Empty"}
		}, {
			"name": "report2",
			"trigger_names": ["trig1", "trig2"],
			"message_builder": {"message_type": ".google.protobuf.Empty"}
		}]
	}`

	for _, testCase := range []struct {
		params       string
		wantHttpCode int
	}{
		{params: "?ignore_validation=true", wantHttpCode: http.StatusOK},
		// Defaults to ignore_validation=false when not present.
		{params: "", wantHttpCode: http.StatusBadRequest},
		{params: "?ignore_validation=false", wantHttpCode: http.StatusBadRequest},
		{params: "?ignore_validation=invalidBoolValue", wantHttpCode: http.StatusBadRequest},
	} {
		t.Logf("Testcase: %s", testCase.params)

		w := performPostRequest(router,
			fmt.Sprintf("/api/%s/generate_metrics_config%s", constants.CurrentAPIVersion, testCase.params),
			"application/json", "application/x-protobuf", []byte(invalidPayload))

		if w.Result().StatusCode != testCase.wantHttpCode {
			t.Fatalf("The payload has invalid references to non-existent triggers. This should respect the ignore_validation query param value but got: %d\n%v", w.Result().StatusCode, w.Body.String())
		}
	}
}

func TestGenerateWithExistingInvalidUuidFails(t *testing.T) {
	for _, tc := range getInvalidUuidTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			router, _ := setupServer(ctx, t, false)

			payload := fmt.Sprintf(`{"existing_uuid": %q}`, tc.uuid)
			w := performPostRequest(router,
				fmt.Sprintf("/api/%s/generate_metrics_config", constants.CurrentAPIVersion),
				"application/json", "application/x-protobuf", []byte(payload))

			if want, got := http.StatusBadRequest, w.Result().StatusCode; want != got {
				t.Errorf("w.Result().StatusCode = %d, want: %d", got, want)
			}
			if contentType := w.Result().Header.Get(CONTENT_TYPE_HEADER_NAME); contentType != "application/json; charset=utf-8" {
				t.Errorf("Unexpected Content-Type header, want %q, got %q", "application/json; charset=utf-8", contentType)
				return
			}

			if got := w.Body.String(); !strings.Contains(got, tc.wantErrContaining) {
				t.Errorf("w.Body.String() = %q, want containing %q", got, tc.wantErrContaining)
			}
		})
	}
}

func TestGenerateWithInference(t *testing.T) {
	fixtureTestCurrent(t, string(FileAsBytes(INFER_JSON_FILENAME)), string(FileAsBytes(INFER_TEXTPROTO_FILENAME)))
}

func TestGenerateWithInferenceCustomAggregationMessageType(t *testing.T) {
	fixtureTestCurrent(t, string(FileAsBytes(CUSTOM_AGGREGATION_MESSAGE_TYPE_JSON_FILENAME)), string(FileAsBytes(CUSTOM_AGGREGATION_MESSAGE_TYPE_TEXTPROTO_FILENAME)))
}

// Test for b/378900418.
func TestSourcesAreInferredTopologically(t *testing.T) {
	fixtureTestCurrent(t, string(FileAsBytes(BUG_378900418_JSON_FILENAME)), string(FileAsBytes(BUG_378900418_TEXTPROTO_FILENAME)))
}

// Test for b/380905512.
func TestAdhocDescriptorsUseProto2(t *testing.T) {
	fixtureTestCurrent(t, string(FileAsBytes(BUG_380905512_JSON_FILENAME)), string(FileAsBytes(BUG_380905512_TEXTPROTO_FILENAME)))
}

func TestDataSourceMessageTypes(t *testing.T) {
	fixtureTestCurrent(t, string(FileAsBytes(DATA_SOURCE_MESSAGE_TYPES_JSON_FILENAME)), string(FileAsBytes(DATA_SOURCE_MESSAGE_TYPES_TEXTPROTO_FILENAME)))
}

// Test for source configuration and b/384562190.
func TestDataSourceConfigurationWorks(t *testing.T) {
	fixtureTestCurrent(t, string(FileAsBytes(SOURCE_CONFIGURATION_JSON_FILENAME)), string(FileAsBytes(SOURCE_CONFIGURATION_TEXTPROTO_FILENAME)))
}

func TestConditionalTriggerConfigurationWorks(t *testing.T) {
	fixtureTestCurrent(t, string(FileAsBytes(CONDITIONAL_TRIGGER_JSON_FILENAME)), string(FileAsBytes(CONDITIONAL_TRIGGER_TEXTPROTO_FILENAME)))
}

func TestInitNodeIndex(t *testing.T) {
	fixtureTestCurrent(t, string(FileAsBytes(INIT_NODE_INDEX_JSON_FILENAME)), string(FileAsBytes(INIT_NODE_INDEX_TEXTPROTO_FILENAME)))
}

func TestComprehensiveDescriptorOptimization(t *testing.T) {
	fixtureTestCurrent(t, string(FileAsBytes(COMPREHENSIVE_DESCRIPTOR_OPTIMIZATION_JSON_FILENAME)), string(FileAsBytes(COMPREHENSIVE_DESCRIPTOR_OPTIMIZATION_TEXTPROTO_FILENAME)))
}

func TestRetainAggregationsOnStop(t *testing.T) {
	testCases := []struct {
		name  string
		json  string
		proto string
	}{
		{
			name: "RetainTrue",
			json: `{"retain_aggregations_on_stop": true}`,
			proto: `
			version: 3758096386
			retain_aggregations_on_stop: true
			`,
		},
		{
			name: "RetainFalse",
			json: `{"retain_aggregations_on_stop": false}`,
			proto: `
			version: 3758096386
			retain_aggregations_on_stop: false
			`,
		},
		{
			name: "RetainOmitted",
			json: `{}`, // Should default to false
			proto: `
			version: 3758096386
			retain_aggregations_on_stop: false
			`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fixtureTestCurrent(t, tc.json, tc.proto)
		})
	}
}

func TestAPIVersionCompatibility(t *testing.T) {
	testCases := []struct {
		name       string
		apiVersion constants.APIVersion
		jsonFile   string
	}{
		{
			name:       "V1_Legacy",
			apiVersion: constants.APIVersionV1,
			jsonFile:   API_COMPATIBILITY_LEGACY_JSON_FILENAME,
		},
		{
			name:       "Current_Canonical",
			apiVersion: constants.CurrentAPIVersion,
			jsonFile:   API_COMPATIBILITY_CANONICAL_JSON_FILENAME,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fixtureTest(t, tc.apiVersion, string(FileAsBytes(tc.jsonFile)), string(FileAsBytes(API_COMPATIBILITY_OUTPUT_TEXTPROTO_FILENAME)))
		})
	}
}

func TestSourceReferencingNonExistingSource(t *testing.T) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, false)

	payload := `{
		"aggregators": [{
			"name": "agg_name",
			"reset_on_get": false,
			"trigger_names": [
				"trigger_name"
			],
			"message_builder": {
				"field_assignments": [
					{
						"field_name": "data",
						"aggregation": {
							"@type": "vector",
							"expression": "non_existing.data"
						}
					}
				]
			}
		}]
	}`

	w := performPostRequest(router,
		fmt.Sprintf("/api/%s/generate_metrics_config", constants.CurrentAPIVersion),
		"application/json", "application/x-protobuf", []byte(payload))

	if want, got := http.StatusBadRequest, w.Result().StatusCode; want != got {
		t.Errorf("w.Result().StatusCode = %d, want: %d", got, want)
	}
	if contentType := w.Result().Header.Get(CONTENT_TYPE_HEADER_NAME); contentType != "application/json; charset=utf-8" {
		t.Errorf("Unexpected Content-Type header, want %q, got %q", "application/json; charset=utf-8", contentType)
		return
	}

	if want, got := `Source \"non_existing\" does not exist.`, w.Body.String(); !strings.Contains(got, want) {
		t.Errorf("w.Body.String() = %q, want containing %q", got, want)
	}
}

func TestGetFileDescriptor(t *testing.T) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, false)

	fds_want_bytes, err := os.ReadFile(EIPF_B_TEXTPROTO_FDS_FILENAME)
	if err != nil {
		t.Errorf("Error reading file: %v", err)
	}

	fds_want := &descriptorpb.FileDescriptorSet{}
	prototext.Unmarshal(fds_want_bytes, fds_want)

	textProtoAsBytes := []byte(FileAsBytes(EIPF_B_TEXTPROTO_FILENAME))

	w := performPostRequest(router,
		fmt.Sprintf("/api/%s/get_file_descriptor_set", constants.CurrentAPIVersion),
		mcg.CONTENT_TYPE_TEXT_X_PROTOBUF, "text/x-protobuf", textProtoAsBytes)
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("Failed to extract FileDescriptors from MetricsConfig: %d\n%v", w.Result().StatusCode, w.Body.String())
	}
	if contentType := w.Result().Header.Get(CONTENT_TYPE_HEADER_NAME); contentType != mcg.ContentTypeTextProtoMetricsConfig {
		t.Errorf("Unexpected Content-Type header, want %q, got %q", mcg.ContentTypeTextProtoMetricsConfig, contentType)
		return
	}

	body, err := io.ReadAll(w.Body)

	fds_got := &descriptorpb.FileDescriptorSet{}
	prototext.Unmarshal(body, fds_got)

	if diff := cmp.Diff(fds_want.File, fds_got.File, protocmp.Transform()); diff != "" {
		t.Errorf("Unexpected difference in FileDescriptorSet (-want +got):\n%s", diff)
	}
}

func TestInferenceFailsForVectorOfVector(t *testing.T) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, false)

	payload := `{
		"triggers": [{
			"name": "t1",
			"periodic": {"period_ms": 1000}
		}],
		"aggregators": [{
			"name": "vector_source",
			"trigger_names": ["t1"],
			"message_builder": {
				"field_assignments": [{
					"field_name": "values",
					"aggregation": {
						"@type": "vector",
						"expression": "1"
					}
				}]
			}
		}, {
			"name": "vector_of_vector_source",
			"trigger_names": ["t1"],
			"message_builder": {
				"field_assignments": [{
					"field_name": "values",
					"aggregation": {
						"@type": "vector",
						"expression": "vector_source.values"
					}
				}]
			}
		}]
	}`

	w := performPostRequest(router,
		fmt.Sprintf("/api/%s/generate_metrics_config", constants.CurrentAPIVersion),
		"application/json", "application/x-protobuf", []byte(payload))

	log.Println(w.Result().StatusCode)

	if want, got := http.StatusBadRequest, w.Result().StatusCode; want != got {
		t.Errorf("w.Result().StatusCode = %d, want: %d", got, want)
	}
	if contentType := w.Result().Header.Get(CONTENT_TYPE_HEADER_NAME); contentType != "application/json; charset=utf-8" {
		t.Errorf("Unexpected Content-Type header, want %q, got %q", "application/json; charset=utf-8", contentType)
		return
	}

	if want, got := "Invalid vector aggregation on type REPEATED", w.Body.String(); !strings.Contains(got, want) {
		t.Errorf("w.Body.String() = %q, want containing %q", got, want)
	}
}

func loadEipfBAsLegacyTextproto(t *testing.T) []byte {
	t.Helper()
	textproto := FileAsBytes(EIPF_B_TEXTPROTO_FILENAME)
	var mc pb.MetricsConfig
	if err := prototext.Unmarshal(textproto, &mc); err != nil {
		t.Fatalf("prototext.Unmarshal failed: %v", err)
	}

	mc.SetUuid(uuid.New().String())

	legacyTextproto, err := mcg.TextprotoMarshal(&mc, true)
	if err != nil {
		t.Fatalf("mcg.TextprotoMarshal failed: %v", err)
	}
	return legacyTextproto
}

func TestV1EndpointsAcceptLegacyFormat(t *testing.T) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, false)

	t.Run("validate_v1_textproto", func(t *testing.T) {
		legacyTextproto := loadEipfBAsLegacyTextproto(t)
		w := performPostRequest(router, "/api/v1/validate_metrics_config", mcg.CONTENT_TYPE_TEXT_X_PROTOBUF, "", legacyTextproto)

		if w.Code != http.StatusOK {
			t.Errorf("POST /api/v1/validate_metrics_config failed: %d %s", w.Code, w.Body.String())
		}
	})

	t.Run("generate_v1_json", func(t *testing.T) {
		deprecatedPayload := FileAsBytes(API_COMPATIBILITY_LEGACY_JSON_FILENAME)
		w := performPostRequest(router, "/api/v1/generate_metrics_config", "application/json", "text/x-protobuf", deprecatedPayload)

		if w.Code != http.StatusOK {
			t.Errorf("POST /api/v1/generate_metrics_config failed: %d %s", w.Code, w.Body.String())
		}

		// Ensure the output is also in the legacy format if it's V1
		if !strings.Contains(w.Body.String(), "publishers") {
			t.Errorf("Response body does not contain legacy 'publishers' field: %s", w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "end_trigger_name") {
			t.Errorf("Response body does not contain legacy 'end_trigger_name' field: %s", w.Body.String())
		}
	})

	t.Run("get_file_descriptor_set_v1", func(t *testing.T) {
		legacyTextproto := loadEipfBAsLegacyTextproto(t)
		w := performPostRequest(router, "/api/v1/get_file_descriptor_set", mcg.CONTENT_TYPE_TEXT_X_PROTOBUF, "text/x-protobuf", legacyTextproto)

		if w.Code != http.StatusOK {
			t.Errorf("POST /api/v1/get_file_descriptor_set failed: %d %s", w.Code, w.Body.String())
		}

		// Verify we got a FileDescriptorSet back
		var fds descriptorpb.FileDescriptorSet
		if err := prototext.Unmarshal(w.Body.Bytes(), &fds); err != nil {
			t.Errorf("Failed to unmarshal response into FileDescriptorSet: %v", err)
		}
		if len(fds.File) == 0 {
			t.Error("Response FileDescriptorSet has no files")
		}
	})
}

func TestAPIVersionSchemaEnforcement(t *testing.T) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, false)

	deprecatedPayload := string(FileAsBytes(API_COMPATIBILITY_LEGACY_JSON_FILENAME))
	canonicalPayload := string(FileAsBytes(API_COMPATIBILITY_CANONICAL_JSON_FILENAME))

	testCases := []struct {
		name       string
		apiVersion constants.APIVersion
		payload    string
		wantCode   int
	}{
		{
			name:       "v1_legacy_ok",
			apiVersion: constants.APIVersionV1,
			payload:    deprecatedPayload,
			wantCode:   http.StatusOK,
		},
		{
			name:       "v1_canonical_fail",
			apiVersion: constants.APIVersionV1,
			payload:    canonicalPayload,
			wantCode:   http.StatusBadRequest,
		},
		{
			name:       "v2_legacy_fail",
			apiVersion: constants.APIVersionV2,
			payload:    deprecatedPayload,
			wantCode:   http.StatusBadRequest,
		},
		{
			name:       "v2_canonical_ok",
			apiVersion: constants.APIVersionV2,
			payload:    canonicalPayload,
			wantCode:   http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := performPostRequest(router,
				fmt.Sprintf("/api/%s/generate_metrics_config", tc.apiVersion),
				"application/json", "application/x-protobuf", []byte(tc.payload))

			if got, want := w.Code, tc.wantCode; got != want {
				t.Errorf("StatusCode = %d, want %d. Body: %s", got, want, w.Body.String())
			}
		})
	}
}

func TestNoMessageInference(t *testing.T) {
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("my_custom_descriptor.proto"),
		Package: proto.String("my.pkg"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("MyMessage")},
		},
	}
	fdSet := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fd},
	}
	bytes, err := proto.Marshal(fdSet)
	if err != nil {
		t.Fatalf("Failed to marshal fdSet: %v", err)
	}
	jsonInput := fmt.Sprintf(`{
		"descriptor_protos": ["%s"]
	}`, base64.StdEncoding.EncodeToString(bytes))

	expectedTextproto := `
		descriptor_protos {
			name: "my_custom_descriptor.proto"
			package: "my.pkg"
			message_type {
				name: "MyMessage"
			}
			syntax: "proto3"
		}
	`

	ctx := context.Background()
	router, _ := setupServer(ctx, t, false)
	body := strings.NewReader(jsonInput)
	req, _ := http.NewRequest(http.MethodPost, "/api/v2/generate_metrics_config?no_inference=true", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/x-protobuf; messageType=google.sdv.telemetry.MetricsConfig")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failure response: %d\n\t%s", w.Code, w.Body.String())
	}

	var result, expected pb.MetricsConfig
	if err := proto.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Errorf("proto.Unmarshal failed: %v", err)
	}

	unmarshalOptions := prototext.UnmarshalOptions{}
	if err := unmarshalOptions.Unmarshal([]byte(expectedTextproto), &expected); err != nil {
		t.Fatalf("prototext.Unmarshal failed: %v", err)
	}

	opts := []cmp.Option{
		protocmp.IgnoreFields(&pb.MetricsConfig{}, protoreflect.Name("version")),
		protocmp.IgnoreFields(&pb.MetricsConfig{}, protoreflect.Name("expression_nodes")),
	}

	testhelper.AssertMetricsConfigEqual(t, &expected, &result, opts...)
}
