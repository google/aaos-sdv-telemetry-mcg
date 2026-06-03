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

package docs_test

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"

	"sdv.googlesource.com/mcg/mcg/docs"
)

type expectedConfig struct {
	Servers []struct {
		URL       string
		Variables struct {
			URL struct {
				Default     string
				Description string
				Enum        []string
			}
		}
	}
}

func TestGetOpenApiSpec(t *testing.T) {
	const testURL = "https://my-test-url.example.com"

	outYaml, outJson, err := docs.GetOpenApiSpec(testURL)
	if err != nil {
		t.Fatalf("GetOpenApiSpec(%q) failed: %v", testURL, err)
	}

	t.Run("yaml", func(t *testing.T) {
		var config expectedConfig
		if err := yaml.Unmarshal(outYaml, &config); err != nil {
			t.Fatalf("yaml.Unmarshal(%q, _) failed: %v", outYaml, err)
		}

		got := config.Servers[0].Variables.URL.Default
		if want := testURL; got != want {
			t.Errorf("Default URL mismatch: want %q, got %q", want, got)
		}
	})

	t.Run("json", func(t *testing.T) {
		var config expectedConfig
		if err := json.Unmarshal(outJson, &config); err != nil {
			t.Fatalf("json.Unmarshal(%q, _) failed: %v", outJson, err)
		}

		got := config.Servers[0].Variables.URL.Default
		if want := testURL; got != want {
			t.Errorf("Default URL mismatch: want %q, got %q", want, got)
		}
	})
}
