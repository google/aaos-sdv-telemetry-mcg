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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
)

const (
	entryTtl   = 0 // TTL of Redis Entries (0 = no expiration)
	vsPrefix   = "sdv:vehicle_signal:"
	vsWriterId = "sdv:vehicle_signal_writer:"
)

type RedisCache struct {
	Client              redis.UniversalClient
	ClientId            int64
	cancelNotifications context.CancelFunc
}

type storeObj struct {
	VehicleSignals []byte `json:"vehicle_signals"`
}

func (so storeObj) MarshalBinary() ([]byte, error) {
	return json.Marshal(so)
}

func (so *storeObj) UnmarshalBinary(data []byte) error {
	err := json.Unmarshal(data, &so)
	return err
}

func NewRedisCache(client redis.UniversalClient) *RedisCache {
	rcCache := RedisCache{
		Client: client,
	}
	return &rcCache
}

func (rc *RedisCache) close() {
	if rc.cancelNotifications != nil {
		rc.cancelNotifications()
	}
}

func (rc *RedisCache) Get(ctx context.Context, version string) ([]byte, error) {
	var sObj storeObj
	err := rc.Client.Get(ctx, versionKey(version)).Scan(&sObj)
	if errors.Is(err, redis.Nil) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return sObj.VehicleSignals, err
}

func (rc *RedisCache) Set(ctx context.Context, version string, vehicle_signals []byte) error {
	sObj := storeObj{
		VehicleSignals: vehicle_signals,
	}

	txf := func(tx *redis.Tx) error {
		_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, clientWriteKey(version), rc.ClientId, entryTtl)
			pipe.Set(ctx, versionKey(version), sObj, entryTtl)
			return nil
		})
		return err
	}

	err := rc.Client.Watch(ctx, txf, clientWriteKey(version), versionKey(version))
	return err
}

func (rc *RedisCache) Delete(ctx context.Context, version string) (bool, error) {
	deletedCount, err := rc.Client.Del(ctx, versionKey(version)).Result()
	if err != nil {
		return false, err
	}
	return deletedCount > 0, nil
}

func (rc *RedisCache) List(ctx context.Context) ([]string, error) {
	keys := map[string]struct{}{}
	mutex := sync.Mutex{}

	scanNode := func(ctx context.Context, client *redis.Client) error {
		iter := client.Scan(ctx, 0, vsPrefix+"*", 0).Iterator()
		for iter.Next(ctx) {
			key := iter.Val()
			mutex.Lock()
			keys[key] = struct{}{}
			mutex.Unlock()
		}
		return iter.Err()
	}

	var err error
	switch c := rc.Client.(type) {
	case *redis.ClusterClient:
		err = c.ForEachMaster(ctx, scanNode)
	case *redis.Client:
		err = scanNode(ctx, c)
	default:
		return nil, fmt.Errorf("unsupported redis client type for List: %T", rc.Client)
	}
	if err != nil {
		return nil, err
	}

	trimmedKeys := make([]string, 0, len(keys))
	for key := range keys {
		trimmedKeys = append(trimmedKeys, strings.TrimSuffix(strings.TrimPrefix(key, vsPrefix+"{"), "}"))
	}
	return trimmedKeys, nil
}

func (rc *RedisCache) SetupNotifications(ctx context.Context, invalidate func(key string)) error {
	clientId, err := rc.Client.ClientID(ctx).Result()
	if err != nil {
		return err
	}
	rc.ClientId = clientId

	// Create a new context for the background Redis notifications.
	//
	// `context.WithoutCancel(ctx)` creates a context that is not cancelled when
	// the input `ctx` is cancelled. This is crucial because the subscription
	// goroutines should outlive the context used to call SetupNotifications.
	//
	// `context.WithCancel(...)` then adds a cancellation function to this new
	// context, allowing the notifications to be explicitly shut down later via
	// `rc.close()`.
	notificationCtx, cancelNotifications := context.WithCancel(context.WithoutCancel(ctx))
	subscribeOnNode := func(ctx context.Context, client *redis.Client) error {
		pubSub := client.PSubscribe(ctx, fmt.Sprintf("__keyspace@0__:%s{*}", vsPrefix))

		go func() {
			defer pubSub.Close()
			notificationChannel := pubSub.Channel()

			log.Printf("[RedisNotification] Activated on node %s", client.Options().Addr)
			for {
				select {
				case <-notificationCtx.Done():
					log.Printf("[RedisNotification] Subscription channel closed for node %s", client.Options().Addr)
					return
				case notification := <-notificationChannel:
					log.Printf("[RedisNotification] [Channel] %v [Payload] %v", notification.Channel, notification.Payload)
					version := strings.TrimSuffix(strings.TrimPrefix(notification.Channel, "__keyspace@0__:sdv:vehicle_signal:{"), "}")

					if strings.Contains(notification.Payload, "del") || strings.Contains(notification.Payload, "set") {
						var writerClientId int64
						err := rc.Client.Get(notificationCtx, clientWriteKey(version)).Scan(&writerClientId)
						if err != nil && !errors.Is(err, redis.Nil) {
							log.Printf("[RedisNotification] Error getting writer id for key %v: %v", version, err)
							continue
						}
						if writerClientId == rc.ClientId {
							log.Printf("[RedisNotification] Ignoring key space event for key: %v - From same client: %v", version, rc.ClientId)
						} else {
							log.Printf("[RedisNotification] Invalidating key: %v - From client: %v", version, writerClientId)
							invalidate(version)
						}
					}
				}
			}
		}()

		return nil
	}

	switch c := rc.Client.(type) {
	case *redis.ClusterClient:
		err = c.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
			return subscribeOnNode(ctx, client)
		})
		if err != nil {
			cancelNotifications()
			return err
		}
	case *redis.Client:
		err = subscribeOnNode(ctx, c)
		if err != nil {
			cancelNotifications()
			return err
		}
	default:
		cancelNotifications()
		return fmt.Errorf("unsupported redis client type for SetupNotifications: %T", rc.Client)
	}

	rc.cancelNotifications = cancelNotifications
	return nil
}

func clientWriteKey(version string) string {
	return fmt.Sprintf("%s{%s}", vsWriterId, version)
}

func versionKey(version string) string {
	return fmt.Sprintf("%s{%s}", vsPrefix, version)
}
