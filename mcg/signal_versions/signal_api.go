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

package signal_versions

import (
	"net/http"

	"github.com/gin-gonic/gin"

	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
	"sdv.googlesource.com/mcg/mcg/vs_cache"
)

// @Description	Vehicle Signal Catalog metadata
type VSCatalogVersionMetadata struct {
	// Vendor-defined identifier of the catalog
	Id string `json:"version"`
}

// @Description	A list of Vehicle Signal Catalog metadata
type VSCatalogVersionsMetadata struct {
	// List of Vehicle Signal Catalog metadata
	Items []VSCatalogVersionMetadata `json:"versions"`
}

// @Description	Vehicle Signal Catalog
type VSCatalogVersion struct {
	VSCatalogVersionMetadata

	// A `google.protobuf.FileDescriptorSet` protobuf message (base64-encoded) containing the Vehicle Signals
	FileDescriptorSet []byte `json:"vehicle_signals"`
}

func InstallRoutes(r *gin.RouterGroup) {
	vs := r.Group("/vs")
	{
		vs.GET("/", HandleSignalList)
		vs.POST("/", HandleSignalAdd)
		vs.DELETE("/:version", HandleSignalDelete)
	}
}

func HandleSignalAdd(c *gin.Context) {
	cache, err := vs_cache.GetCacheFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}

	var req VSCatalogVersion
	if err = c.BindJSON(&req); err != nil {
		c.Error(mcgerrors.InvalidArgument("Invalid request body"))
		return
	}
	if len(req.Id) == 0 {
		c.Error(mcgerrors.InvalidArgument("No version specified."))
		return
	}

	err = cache.Set(c, req.Id, req.FileDescriptorSet)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, VSCatalogVersionMetadata{Id: req.Id})
}

func HandleSignalDelete(c *gin.Context) {
	version := c.Param("version")
	if len(version) == 0 {
		c.Error(mcgerrors.InvalidArgument("No version specified."))
		return
	}

	cache, err := vs_cache.GetCacheFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}

	deleted, err := cache.Delete(c, version)
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}
	if deleted {
		c.JSON(http.StatusOK, VSCatalogVersionMetadata{Id: version})
	} else {
		c.Error(mcgerrors.NotFound("Vehicle Signals of the provided version not found."))
		return
	}
}

func HandleSignalList(c *gin.Context) {
	cache, err := vs_cache.GetCacheFromContext(c)
	if err != nil {
		c.Error(err)
		return
	}

	vsList, err := cache.List(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}

	items := make([]VSCatalogVersionMetadata, len(vsList))
	for i, v := range vsList {
		items[i].Id = v
	}
	c.JSON(http.StatusOK, VSCatalogVersionsMetadata{Items: items})
}
