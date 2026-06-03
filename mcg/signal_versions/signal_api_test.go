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

package signal_versions_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/go-cmp/cmp"

	"sdv.googlesource.com/mcg/mcg/constants"
	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
	"sdv.googlesource.com/mcg/mcg/signal_versions"
	"sdv.googlesource.com/mcg/mcg/vs_cache"
)

const VALID_VS_VERSION = "../testdata/vsignal_api/sample_ssot_ver1.json"

func TestLocalCache(t *testing.T) {
	ctx := context.Background()
	config := vs_cache.CacheConfig{
		EnableLocalCache: true,
		EnableRedCache:   false,
	}
	cache, closeCache, err := vs_cache.NewCache(ctx, config)
	if err != nil {
		t.Fatalf("vs_cache.NewCache(%v, %v) failed: %v", ctx, config, err)
	}
	defer closeCache()

	router := setupGinAndRoutes(cache)

	w := sendAddRequest(t, router, VALID_VS_VERSION)
	if got, want := w.Code, http.StatusOK; got != want {
		t.Errorf("POST api/%s/vs/ Status Code = %v, Want: %v", constants.CurrentAPIVersion, got, want)
	}
	if err := checkAddResponse(w.Body.Bytes(), &signal_versions.VSCatalogVersionMetadata{Id: "1"}); err != nil {
		t.Error(err)
	}

	w = sendListRequest(t, router)
	if got, want := w.Code, http.StatusOK; got != want {
		t.Errorf("GET api/%s/vs/ Status Code = %v, Want: %v", constants.CurrentAPIVersion, got, want)
	}
	want := &signal_versions.VSCatalogVersionsMetadata{
		Items: []signal_versions.VSCatalogVersionMetadata{
			{Id: "1"},
		},
	}
	if err := checkListResponse(w.Body.Bytes(), want); err != nil {
		t.Error(err)
	}

	w = sendDelRequest(t, router, "1")
	if got, want := w.Code, http.StatusOK; got != want {
		t.Errorf("Delete api/%s/vs/1 Status Code = %v, Want: %v", constants.CurrentAPIVersion, got, want)
	}

	w = sendListRequest(t, router)

	if got, want := w.Code, http.StatusOK; got != want {
		t.Errorf("GET api/%s/vs/ Status Code = %v, Want: %v", constants.CurrentAPIVersion, got, want)
	}

	want = &signal_versions.VSCatalogVersionsMetadata{
		Items: []signal_versions.VSCatalogVersionMetadata{},
	}
	if err := checkListResponse(w.Body.Bytes(), want); err != nil {
		t.Error(err)
	}
}

func TestNoCacheAdd(t *testing.T) {
	ctx := context.Background()
	config := vs_cache.CacheConfig{
		EnableLocalCache: false,
		EnableRedCache:   false,
	}
	cache, closeCache, err := vs_cache.NewCache(ctx, config)
	if err != nil {
		t.Fatalf("vs_cache.NewCache(%v, %v) failed: %v", ctx, config, err)
	}
	defer closeCache()

	router := setupGinAndRoutes(cache)

	w := sendAddRequest(t, router, VALID_VS_VERSION)
	if got, want := w.Code, http.StatusBadRequest; got != want {
		t.Errorf("POST api/%s/vs/ Status Code = %v, Want: %v", constants.CurrentAPIVersion, got, want)
	}
}

func TestNoCacheDelete(t *testing.T) {
	ctx := context.Background()
	config := vs_cache.CacheConfig{
		EnableLocalCache: false,
		EnableRedCache:   false,
	}
	cache, closeCache, err := vs_cache.NewCache(ctx, config)
	if err != nil {
		t.Fatalf("vs_cache.NewCache(%v, %v) failed: %v", ctx, config, err)
	}
	defer closeCache()

	router := setupGinAndRoutes(cache)

	w := sendDelRequest(t, router, "1")
	if got, want := w.Code, http.StatusBadRequest; got != want {
		t.Errorf("Delete api/%s/vs/1 Status Code = %v, Want: %v", constants.CurrentAPIVersion, got, want)
	}
}

func TestNoCacheList(t *testing.T) {
	ctx := context.Background()
	config := vs_cache.CacheConfig{
		EnableLocalCache: false,
		EnableRedCache:   false,
	}
	cache, closeCache, err := vs_cache.NewCache(ctx, config)
	if err != nil {
		t.Fatalf("vs_cache.NewCache(%v, %v) failed: %v", ctx, config, err)
	}
	defer closeCache()

	router := setupGinAndRoutes(cache)
	w := sendListRequest(t, router)
	if got, want := w.Code, http.StatusBadRequest; got != want {
		t.Errorf("GET api/%s/vs/ Status Code = %v, Want: %v", constants.CurrentAPIVersion, got, want)
	}
}

func sendDelRequest(t *testing.T, router *gin.Engine, id string) *httptest.ResponseRecorder {
	delReq, err := createDelRequest(id)
	if err != nil {
		t.Error(err)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, delReq)
	return w
}

func checkAddResponse(body []byte, want *signal_versions.VSCatalogVersionMetadata) error {
	var got signal_versions.VSCatalogVersionMetadata
	if err := json.Unmarshal(body, &got); err != nil {
		return fmt.Errorf("json.Unmarshal(%q, _) = %v, want nil", string(body), err)
	}
	if diff := cmp.Diff(&got, want); diff != "" {
		return fmt.Errorf("signal_versions.HandleSignalAdd unexpected results (-want +got):\n%s", diff)
	}
	return nil
}

func checkListResponse(body []byte, want *signal_versions.VSCatalogVersionsMetadata) error {
	var got signal_versions.VSCatalogVersionsMetadata
	if err := json.Unmarshal(body, &got); err != nil {
		return fmt.Errorf("json.Unmarshal(%q, _) = %v, want nil", string(body), err)
	}
	if diff := cmp.Diff(&got, want); diff != "" {
		return fmt.Errorf("signal_versions.HandleSignalList unexpected results (-want +got):\n%s", diff)
	}
	return nil
}

func sendListRequest(t *testing.T, router *gin.Engine) *httptest.ResponseRecorder {
	listReq, err := createListRequest()
	if err != nil {
		t.Error(err)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, listReq)
	return w
}

func sendAddRequest(t *testing.T, router *gin.Engine, fName string) *httptest.ResponseRecorder {
	reqBytes, err := os.ReadFile(VALID_VS_VERSION)
	if err != nil {
		t.Error(err)
	}

	addReq, err := createAddRequest(reqBytes)
	if err != nil {
		t.Error(err)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, addReq)

	return w
}

func createAddRequest(reqBytes []byte) (*http.Request, error) {
	path := fmt.Sprintf("/api/%s/vs/", constants.CurrentAPIVersion)
	req, err := http.NewRequest(http.MethodPost, path, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func createListRequest() (*http.Request, error) {
	path := fmt.Sprintf("/api/%s/vs/", constants.CurrentAPIVersion)
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func createDelRequest(id string) (*http.Request, error) {
	path := fmt.Sprintf("/api/%s/vs/%s", constants.CurrentAPIVersion, id)
	req, err := http.NewRequest(http.MethodDelete, path, nil)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func setupGinAndRoutes(cache *vs_cache.Cache) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.Use(mcgerrors.MiddlewareRenderErrors)
	router.Use(vs_cache.CacheMiddleware(cache))

	rGroup := router.Group(fmt.Sprintf("/api/%s", constants.CurrentAPIVersion))
	signal_versions.InstallRoutes(rGroup)
	return router
}
