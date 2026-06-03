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

package mcg

import (
	"github.com/gin-gonic/gin"
	"sdv.googlesource.com/mcg/mcg/constants"
	"sdv.googlesource.com/mcg/mcg/signal_versions"
)

// Installs the v1 API routes that implement metrics configs generation from one-time request.
func installV1Routes(r *gin.Engine, server *Server) {
	v1 := r.Group("/api/v1")
	{
		v1.POST("/generate_metrics_config", handleCompile(constants.APIVersionV1))
		v1.POST("/validate_metrics_config", handleValidate(constants.APIVersionV1))
		v1.POST("/get_file_descriptor_set", handleFileDescriptors(constants.APIVersionV1))
		v1.GET("/version", handleVersionInfo)
		signal_versions.InstallRoutes(v1)

	}
}
