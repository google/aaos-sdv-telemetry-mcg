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

package vs_cache_integration_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/reflect/protoreflect"

	"sdv.googlesource.com/mcg/mcg/testhelper"
	"sdv.googlesource.com/mcg/mcg/vs_cache"
)

const (
	clusterDockerfilePath                          = "../testdata/redis/Dockerfile"
	standaloneDockerfilePath                       = "../testdata/redis-standalone/Dockerfile"
	VALID_VS_VERSION                               = "../testdata/vsignal_api/sample_ssot_ver1.json"
	SPEED_REPORT_MSG_FNAME   protoreflect.FullName = "mcg.test.report.speed_report"
)

type VehicleSignalRequest struct {
	Version        string `json:"version"`
	VehicleSignals []byte `json:"vehicle_signals"`
}

func TestRedisCacheCluster(t *testing.T) {
	// Temporary copy for dockertest is required to resolve pathing issue in dockertest
	tmpDir := t.TempDir()
	err := copyDirectory(filepath.Dir(clusterDockerfilePath), tmpDir)
	if err != nil {
		t.Fatalf("copyDirectory(%q, %q) failed: %v", filepath.Dir(clusterDockerfilePath), tmpDir, err)
	}

	pool, resource, err := setupDockerRedisCluster(filepath.Join(tmpDir, filepath.Base(clusterDockerfilePath)))
	if err != nil {
		t.Fatalf("setupDockerRedisCluster(dockerfilePath) failed: %v", err)
	}
	t.Logf("Docker container started: %s", resource.Container.ID)

	defer func() {
		if err = pool.Purge(resource); err != nil {
			log.Fatalf("Could not purge resource: %s", err)
		}
	}()

	ctx := context.Background()

	waitForContainerHealthy(ctx, t, pool, resource)

	config := vs_cache.CacheConfig{
		EnableLocalCache: false,
		EnableRedCache:   true,
		IsRedisCluster:   true,
		RedClusterOpts: redis.ClusterOptions{
			Addrs: []string{net.JoinHostPort("127.0.0.1", resource.GetPort("7000/tcp"))},
		},
	}
	cache, closeCache, err := vs_cache.NewCache(ctx, config)
	if err != nil {
		t.Fatalf("vs_cache.NewCache(%v, %v) failed: %v", ctx, config, err)
	}
	defer closeCache()

	runCacheOperationsTest(ctx, t, cache)
}

func TestRedisCacheStandalone(t *testing.T) {
	// Temporary copy for dockertest is required to resolve pathing issue in dockertest
	tmpDir := t.TempDir()
	err := copyDirectory(filepath.Dir(standaloneDockerfilePath), tmpDir)
	if err != nil {
		t.Fatalf("copyDirectory(%q, %q) failed: %v", filepath.Dir(standaloneDockerfilePath), tmpDir, err)
	}

	pool, resource, err := setupDockerRedisStandalone(filepath.Join(tmpDir, filepath.Base(standaloneDockerfilePath)))
	if err != nil {
		t.Fatalf("setupDockerRedisStandalone(dockerfilePath) failed: %v", err)
	}
	t.Logf("Docker container started: %s", resource.Container.ID)

	defer func() {
		if err = pool.Purge(resource); err != nil {
			log.Fatalf("Could not purge resource: %s", err)
		}
	}()

	ctx := context.Background()

	waitForContainerHealthy(ctx, t, pool, resource)
	config := vs_cache.CacheConfig{
		EnableLocalCache: false,
		EnableRedCache:   true,
		IsRedisCluster:   false,
		RedOpts: redis.Options{
			Addr: net.JoinHostPort("127.0.0.1", resource.GetPort("6379/tcp")),
		},
	}
	cache, closeCache, err := vs_cache.NewCache(ctx, config)
	if err != nil {
		t.Fatalf("vs_cache.NewCache(%v, %v) failed: %v", ctx, config, err)
	}
	defer closeCache()

	runCacheOperationsTest(ctx, t, cache)
}

func runCacheOperationsTest(ctx context.Context, t *testing.T, cache *vs_cache.Cache) {
	var vsReq VehicleSignalRequest
	if err := testhelper.LoadJSON(VALID_VS_VERSION, &vsReq); err != nil {
		t.Fatalf("testhelper.LoadJSON(%q, &vsReq) failed: %v", VALID_VS_VERSION, err)
	}

	deleted, err := cache.Delete(ctx, vsReq.Version)
	if err != nil || deleted {
		t.Fatalf("cache.Delete(ctx, %q) = (%v, %v), want (false, nil)", vsReq.Version, deleted, err)
	}

	if err = cache.Set(ctx, vsReq.Version, vsReq.VehicleSignals); err != nil {
		t.Fatalf("cache.Set(ctx, %q, vsReq.VehicleSignals) failed: %v", vsReq.Version, err)
	}

	res, err := cache.Get(ctx, vsReq.Version)
	if err != nil {
		t.Fatalf("cache.Get(ctx, %q) failed: %v", vsReq.Version, err)
	}

	resMsg, err := res.FindMessageByName(SPEED_REPORT_MSG_FNAME)
	if err != nil {
		t.Fatalf("res.FindMessageByName(%q) failed: %v", SPEED_REPORT_MSG_FNAME, err)
	}

	if want, got := SPEED_REPORT_MSG_FNAME, resMsg.Descriptor().FullName(); want != got {
		t.Errorf("Retrieved Cache Message Malformed: %s != %s", want, got)
	}

	deleted, err = cache.Delete(ctx, vsReq.Version)
	if err != nil || !deleted {
		t.Fatalf("cache.Delete(ctx, %q) = (%v, %v), want (true, nil)", vsReq.Version, deleted, err)
	}

	deleted, err = cache.Delete(ctx, vsReq.Version)
	if err != nil || deleted {
		t.Fatalf("cache.Delete(ctx, %q) = (%v, %v), want (false, nil)", vsReq.Version, deleted, err)
	}
}

func waitForContainerHealthy(ctx context.Context, t *testing.T, pool *dockertest.Pool, resource *dockertest.Resource) {
	t.Helper()
	if err := backoff.Retry(func() error {
		container, err := pool.Client.InspectContainerWithContext(resource.Container.ID, ctx)
		if err != nil {
			return backoff.Permanent(err)
		}
		t.Logf("Waiting for container to be healthy. Current status: %s\n", container.State.String())

		if !container.State.Running {
			return backoff.Permanent(fmt.Errorf("Container exited"))
		}

		if container.State.Health.Status != "healthy" {
			return fmt.Errorf("Container health is: %s", container.State.Health.Status)
		}
		return nil
	}, backoff.WithContext(backoff.NewConstantBackOff(time.Second), ctx)); err != nil {
		t.Fatalf("Failed to wait for container to become healthy: %v", err)
	}
	t.Log("Container is healthy")
}

func setupDockerRedisCluster(dockerfilePath string) (*dockertest.Pool, *dockertest.Resource, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, nil, fmt.Errorf("Could not construct pool: %w", err)
	}

	err = pool.Client.Ping()
	if err != nil {
		return nil, nil, fmt.Errorf("Could not connect to Docker: %w", err)
	}

	runOpts := &dockertest.RunOptions{
		Name:         "test-redis-cluster",
		Repository:   "test-redis-cluster",
		ExposedPorts: []string{"7000", "7001", "7002"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"7000/tcp": {{HostIP: "0.0.0.0", HostPort: "7000/tcp"}},
			"7001/tcp": {{HostIP: "0.0.0.0", HostPort: "7001/tcp"}},
			"7002/tcp": {{HostIP: "0.0.0.0", HostPort: "7002/tcp"}},
		},
	}

	resource, err := pool.BuildAndRunWithOptions(dockerfilePath, runOpts)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not start resource: %w", err)
	}
	resource.Expire(60)

	return pool, resource, err
}

func setupDockerRedisStandalone(dockerfilePath string) (*dockertest.Pool, *dockertest.Resource, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, nil, fmt.Errorf("could not construct pool: %w", err)
	}

	if err = pool.Client.Ping(); err != nil {
		return nil, nil, fmt.Errorf("could not connect to Docker: %w", err)
	}

	runOpts := &dockertest.RunOptions{
		Name:         "test-redis-standalone",
		Repository:   "test-redis-standalone",
		ExposedPorts: []string{"7000"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"7000/tcp": {{HostIP: "0.0.0.0", HostPort: "7000/tcp"}},
		},
	}
	resource, err := pool.BuildAndRunWithOptions(dockerfilePath, runOpts)
	if err != nil {
		return nil, nil, fmt.Errorf("could not start resource: %w", err)
	}
	resource.Expire(60)
	return pool, resource, err
}

func copyDirectory(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dest, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		destFile, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer destFile.Close()

		_, err = io.Copy(destFile, srcFile)
		return err
	})
}
