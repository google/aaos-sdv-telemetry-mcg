// Copyright 2026 Google LLC
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
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"sdv.googlesource.com/mcg/mcg/constants"
	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
	"sdv.googlesource.com/mcg/mcg/mcuuid"
	"sdv.googlesource.com/mcg/mcg/requests"
	"sdv.googlesource.com/mcg/mcg/validators"
)

func handleCompile(apiVersion constants.APIVersion) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req requests.MetricsConfigRequest
		if !checkBind(c, &req) {
			return
		}

		render, statusErr := chooseRenderFunc(c, apiVersion)
		if statusErr != nil {
			c.Error(statusErr)
			return
		}

		if errorList := req.ValidateSchemaConsistency(apiVersion); len(errorList) > 0 {
			c.JSON(http.StatusBadRequest, mcgerrors.FlattenErrorList(code.Code_INVALID_ARGUMENT, "Parsing of request failed", errorList))
			return
		}

		sess, errorList := req.ToSession(c)
		if len(errorList) > 0 {
			c.JSON(http.StatusBadRequest, mcgerrors.FlattenErrorList(code.Code_INVALID_ARGUMENT, "Parsing of request failed", errorList))
			return
		}

		var uuid mcuuid.MCUUID
		if req.ExistingUUID != nil {
			uuid = *req.ExistingUUID
		} else {
			var err error
			uuid, err = mcuuid.NewRandom()
			if err != nil {
				c.Error(mcgerrors.InternalFromError(err))
				return
			}
		}
		sess.ConfigUUID = uuid

		shouldIgnore, statusErr := getIgnoreValidationQueryParamValue(c)
		if statusErr != nil {
			c.Error(statusErr)
			return
		}
		sess.IgnoreValidations = shouldIgnore

		noInference, statusErr := getNoInferenceQueryParamValue(c)
		if statusErr != nil {
			c.Error(statusErr)
			return
		}
		sess.NoMessageInference = noInference

		mc, errorList, errorMessage := compileSession(sess)
		if len(errorList) > 0 {
			c.JSON(http.StatusBadRequest, mcgerrors.FlattenErrorList(code.Code_INVALID_ARGUMENT, errorMessage, errorList))
			return
		}

		err := addMcSizeResponseHeader(mc, c)
		if err != nil {
			c.Error(mcgerrors.InvalidArgumentFromError(err))
			return
		}

		render(c, mc)
	}
}

func handleValidate(apiVersion constants.APIVersion) gin.HandlerFunc {
	return func(c *gin.Context) {
		reqBody, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Error(err)
			return
		}

		mc, err := parseMetricsConfig(reqBody, c.ContentType(), apiVersion)
		if err != nil {
			c.Error(err)
			return
		}

		errorList := validators.ValidateWithShallowValidations(mc)
		if len(errorList) > 0 {
			c.JSON(http.StatusBadRequest, mcgerrors.FlattenErrorList(code.Code_INVALID_ARGUMENT, "Metrics config validation failed.", errorList))
			return
		}

		returnConfig, statusErr := getReturnConfigQueryParamValue(c)
		if statusErr != nil {
			c.Error(statusErr)
			return
		}

		if err := addMcSizeResponseHeader(mc, c); err != nil {
			c.Error(mcgerrors.InvalidArgumentFromError(err))
			return
		}

		if !returnConfig {
			c.JSON(http.StatusOK, gin.H{})
			return
		}

		render, statusErr := chooseRenderFunc(c, apiVersion)
		if statusErr != nil {
			c.Error(statusErr)
			return
		}
		render(c, mc)
	}
}

func handleFileDescriptors(apiVersion constants.APIVersion) gin.HandlerFunc {
	return func(c *gin.Context) {
		reqBody, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Error(err)
			return
		}

		mc, err := parseMetricsConfig(reqBody, c.ContentType(), apiVersion)
		if err != nil {
			c.Error(err)
			return
		}

		fds := &descriptorpb.FileDescriptorSet{
			File: mc.GetDescriptorProtos(),
		}

		switch c.NegotiateFormat(ContentTypeBinaryProtoMetricsConfig, ContentTypeTextProtoMetricsConfig) {
		case ContentTypeBinaryProtoMetricsConfig:
			by, err := proto.Marshal(fds)
			if err != nil {
				c.Error(mcgerrors.InternalFromError(err))
				return
			}
			c.Data(http.StatusOK, ContentTypeBinaryProtoMetricsConfig, by)
		case ContentTypeTextProtoMetricsConfig:
			by, err := textprotoMarshal(fds, false)
			if err != nil {
				c.Error(mcgerrors.InternalFromError(err))
				return
			}
			c.Data(http.StatusOK, ContentTypeTextProtoMetricsConfig, by)

		default:
			c.Error(mcgerrors.InvalidArgument(fmt.Sprintf("Requested `Content-Type` not supported, use %q or %q in the `Accept` header.", MediaTypeBinaryProto, MediaTypeTextProto)))
		}
	}
}

func handleVersionInfo(c *gin.Context) {
	c.JSON(http.StatusOK, VersionInfo{
		ServiceVersion: VERSION,
	})
}
