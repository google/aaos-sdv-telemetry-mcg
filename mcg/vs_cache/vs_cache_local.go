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
	"container/list"
	"log"
	"sync"
)

const noCapacityLimit = 0

type entry struct {
	version         string
	vehicle_signals []byte
}

type LocalCache struct {
	// versionsMap maps from version string to element in OrderedVsList which contains vehicle signals
	versionsMap map[string]*list.Element
	capacity    int
	// OrderedVsList directly contains vehicle signals, ordered by last accessed
	OrderedVsList *list.List
	sync.Mutex
}

func NewLocalCache(cap int) *LocalCache {
	return &LocalCache{
		versionsMap:   make(map[string]*list.Element),
		capacity:      cap,
		OrderedVsList: list.New(),
	}
}

func (lc *LocalCache) Get(version string) []byte {
	lc.Lock()
	defer lc.Unlock()

	el, ok := lc.versionsMap[version]

	if ok {
		lc.OrderedVsList.MoveToFront(el)
		return el.Value.(*entry).vehicle_signals
	}
	return nil
}

func (lc *LocalCache) Set(version string, vehicle_signals []byte) {
	lc.Lock()
	defer lc.Unlock()

	el, ok := lc.versionsMap[version]

	if ok {
		lc.OrderedVsList.MoveToFront(el)
		el.Value.(*entry).vehicle_signals = vehicle_signals
	} else {
		el = lc.OrderedVsList.PushFront(&entry{version: version, vehicle_signals: vehicle_signals})
		lc.versionsMap[version] = el
		lc.evict()
	}
}

func (lc *LocalCache) Delete(version string) bool {
	lc.Lock()
	defer lc.Unlock()

	el, ok := lc.versionsMap[version]
	if !ok {
		return false
	}

	delete(lc.versionsMap, version)
	lc.OrderedVsList.Remove(el)
	log.Printf("[LocalCache] Deleted vehicle signals with version %v\n", version)
	return true
}

func (lc *LocalCache) List() []string {
	lc.Lock()
	defer lc.Unlock()

	vsList := make([]string, 0, lc.OrderedVsList.Len())
	for k := range lc.versionsMap {
		vsList = append(vsList, k)
	}

	return vsList
}

// Write lock is expected to be held by the caller
func (lc *LocalCache) evict() {
	if lc.capacity == noCapacityLimit {
		return
	}
	if lc.OrderedVsList.Len() > lc.capacity {
		last := lc.OrderedVsList.Back()
		version := last.Value.(*entry).version
		lc.OrderedVsList.Remove(last)

		delete(lc.versionsMap, version)
	}
}
