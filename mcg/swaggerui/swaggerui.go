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

package swaggerui

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed dist
var embedded embed.FS

var Files fs.FS

func init() {
	var err error
	if Files, err = fs.Sub(embedded, "dist"); err != nil {
		// This can never happen, because the `go:embed` call above embeds,
		// well, a `dist` directory. Therefore, it is fine to outright panic
		// here.
		panic(fmt.Errorf("Embedded Swagger static files are unexpectedly missing a `dist` directory: %w", err))
	}
}
