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
	"encoding/json"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"google.golang.org/genproto/googleapis/rpc/code"

	"sdv.googlesource.com/mcg/mcg"
	"sdv.googlesource.com/mcg/mcg/docs"
	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
	"sdv.googlesource.com/mcg/mcg/vs_cache"
	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

func setupServer(ctx context.Context, t *testing.T, withLocCache bool) (*gin.Engine, *mcg.Server) {
	t.Helper()
	router := gin.New()
	if withLocCache {
		config := vs_cache.CacheConfig{
			EnableLocalCache: true,
		}
		cache, closeCache, err := vs_cache.NewCache(ctx, config)
		if err != nil {
			t.Fatalf("vs_cache.NewCache(%v, %v) failed: %v", ctx, config, err)
		}
		// Given that this is just test code, instead of returning `closeCache`,
		// we simply close the cache once the provided `ctx` closes.
		context.AfterFunc(ctx, closeCache)

		router.Use(vs_cache.CacheMiddleware(cache))
	}
	router.Use(mcgerrors.MiddlewareRenderErrors)
	if err := docs.InstallRoutes("/", router); err != nil {
		t.Fatalf("docs.InstallRoutes failed: %v", err)
	}
	server := &mcg.Server{}
	server.InstallRoutes(router)
	return router, server
}

func assertErrorResponse(t *testing.T, w *httptest.ResponseRecorder, expectHTTPStatus int, expectStatusCode code.Code, checkMessage func(*testing.T, string)) mcgerrors.JsonErrorResponse {
	resp := w.Result()
	if resp.StatusCode != expectHTTPStatus {
		t.Errorf("wrong http status: want %d, got %d", expectHTTPStatus, resp.StatusCode)
	}
	const expectContentType = "application/json"
	gotContentType := resp.Header.Get("Content-Type")
	mediatype, _, err := mime.ParseMediaType(gotContentType)
	if err != nil {
		t.Fatalf("Invalid Content-Type: %q %v", gotContentType, err)
	}
	if mediatype != expectContentType {
		t.Errorf("wrong content type: want %s, got %s", expectContentType, gotContentType)
	}
	var errResp mcgerrors.JsonErrorResponse
	if err = json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatal("response is not JSON:", err)
	}
	if errResp.Error.Code == 0 {
		var actualJSON map[string]any
		if err = json.Unmarshal(w.Body.Bytes(), &actualJSON); err != nil {
			t.Fatal(err)
		}
		t.Errorf("json response is not an application error: %+v", actualJSON)
		return errResp
	}
	if errResp.Error.Code != int32(resp.StatusCode) {
		t.Errorf("mismatch http status: body %d, header %d", errResp.Error.Code, resp.StatusCode)
	}
	assertStatusErrorResponse(t, &errResp, expectHTTPStatus, expectStatusCode, checkMessage)
	return errResp
}

func assertStatusErrorResponse(t *testing.T, errResp *mcgerrors.JsonErrorResponse, expectHTTPStatus int, expectStatusCode code.Code, checkMessage func(*testing.T, string)) {
	if errResp == nil {
		t.Errorf("expected a %s error, got OK", expectStatusCode.String())
		return
	}
	if errResp.Error.Code != int32(expectHTTPStatus) {
		t.Errorf("wrong http status: want %d, got %d", expectHTTPStatus, errResp.Error.Code)
	}
	if errResp.Error.Status != expectStatusCode.String() {
		parsedGot, ok := code.Code_value[errResp.Error.Status]
		if !ok {
			parsedGot = -1
		}
		t.Errorf("wrong canonical status: want %s(%d) got %q(%d)", expectStatusCode, expectStatusCode, errResp.Error.Status, parsedGot)
	}
	if checkMessage != nil {
		checkMessage(t, errResp.Error.Message)
	}
}

func extractFieldAssignment(pmb *pb.ProtoMessageBuilder, fieldName string) *pb.ProtoMessageBuilder_FieldAssignment {
	for _, v := range pmb.GetFieldAssignments() {
		if v.GetFieldName() == fieldName {
			return v
		}
	}
	return nil
}

func helperGeneralGet(t *testing.T, path string, headers http.Header, router *gin.Engine) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header[k] = v
	}
	router.ServeHTTP(w, req)
	return w
}

func FileAsBytes(tb testing.TB, filename string) []byte {
	tb.Helper()
	b, err := os.ReadFile(filename)
	if err != nil {
		tb.Fatalf("failed to read file %q: %v", filename, err)
	}
	return b
}
