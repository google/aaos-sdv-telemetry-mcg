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

package mcg_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"sdv.googlesource.com/mcg/mcg/constants"
)

const (
	jsonInferVsFile    = "testdata/infer_vs.json"
	validVsVersionFile = "testdata/vsignal_api/sample_ssot_ver1.json"
)

func TestGenerateWithVersionNoCache(t *testing.T) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, false)

	w := sendGenerateRequest(t, router, jsonInferVsFile)

	if want, got := http.StatusBadRequest, w.Result().StatusCode; want != got {
		t.Errorf("w.Result().StatusCode = %d, want: %d", got, want)
	}

	if want, got := `No cache enabled.`, w.Body.String(); !strings.Contains(got, want) {
		t.Errorf("w.Body.String() = %q, want containing %q", got, want)
	}
}

func TestGenerateWithMissingVsVersion(t *testing.T) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, true)

	w := sendGenerateRequest(t, router, jsonInferVsFile)

	if want, got := http.StatusBadRequest, w.Result().StatusCode; want != got {
		t.Errorf("w.Result().StatusCode = %d, want: %d", got, want)
	}

	if want, got := `Failed to lookup vehicle signal version`, w.Body.String(); !strings.Contains(got, want) {
		t.Errorf("w.Body.String() = %q, want containing %q", got, want)
	}
}

func TestGenerateWithVsVersion(t *testing.T) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, true)

	w := sendVsAddRequest(t, router, validVsVersionFile)

	if want, got := http.StatusOK, w.Result().StatusCode; want != got {
		t.Errorf("w.Result().StatusCode = %d, want: %d", got, want)
	}

	w = sendGenerateRequest(t, router, jsonInferVsFile)

	if want, got := http.StatusOK, w.Result().StatusCode; want != got {
		t.Errorf("w.Result().StatusCode = %d, want: %d", got, want)
	}

	if want, got := "name: \"mcg/testdata/vehicle_signals_sample/subpkg/sample_speed.proto\"", w.Body.String(); !strings.Contains(got, want) {
		t.Errorf("w.Body.String() = %q, want containing %q", got, want)
	}
}

func sendGenerateRequest(tb testing.TB, router *gin.Engine, file string) *httptest.ResponseRecorder {
	tb.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/api/%s/generate_metrics_config", constants.CurrentAPIVersion),
		strings.NewReader(string(FileAsBytes(tb, file))))
	req.Header.Set("Accept", "text/x-protobuf")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}

func sendVsAddRequest(tb testing.TB, router *gin.Engine, file string) *httptest.ResponseRecorder {
	tb.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/api/%s/vs/", constants.CurrentAPIVersion),
		strings.NewReader(string(FileAsBytes(tb, file))))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}
