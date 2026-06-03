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

package mcg_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSwaggerUIIsServed(t *testing.T) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, false)

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/docs/swagger-ui/", nil)
	router.ServeHTTP(w, req)

	if want, got := http.StatusOK, w.Result().StatusCode; want != got {
		t.Errorf("w.Result().StatusCode = %d, want: %d", got, want)
	}
	if contentType := w.Result().Header.Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want: %q", contentType, "text/html; charset=utf-8")
	}

	if body := w.Body.String(); !strings.Contains(body, "<title>Swagger UI</title>") {
		t.Errorf("body does not contain <title>Swagger UI</title>")
	}
}

func TestDocsRedirect(t *testing.T) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, false)

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/docs/", nil)
	router.ServeHTTP(w, req)

	if want, got := http.StatusTemporaryRedirect, w.Result().StatusCode; want != got {
		t.Errorf("w.Result().StatusCode = %d, want: %d", got, want)
	}
	if location := w.Result().Header.Get("Location"); location != "/docs/swagger-ui" {
		t.Errorf("Location = %q, want: %q", location, "/docs/swagger-ui")
	}
}

func TestOpenApiSpec(t *testing.T) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, false)

	t.Run("yaml", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/docs/openapi.yaml", nil)
		router.ServeHTTP(w, req)

		body := w.Body.Bytes()

		var config map[string]any
		if err := yaml.Unmarshal(body, &config); err != nil {
			t.Fatalf("yaml.Unmarshal(%q, _) failed: %v", string(body), err)
		}
	})

	t.Run("json", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/docs/openapi.json", nil)
		router.ServeHTTP(w, req)

		body := w.Body.Bytes()

		var config map[string]any
		if err := json.Unmarshal(body, &config); err != nil {
			t.Fatalf("json.Unmarshal(%q, _) failed: %v", string(body), err)
		}
	})
}
