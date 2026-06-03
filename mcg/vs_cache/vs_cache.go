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

package vs_cache

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
	"sdv.googlesource.com/mcg/mcg/type_resolvers"
)

type Cache struct {
	LocCache *LocalCache
	RedCache *RedisCache
}

type CacheConfig struct {
	EnableLocalCache bool
	LocalCapacity    int // 0 = No Capacity Limit
	EnableRedCache   bool
	IsRedisCluster   bool
	RedOpts          redis.Options
	RedClusterOpts   redis.ClusterOptions
}

// NewCache creates a new cache instance. Make sure to call the returned
// cancellation callback when you no longer need the cache or it will not be
// cleaned up properly.
func NewCache(ctx context.Context, config CacheConfig) (*Cache, func(), error) {
	cache := &Cache{}

	if config.EnableLocalCache {
		cache.LocCache = NewLocalCache(config.LocalCapacity)
	}
	if config.EnableRedCache {
		var client redis.UniversalClient
		if config.IsRedisCluster {
			client = redis.NewClusterClient(&config.RedClusterOpts)
		} else {
			client = redis.NewClient(&config.RedOpts)
		}
		cache.RedCache = NewRedisCache(client)
	}

	if config.EnableLocalCache && config.EnableRedCache {
		if err := cache.RedCache.SetupNotifications(ctx, func(key string) {
			cache.LocCache.Delete(key)
		}); err != nil {
			return nil, nil, err
		}
	}

	return cache, func() {
		if cache.RedCache != nil {
			cache.RedCache.close()
		}
	}, nil
}

func (ca *Cache) Get(ctx context.Context, key string) (*type_resolvers.EnrichedTypeResolver, error) {
	if !ca.IsActive() {
		return nil, mcgerrors.NoCache()
	}

	var vsBytes []byte

	// Check local cache first.
	if ca.LocCache != nil {
		vsBytes = ca.LocCache.Get(key)
	}

	// If not in local cache, check Redis cache.
	if vsBytes == nil && ca.RedCache != nil {
		var err error
		vsBytes, err = ca.RedCache.Get(ctx, key)
		if err != nil {
			return nil, mcgerrors.CacheFailedLookup(key)
		}
	}

	// Version not found.
	if vsBytes == nil {
		return nil, nil
	}

	retVal, err := type_resolvers.NewEnrichedTypeResolverFromBytes(vsBytes)
	if err != nil {
		return retVal, mcgerrors.FileDescriptorSetVehicleSignalsFailToParse(err)
	}

	return retVal, err
}

func (ca *Cache) Set(ctx context.Context, key string, vehicle_signals []byte) error {
	if !ca.IsActive() {
		return mcgerrors.NoCache()
	}

	// Validate vehicle signals
	_, err := type_resolvers.NewEnrichedTypeResolverFromBytes(vehicle_signals)
	if err != nil {
		return mcgerrors.FileDescriptorSetVehicleSignalsFailToParse(err)
	}

	if ca.RedCache != nil {
		err := ca.RedCache.Set(ctx, key, vehicle_signals)
		if err != nil {
			return err
		}
	}

	if ca.LocCache != nil {
		ca.LocCache.Set(key, vehicle_signals)
	}

	return nil
}

func (ca *Cache) Delete(ctx context.Context, key string) (bool, error) {
	deleted := false

	if !ca.IsActive() {
		return deleted, mcgerrors.NoCache()
	}

	if ca.RedCache != nil {
		redDeleted, err := ca.RedCache.Delete(ctx, key)
		deleted = deleted || redDeleted
		if err != nil {
			return deleted, err
		}
	}

	if ca.LocCache != nil {
		deleted = deleted || ca.LocCache.Delete(key)
	}

	return deleted, nil
}

func (ca *Cache) List(ctx context.Context) ([]string, error) {
	if !ca.IsActive() {
		return nil, mcgerrors.NoCache()
	}

	if ca.RedCache != nil {
		return ca.RedCache.List(ctx)
	} else if ca.LocCache != nil {
		return ca.LocCache.List(), nil
	}
	return nil, nil
}

func (ca *Cache) IsActive() bool {
	return ca.LocCache != nil || ca.RedCache != nil
}

func GetCacheFromContext(ctx context.Context) (*Cache, error) {
	cache := ctx.Value("cache")
	if cache == nil {
		return nil, mcgerrors.NoCache()
	}
	return cache.(*Cache), nil
}

func CacheMiddleware(cache *Cache) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("cache", cache)
		c.Next()
	}
}
