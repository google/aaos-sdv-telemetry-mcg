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

package mcuuid

import (
	"encoding"
	"fmt"

	"github.com/google/uuid"
)

type MCUUID uuid.UUID

func NewRandom() (MCUUID, error) {
	m, err := uuid.NewRandom()
	if err != nil {
		return MCUUID{}, err
	}
	return MCUUID(m), nil
}

func ParseBytes(s []byte) (MCUUID, error) {
	u, err := uuid.ParseBytes(s)
	if err != nil {
		return MCUUID{}, fmt.Errorf("UUID is formatted incorrectly: %w", err)
	}
	// Enforce the hyphenated, standardized format for UUIDs required by the
	// telemetry service (e.g. 'a1a2a3a4-b1b2-c1c2-d1d2-d3d4d5d6d7d8').
	// `uuid.ParseBytes` is not strict enough and supports non-standard UUID
	// formats, including uppercase UUIDs.
	if u.String() != string(s) {
		return MCUUID{}, fmt.Errorf("UUID is formatted incorrectly; must be a valid UUID in the standard lowercase hyphenated format (e.g., 'a1a2a3a4-b1b2-c1c2-d1d2-d3d4d5d6d7d8')")
	}

	if u == uuid.Nil {
		return MCUUID{}, fmt.Errorf("UUID must not be the Nil UUID (https://www.ietf.org/rfc/rfc9562.html#name-nil-uuid)")
	}

	if u == uuid.Max {
		return MCUUID{}, fmt.Errorf("UUID must not be the Max UUID (https://www.ietf.org/rfc/rfc9562.html#name-max-uuid")
	}

	return MCUUID(u), nil
}

func (m MCUUID) String() string {
	return uuid.UUID(m).String()
}

func (m MCUUID) MarshalText() ([]byte, error) {
	return uuid.UUID(m).MarshalText()
}

func (m *MCUUID) UnmarshalText(data []byte) error {
	u, err := ParseBytes(data)
	if err != nil {
		return err
	}
	*m = u
	return nil
}

var _ fmt.Stringer = (*MCUUID)(nil)
var _ encoding.TextMarshaler = (*MCUUID)(nil)
var _ encoding.TextUnmarshaler = (*MCUUID)(nil)
