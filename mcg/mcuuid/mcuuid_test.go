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

package mcuuid_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"

	"sdv.googlesource.com/mcg/mcg/mcuuid"
)

const VALID_UUID string = "24335816-82b5-4878-9a19-0792b72dab5b"

func TestNewRandom(t *testing.T) {
	m1, err := mcuuid.NewRandom()
	if err != nil {
		t.Fatalf("mcuuid.NewRandom() failed: %v", err)
	}

	m2, err := mcuuid.NewRandom()
	if err != nil {
		t.Fatalf("mcuuid.NewRandom() failed: %v", err)
	}

	if m1 == m2 {
		t.Errorf("mcuuid.NewRandom() returned the same UUID twice in a row: %q, %q", m1, m2)
	}
}

func TestParse(t *testing.T) {
	m, err := mcuuid.ParseBytes([]byte(VALID_UUID))

	if err != nil {
		t.Fatalf("ParseBytes(%q) failed: %v", VALID_UUID, err)
	}
	if m.String() != VALID_UUID {
		t.Errorf("ParseBytes(%q) = %q, _, want %q, _", VALID_UUID, m.String(), VALID_UUID)
	}
}

func TestUnmarshalText(t *testing.T) {
	m := mcuuid.MCUUID{}
	err := m.UnmarshalText([]byte(VALID_UUID))

	if err != nil {
		t.Fatalf("m.UnmarshalText(%q) failed: %v", VALID_UUID, err)
	}
	if m.String() != VALID_UUID {
		t.Errorf("m.UnmarshalText(%q) = %q, _, want %q, _", VALID_UUID, m.String(), VALID_UUID)
	}
}

func TestParseAndUnmarshalTextInvalidUuid(t *testing.T) {
	testCases := []struct {
		name              string
		input             string
		wantErrContaining string
	}{
		{
			name:              "invalid_uuid_format",
			input:             "not-a-uuid",
			wantErrContaining: "invalid UUID length: 10",
		},
		{
			name:              "uppercase",
			input:             "A1A2A3A4-B1B2-C1C2-D1D2-D3D4D5D6D7D8",
			wantErrContaining: "must be a valid UUID in the standard lowercase hyphenated format",
		},
		{
			name:              "no_hyphens",
			input:             "a1a2a3a4b1b2c1c2d1d2d3d4d5d6d7d8",
			wantErrContaining: "must be a valid UUID in the standard lowercase hyphenated format",
		},
		{
			name:              "nil_uuid",
			input:             uuid.Nil.String(),
			wantErrContaining: "must not be the Nil UUID",
		},
		{
			name:              "max_uuid",
			input:             uuid.Max.String(),
			wantErrContaining: "must not be the Max UUID",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := mcuuid.ParseBytes([]byte(tc.input))
			if err == nil {
				t.Errorf("ParseBytes(%q) expected an error, but got nil", tc.input)
			} else if tc.wantErrContaining != "" && !strings.Contains(err.Error(), tc.wantErrContaining) {
				t.Errorf("ParseBytes(%q) error = %q, want containing %q", tc.input, err.Error(), tc.wantErrContaining)
			}

			var m mcuuid.MCUUID
			err = m.UnmarshalText([]byte(tc.input))
			if err == nil {
				t.Errorf("m.UnmarshalText(%q) should fail", tc.input)
			} else if tc.wantErrContaining != "" && !strings.Contains(err.Error(), tc.wantErrContaining) {
				t.Errorf("m.UnmarshalText(%q) error = %q, want containing %q", tc.input, err.Error(), tc.wantErrContaining)
			}
		})
	}
}

func TestString(t *testing.T) {
	u := uuid.New()
	m := mcuuid.MCUUID(u)
	if want, got := u.String(), m.String(); want != got {
		t.Errorf("m.String() = %q, want %q", got, want)
	}
}

func TestMarshalText(t *testing.T) {
	u := uuid.New()
	m := mcuuid.MCUUID(u)

	got, err := m.MarshalText()
	if err != nil {
		t.Fatalf("m.MarshalText() failed: %v", err)
	}

	if want := []byte(u.String()); !cmp.Equal(want, got) {
		t.Errorf("m.MarshalText() = %q, want %q", got, want)
	}
}
