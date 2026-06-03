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

// Collects important constants used in several different packages. Should have minimal or no
// imports.
package constants

import "fmt"

// MetricsConfigVersion is Experimental 2
const MetricsConfigVersion uint32 = 0xE0000002

type APIVersion int

const (
	APIVersionUnknown APIVersion = iota
	APIVersionV1
	APIVersionV2
)

const (
	CurrentAPIVersion = APIVersionV2
)

func (v APIVersion) String() string {
	switch v {
	case APIVersionV1:
		return "v1"
	case APIVersionV2:
		return "v2"
	default:
		panic(fmt.Sprintf("unknown API version: %d", v))
	}
}
