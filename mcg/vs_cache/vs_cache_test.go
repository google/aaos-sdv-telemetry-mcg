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

package vs_cache_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/reflect/protoreflect"

	"sdv.googlesource.com/mcg/mcg/testhelper"
	"sdv.googlesource.com/mcg/mcg/vs_cache"
)

const (
	VALID_VS_VERSION                              = "../testdata/vsignal_api/sample_ssot_ver1.json"
	SPEED_REPORT_MSG_FNAME  protoreflect.FullName = "mcg.test.report.speed_report"
	VEHICLE_SPEED_MSG_FNAME                       = "android.sdv.telemetry.mcg.testdata.VehicleSpeed"
)

type VehicleSignalRequest struct {
	Version        string `json:"version"`
	VehicleSignals []byte `json:"vehicle_signals"`
}

func TestLocalCache(t *testing.T) {
	ctx := context.Background()
	config := vs_cache.CacheConfig{
		EnableLocalCache: true,
	}
	cache, closeCache, err := vs_cache.NewCache(ctx, config)
	if err != nil {
		t.Fatalf("vs_cache.NewCache(%v, %v) failed: %v", ctx, config, err)
	}
	defer closeCache()

	var vsReq VehicleSignalRequest
	if err := testhelper.LoadJSON(VALID_VS_VERSION, &vsReq); err != nil {
		t.Fatalf("testhelper.LoadJSON(%q, &vsReq) = %v, want nil", VALID_VS_VERSION, err)
	}

	if err := cache.Set(ctx, vsReq.Version, vsReq.VehicleSignals); err != nil {
		t.Error(err)
	}

	res, err := cache.Get(ctx, vsReq.Version)
	if err != nil {
		t.Error(err)
	}

	resMsg, err := res.FindMessageByName(SPEED_REPORT_MSG_FNAME)
	if err != nil {
		t.Error(err)
	}

	if want, got := SPEED_REPORT_MSG_FNAME, resMsg.Descriptor().FullName(); want != got {
		t.Errorf("Retrieved Cache Message Malformed: %s != %s", want, got)
	}
}

func TestLocalCacheDelete(t *testing.T) {
	ctx := context.Background()
	config := vs_cache.CacheConfig{
		EnableLocalCache: true,
	}
	cache, closeCache, err := vs_cache.NewCache(ctx, config)
	if err != nil {
		t.Fatalf("vs_cache.NewCache(%v, %v) failed: %v", ctx, config, err)
	}
	defer closeCache()

	var vsReq VehicleSignalRequest
	if err := testhelper.LoadJSON(VALID_VS_VERSION, &vsReq); err != nil {
		t.Fatalf("testhelper.LoadJSON(%q, &vsReq) = %v, want nil", VALID_VS_VERSION, err)
	}

	deleted, err := cache.Delete(ctx, vsReq.Version)
	if err != nil || deleted {
		t.Errorf("cache.Delete(ctx, %q) = (%v, %v), want (false, nil)", vsReq.Version, deleted, err)
	}

	err = cache.Set(ctx, vsReq.Version, vsReq.VehicleSignals)
	if err != nil {
		t.Errorf("cache.Set(ctx, %q, vsReq.VehicleSignals) = %v, want nil", vsReq.Version, err)
	}

	deleted, err = cache.Delete(ctx, vsReq.Version)
	if err != nil || !deleted {
		t.Errorf("cache.Delete(ctx, %q) = (%v, %v), want (true, nil)", vsReq.Version, deleted, err)
	}

	deleted, err = cache.Delete(ctx, vsReq.Version)
	if err != nil || deleted {
		t.Errorf("cache.Delete(ctx, %q) = (%v, %v), want (false, nil)", vsReq.Version, deleted, err)
	}
}

func TestLocalCacheLimitSet(t *testing.T) {
	ctx := context.Background()
	config := vs_cache.CacheConfig{
		EnableLocalCache: true,
		LocalCapacity:    3,
	}
	cache, closeCache, err := vs_cache.NewCache(ctx, config)
	if err != nil {
		t.Fatalf("vs_cache.NewCache(%v, %v) failed: %v", ctx, config, err)
	}
	defer closeCache()

	var vsReq VehicleSignalRequest
	if err := testhelper.LoadJSON(VALID_VS_VERSION, &vsReq); err != nil {
		t.Error(err)
	}

	for _, tc := range []struct {
		action        string
		key           string
		wantCacheList []string
	}{
		{action: "Delete", key: "1", wantCacheList: []string{}},
		{action: "Set", key: "1", wantCacheList: []string{"1"}},
		{action: "Set", key: "1", wantCacheList: []string{"1"}},
		{action: "Set", key: "2", wantCacheList: []string{"1", "2"}},
		{action: "Set", key: "3", wantCacheList: []string{"1", "2", "3"}},
		{action: "Set", key: "4", wantCacheList: []string{"2", "3", "4"}},
		{action: "Set", key: "4", wantCacheList: []string{"2", "3", "4"}},
		{action: "Delete", key: "1", wantCacheList: []string{"2", "3", "4"}},
		{action: "Delete", key: "4", wantCacheList: []string{"2", "3"}},
	} {
		t.Logf("Testcase: %v %s", tc.action, tc.key)

		if tc.action == "Delete" {
			if _, err := cache.Delete(ctx, tc.key); err != nil {
				t.Error(err)
			}
		} else if tc.action == "Set" {
			if err := cache.Set(ctx, tc.key, vsReq.VehicleSignals); err != nil {
				t.Error(err)
			}
		}

		got, err := cache.List(ctx)
		if err != nil {
			t.Error(err)
		}

		less := func(a, b string) bool { return a < b }
		if diff := cmp.Diff(tc.wantCacheList, got, cmpopts.SortSlices(less)); diff != "" {
			t.Errorf("cache.List(ctx) is unexpectedly different (-want +got):\n%s", diff)
		}
	}
}

func TestDataRaceGetSet(t *testing.T) {
	ctx := context.Background()
	config := vs_cache.CacheConfig{
		EnableLocalCache: true,
		LocalCapacity:    100,
	}
	cache, closeCache, err := vs_cache.NewCache(ctx, config)
	if err != nil {
		t.Fatalf("vs_cache.NewCache(%v, %v) failed: %v", ctx, config, err)
	}
	defer closeCache()

	var vsReq VehicleSignalRequest
	if err := testhelper.LoadJSON(VALID_VS_VERSION, &vsReq); err != nil {
		t.Error(err)
	}

	if err := cache.Set(ctx, "1", vsReq.VehicleSignals); err != nil {
		t.Error(err)
	}
	if err := cache.Set(ctx, "2", vsReq.VehicleSignals); err != nil {
		t.Error(err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: invoke Set
	go func() {
		defer wg.Done()
		cache.Set(ctx, "3", vsReq.VehicleSignals)
	}()

	// Goroutine 2: invoke Get
	go func() {
		defer wg.Done()
		cache.Get(ctx, "1")
	}()

	wg.Wait()
}

func TestGetDoesNotModifyCache(t *testing.T) {
	ctx := context.Background()
	config := vs_cache.CacheConfig{
		EnableLocalCache: true,
	}
	cache, closeCache, err := vs_cache.NewCache(ctx, config)
	if err != nil {
		t.Fatalf("vs_cache.NewCache(%v, %v) failed: %v", ctx, config, err)
	}
	defer closeCache()

	var vsReq VehicleSignalRequest
	if err := testhelper.LoadJSON(VALID_VS_VERSION, &vsReq); err != nil {
		t.Fatalf("testhelper.LoadJSON(%q, &vsReq) failed: %v", VALID_VS_VERSION, err)
	}

	key := "test_key"
	if err := cache.Set(ctx, key, vsReq.VehicleSignals); err != nil {
		t.Fatalf("cache.Set(ctx, %q, ...) failed: %v", key, err)
	}

	// Get the resolver from the cache.
	resolver1, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("cache.Get(ctx, %q) failed: %v", key, err)
	}

	// Modify the returned resolver by extending it with new types.
	// This should not affect the cached version.
	newTypes := testhelper.GetSpeedFdWithDependencies()
	if err := resolver1.ExtendLocalTypes(newTypes); err != nil {
		t.Fatalf("resolver1.ExtendLocalTypes() failed: %v", err)
	}

	// Verify that the resolver now contains the new type.
	if _, err := resolver1.Local.FindMessageByName(VEHICLE_SPEED_MSG_FNAME); err != nil {
		t.Errorf("resolver1.Local.FindMessageByName(%q) failed after ExtendLocalTypes: %v, want nil", VEHICLE_SPEED_MSG_FNAME, err)
	}

	// Verify that a new resolver for the same vehicle version does NOT contain the new type.
	resolver2, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("second cache.Get(ctx, %q) failed: %v", key, err)
	}
	if _, err := resolver2.Local.FindMessageByName(VEHICLE_SPEED_MSG_FNAME); err == nil {
		t.Errorf("resolver2.Local.FindMessageByName(%q) got a message, want an error because the cached resolver should not have been modified", VEHICLE_SPEED_MSG_FNAME)
	}
}
