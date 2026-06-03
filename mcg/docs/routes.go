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

package docs

import (
	"fmt"
	"net/http"
	"testing/fstest"

	"github.com/gin-gonic/gin"

	"sdv.googlesource.com/mcg/mcg/swaggerui"
)

func InstallRoutes(baseURL string, r *gin.Engine) error {
	openapiYaml, openapiJson, err := GetOpenApiSpec(baseURL)
	if err != nil {
		return fmt.Errorf("Failed to initialize OpenAPI docs: %v", err)
	}

	d := r.Group("/docs")
	d.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/docs/swagger-ui")
	})
	d.StaticFS("/swagger-ui", http.FS(swaggerui.Files))
	d.StaticFileFS("/openapi.yaml", "openapi.yaml", http.FS(fstest.MapFS{
		"openapi.yaml": &fstest.MapFile{Data: openapiYaml},
	}))
	d.StaticFileFS("/openapi.json", "openapi.json", http.FS(fstest.MapFS{
		"openapi.json": &fstest.MapFile{Data: openapiJson},
	}))

	return nil
}
