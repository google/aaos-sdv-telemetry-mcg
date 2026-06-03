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
	"context"
	"mime"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"sdv.googlesource.com/mcg/mcg"
	"sdv.googlesource.com/mcg/mcg/constants"
	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

func TestGetSortedStringKeys(t *testing.T) {
	str := "value"
	strmap := map[string]*string{
		"c": &str,
		"b": &str,
		"a": &str,
	}

	want := []string{"a", "b", "c"}
	got := mcg.GetSortedStringKeys[string](strmap)
	for i, val := range want {
		if val != got[i] {
			t.Errorf("not alphabetical, want %s got %s at index %d", val, got[i], i)
		}
	}
}

func TestTextprotoMarshal(t *testing.T) {
	configuration, err := anypb.New(wrapperspb.Int32(42))
	if err != nil {
		t.Fatalf("anypb.New(wrapperspb.Int32(42)) = _, %v, want _, nil", err)
	}

	mc := pb.MetricsConfig_builder{
		Sources: []*pb.Source{pb.Source_builder{
			Name: "SourceName",
			DataSource: pb.DataSource_builder{
				SourceIdentifier: `ServiceNameWithSpecialCharacters"'`,
				Configuration:    configuration,
			}.Build(),
		}.Build()},
		ExpressionNodes: []*pb.Node{
			pb.Node_builder{
				ConstantLeafNode: pb.ConstantLeafNode_builder{
					BoolValue: proto.Bool(true),
				}.Build(),
			}.Build(),
			pb.Node_builder{
				ConstantLeafNode: pb.ConstantLeafNode_builder{
					BoolValue: proto.Bool(false),
				}.Build(),
			}.Build(),
		},
	}.Build()

	got, err := mcg.TextprotoMarshal(mc, false)
	if err != nil {
		t.Fatalf("mcg.TextprotoMarshal(proto) = error %v, want no error", err)
	}

	want := []byte(`
sources {
  name: "SourceName"
  data_source {
    source_identifier: "ServiceNameWithSpecialCharacters\"'"
    configuration {
      type_url: "type.googleapis.com/google.protobuf.Int32Value"
      value: "\x08*"
    }
  }
}
# Expression Node 0
expression_nodes {
  constant_leaf_node {
    bool_value: true
  }
}
# Expression Node 1
expression_nodes {
  constant_leaf_node {
    bool_value: false
  }
}
`[1:])

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("w.Body.String() returned unexpected diff (-want +got):\n%s", diff)
	}
}

func TestChooseRenderFunc(t *testing.T) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, false)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/some-url", nil)
	req.Header.Set("Accept", mcg.CONTENT_TYPE_TEXT_X_PROTOBUF)

	c := gin.CreateTestContextOnly(w, router)
	c.Request = req

	f, err := mcg.ChooseRenderFunc(c, constants.APIVersionV2)
	if err != nil {
		t.Fatalf("mcg.ChooseRenderFunc(c) = error %v, want no error", err)
	}

	mc := pb.MetricsConfig_builder{
		Sources: []*pb.Source{pb.Source_builder{
			Name: "SourceName",
			DataSource: pb.DataSource_builder{
				SourceIdentifier: `ServiceNameWithSpecialCharacters"'`,
			}.Build(),
		}.Build()},
	}.Build()

	f(c, mc)
	got := w.Body.String()
	want := `
sources {
  name: "SourceName"
  data_source {
    source_identifier: "ServiceNameWithSpecialCharacters\"'"
  }
}
`[1:]

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("w.Body.String() returned unexpected diff (-want +got):\n%s", diff)
	}
}

func TestTextprotoMarshalLegacy(t *testing.T) {
	input := FileAsBytes("testdata/v2_canonical_fixture.textproto")
	var mc pb.MetricsConfig
	if err := prototext.Unmarshal(input, &mc); err != nil {
		t.Fatalf("prototext.Unmarshal(input) = %v, want nil", err)
	}

	got, err := mcg.TextprotoMarshal(&mc, true)
	if err != nil {
		t.Fatalf("mcg.TextprotoMarshal(mc, true) = %v, want nil", err)
	}

	want := FileAsBytes("testdata/v1_legacy_fixture.textproto")

	if diff := cmp.Diff(string(want), string(got)); diff != "" {
		t.Errorf("mcg.TextprotoMarshal(mc, true) mismatch (-want +got):\n%s", diff)
	}
}

func TestTextprotoUnmarshalLegacy(t *testing.T) {
	input := FileAsBytes("testdata/v1_legacy_fixture.textproto")

	mc, err := mcg.ParseMetricsConfig(input, mcg.CONTENT_TYPE_TEXT_X_PROTOBUF, constants.APIVersionV1)
	if err != nil {
		t.Fatalf("ParseMetricsConfig(input, V1) failed: %v", err)
	}

	wantInput := FileAsBytes("testdata/v2_canonical_fixture.textproto")
	var want pb.MetricsConfig
	if err := prototext.Unmarshal(wantInput, &want); err != nil {
		t.Fatalf("prototext.Unmarshal(wantInput) failed: %v", err)
	}

	if diff := cmp.Diff(&want, mc, protocmp.Transform()); diff != "" {
		t.Errorf("ParseMetricsConfig(input, V1) result mismatch (-want +got):\n%s", diff)
	}
}

func TestContentTypeConstants(t *testing.T) {
	metricsConfigProtoFullName := (&pb.MetricsConfig{}).ProtoReflect().Descriptor().FullName()

	tests := []struct {
		name          string
		contentType   string
		wantMediaType string
	}{
		{
			name:          "binary",
			contentType:   mcg.ContentTypeBinaryProtoMetricsConfig,
			wantMediaType: "application/x-protobuf",
		},
		{
			name:          "text",
			contentType:   mcg.ContentTypeTextProtoMetricsConfig,
			wantMediaType: "text/x-protobuf",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mediatype, params, err := mime.ParseMediaType(test.contentType)
			if err != nil {
				t.Fatalf("mime.ParseMediaType(%q) failed: %v", test.contentType, err)
			}
			if want, got := test.wantMediaType, mediatype; want != got {
				t.Errorf("mediatype = %q, want %q", got, want)
			}

			protoName, ok := params["proto"]
			if !ok {
				t.Fatalf("Key %q not found in params", "proto")
			}
			if want, got := metricsConfigProtoFullName, protoName; want != protoreflect.FullName(got) {
				t.Errorf("protoName = %q, want %q", got, want)
			}
		})
	}
}

func TestParseMetricsConfigValidation(t *testing.T) {
	legacyInput := FileAsBytes("testdata/v1_legacy_fixture.textproto")
	canonicalInput := FileAsBytes("testdata/v2_canonical_fixture.textproto")
	mixedInput := FileAsBytes("testdata/mixed_v1_v2_fixture.textproto")

	tests := []struct {
		name       string
		input      []byte
		apiVersion constants.APIVersion
		wantErr    bool
	}{
		{
			name:       "V1 with Legacy Input",
			input:      legacyInput,
			apiVersion: constants.APIVersionV1,
			wantErr:    false,
		},
		{
			name:       "V1 with Canonical Input",
			input:      canonicalInput,
			apiVersion: constants.APIVersionV1,
			wantErr:    true,
		},
		{
			name:       "V1 with Mixed Input",
			input:      mixedInput,
			apiVersion: constants.APIVersionV1,
			wantErr:    true,
		},
		{
			name:       "V2 with Canonical Input",
			input:      canonicalInput,
			apiVersion: constants.APIVersionV2,
			wantErr:    false,
		},
		{
			name:       "V2 with Legacy Input",
			input:      legacyInput,
			apiVersion: constants.APIVersionV2,
			wantErr:    true,
		},
		{
			name:       "V2 with Mixed Input",
			input:      mixedInput,
			apiVersion: constants.APIVersionV2,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mcg.ParseMetricsConfig(tt.input, mcg.CONTENT_TYPE_TEXT_X_PROTOBUF, tt.apiVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMetricsConfig() got error = %v, want error? %v", err, tt.wantErr)
			}
		})
	}
}
