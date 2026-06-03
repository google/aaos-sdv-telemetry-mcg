// Copyright 2023 Google LLC
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

package mcg

import (
	"net/http"

	"github.com/gin-gonic/gin"

	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
)

type Server struct{}

var VERSION = "redacted"

const (
	CONTENT_TYPE_TEXT_X_PROTOBUF = "text/x-protobuf"
	CONTENT_TYPE_APP_X_PROTOBUF  = "application/x-protobuf"
)

type VersionInfo struct {
	ServiceVersion string `json:"service_version"`
}

// InstallRoutes is the primary entry point for this package, installing route handlers into a gin.Engine.
func (server *Server) InstallRoutes(r *gin.Engine) {
	r.NoRoute(func(c *gin.Context) {
		c.Error(mcgerrors.NotFound("Page not found"))
	})
	r.GET("/health", handleHealth)

	installV1Routes(r, server)
	installV2Routes(r, server)
}

func handleHealth(c *gin.Context) {
	c.String(http.StatusOK, "ok")
}
