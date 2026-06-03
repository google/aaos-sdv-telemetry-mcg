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

// Package docs provides MCG's documentation.
package docs

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

//go:embed openapi.yaml
var in []byte

func GetOpenApiSpec(url string) ([]byte, []byte, error) {
	var config map[string]any
	if err := yaml.Unmarshal(in, &config); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal openapi.yaml: %w", err)
	}

	servers, ok := config["servers"]
	if !ok {
		return nil, nil, fmt.Errorf("unexpected openapi.yaml format: missing servers field")
	}

	serversSlice, ok := servers.([]any)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected openapi.yaml format: servers field is not a slice of strings, got %T", servers)
	}

	if len(serversSlice) != 1 {
		return nil, nil, fmt.Errorf("unexpected openapi.yaml format: servers array should have exactly one element, got %d", len(serversSlice))
	}

	serverValue, ok := serversSlice[0].(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected openapi.yaml format: servers array element is not a map, got %T", serversSlice[0])
	}

	variables, ok := serverValue["variables"]
	if !ok {
		return nil, nil, fmt.Errorf("unexpected openapi.yaml format: server object is missing 'variables' field")
	}

	variablesMap, ok := variables.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected openapi.yaml format: 'variables' field is not a map, got %T", variables)
	}

	urlVar, ok := variablesMap["url"]
	if !ok {
		return nil, nil, fmt.Errorf("unexpected openapi.yaml format: 'variables' map is missing 'url' field")
	}

	urlVarMap, ok := urlVar.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected openapi.yaml format: 'url' variable is not a map, got %T", urlVar)
	}

	urlVarMap["default"] = url

	outYaml, err := yaml.Marshal(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal openapi.yaml: %w", err)
	}
	outJson, err := json.Marshal(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal openapi.json: %w", err)
	}
	return outYaml, outJson, nil
}
