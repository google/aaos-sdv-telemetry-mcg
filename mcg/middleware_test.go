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
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMiddleware304(t *testing.T) {
	ctx := context.Background()
	router, _ := setupServer(ctx, t, false)

	// Simulate a route that sends `ETag` and `Last-Modified` headers, and
	// produces a 304 response if appropriate.
	router.GET("/produce-304", func(ctx *gin.Context) {
		ctx.Header("ETag", "abc123")
		ctx.Header("Last-Modified", "Wed, 21 Oct 2015 07:28:00 GMT")

		if ctx.Request.Header.Get("If-None-Match") == "abc123" && ctx.Request.Header.Get("If-Modified-Since") == "Wed, 21 Oct 2015 07:28:00 GMT" {
			ctx.Status(http.StatusNotModified)
		} else {
			ctx.Status(http.StatusOK)
		}
	})

	firstResp := helperGeneralGet(t, "/produce-304", nil, router)
	etag := firstResp.Result().Header.Get("ETag")
	date := firstResp.Result().Header.Get("Last-Modified")
	if etag == "" {
		t.Logf("response: %v", firstResp.Result())
		t.Fatal("no etag for asset?")
	}

	hdrs := http.Header{}
	hdrs.Set("If-None-Match", etag)
	hdrs.Set("If-Modified-Since", date)
	secondResp := helperGeneralGet(t, "/produce-304", hdrs, router)
	if want, got := http.StatusNotModified, secondResp.Result().StatusCode; want != got {
		t.Logf("response: %v", secondResp.Result())
		t.Errorf("wrong http status code: want %d, got %d", want, got)
	}
	if got := secondResp.Body.Len(); got != 0 {
		t.Logf("response: %v", secondResp.Result())
		t.Errorf("wrong 304 content length, want 0 got %v", got)
	}
}
