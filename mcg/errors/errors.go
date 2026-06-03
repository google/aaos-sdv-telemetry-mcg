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

package errors

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// StatusError is the primary type placed into gin.Context.Errors.
//
// It contains a canonical RPC status code and an arbitrary number of extensions.
type StatusError struct {
	Status  *statuspb.Status
	Details []proto.Message
}

func (s *StatusError) Error() string {
	return fmt.Sprintf("%s: %s", code.Code(s.Status.Code).String(), s.Status.Message)
}

func (s *StatusError) asProto() *statuspb.Status {
	// Drain the Details array into the proto
	for _, v := range s.Details {
		dst := new(anypb.Any)
		anypb.MarshalFrom(dst, v, proto.MarshalOptions{})
		s.Status.Details = append(s.Status.Details, dst)
	}
	s.Details = nil
	return s.Status
}

func (s *StatusError) asJSON() *ErrorBody {
	code := code.Code(s.Status.Code)
	// Call asProto for the anypb marshaling side effect.
	s.asProto()
	payload := ErrorBody_builder{
		Error: ErrorBody_Status_builder{
			Status:  code,
			Code:    CodeToHTTP(code),
			Message: s.Status.Message,
			Details: s.Status.Details,
		}.Build(),
	}.Build()

	return payload
}

func FormatErrorMsg(message, description string) string {
	return fmt.Sprintf("%s. Reason: %s.", message, description)
}

// formatErrorDetailsToFlatError converts the error messages so that the primary error message is
// combined with any complementary information from the description field. Other fields remain as
// is. If there are several details (in the case of `errdetails.BadRequest`) per primary error, then
// there will be several "flat details" added to the primary flattenedError.
func formatErrorDetailsToFlatError(flattenedError *StatusError, errorList []*StatusError) {
	for _, err := range errorList {
		if len(err.Details) == 0 {
			// Only has the main message, let's "downgrade" that to a detail for the given flattenedError.
			flattenedError.WithDebugDetail(err.Status.Message)
			continue
		}

		for _, detail := range err.Details {
			switch det := detail.(type) {
			case *errdetails.BadRequest:
				for _, viol := range det.FieldViolations {
					flattenedError.WithFieldViolation(viol.Field, FormatErrorMsg(err.Status.Message, viol.Description))
				}
			case *errdetails.DebugInfo:
				flattenedError.WithDebugDetail(fmt.Sprintf("%s. Reason: %s.", err.Status.Message, det.Detail))
			default:
				// This must be a new type that's neither BadRequest or DebugInfo so defaulting to just
				// adding the main error message.
				flattenedError.WithDebugDetail(err.Status.Message)
			}
		}
	}
}

// FlattenErrorList flattens all the so far seen errors into one error where all the previously seen
// errors are shown in the list of details related to the main error.
func FlattenErrorList(mainErrorCode code.Code, mainErrorMsg string, errorList []*StatusError) json.RawMessage {
	flattenedError := &StatusError{
		Status: &statuspb.Status{
			Code:    int32(mainErrorCode),
			Message: mainErrorMsg,
		},
	}
	formatErrorDetailsToFlatError(flattenedError, errorList)

	errorBody := flattenedError.asJSON()
	errorBytes, err := protojson.MarshalOptions{}.Marshal(errorBody)
	if err != nil {
		log.Default().Println("Failed to report error:", err)
		return nil
	}
	return json.RawMessage(errorBytes)
}

func MiddlewareRenderErrors(c *gin.Context) {
	c.Next()

	// Detect cancelled requests and inject an error (this error will never reach the client).
	isCancelled := false
	select {
	case <-c.Request.Context().Done():
		isCancelled = true
	default:
	}
	if isCancelled {
		c.Errors = nil
		status := &StatusError{Status: &statuspb.Status{
			Code:    int32(code.Code_CANCELLED),
			Message: "request cancelled",
		}}
		c.Error(status)
	}

	if c.Writer.Written() {
		if c.Writer.Size() == 0 && c.Writer.Status() >= http.StatusBadRequest {
			// Continue to write the error. This can happen if c.Bind fails.
		} else {
			return
		}
	} else if len(c.Errors) == 0 && c.Writer.Status() != http.StatusOK {
		return
	}

	var payload *ErrorBody

	for _, v := range c.Errors {
		if vStatus, ok := v.Err.(*StatusError); ok {
			payload = vStatus.asJSON()
			break
		} else if v.Type&gin.ErrorTypeBind != 0 {
			contentTypeName := binding.Default(c.Request.Method, c.ContentType()).Name()
			status := InvalidArgument(fmt.Sprintf("Unable to parse request body as %s: %v", contentTypeName, v.Err))
			payload = status.asJSON()
			break
		} else if v.Type&gin.ErrorTypeRender != 0 {
			status := Internal("Internal error while rendering output")
			payload = status.asJSON()
			break
		}
	}
	if payload == nil {
		// Try again, not ignoring other errors, but representing them as UNKNOWN
		cummError := &StatusError{
			Status: &statuspb.Status{
				Code:    int32(code.Code_UNKNOWN),
				Message: "error message missing",
			},
		}
		for _, v := range c.Errors {
			if cummError.Status.Message == "" {
				cummError.Status.Message = v.Error()
			} else {
				cummError = cummError.WithDebugDetail(v.Error())
			}
		}
		payload = cummError.asJSON()
	}

	if payload == nil {
		c.String(http.StatusInternalServerError, "reached end of request handler without writing anything")
		return
	}
	by, err := protojson.MarshalOptions{}.Marshal(payload)
	if err != nil {
		// we are the error reporting code. abandon
		log.Default().Println("failed to report error:", err)
		return
	}
	c.JSON(int(payload.GetError().GetCode()), json.RawMessage(by))
}

// JsonErrorResponse is used by tests to parse a protojson ErrorBody.
//
// For documentation and tests only.
type JsonErrorResponse struct {
	Error struct {
		// HTTP status code
		Code int32 `json:"code" example:"403"`
		// Google RPC canonical status code
		Status string `json:"status" enums:"OK,CANCELLED,UNKNOWN,INVALID_ARGUMENT,DEADLINE_EXCEEDED,NOT_FOUND,ALREADY_EXISTS,PERMISSION_DENIED,UNAUTHENTICATED,RESOURCE_EXHAUSTED,FAILED_PRECONDITION,ABORTED,OUT_OF_RANGE,UNIMPLEMENTED,INTERNAL,UNAVAILABLE,DATA_LOSS"`
		// Developer-facing message in English
		Message string `json:"message"`
		// Array of arbitrary objects identified by their @type member
		Details []json.RawMessage `json:"details"`
	} `json:"error"`
}

// JsonErrorResponseFromBytes converts bytes array into a parsed JsonErrorResponse.
//
// For tests only.
func JsonErrorResponseFromBytes(bts []byte) (*JsonErrorResponse, error) {
	var statusErr JsonErrorResponse
	err := json.Unmarshal(bts, &statusErr)
	if err != nil {
		return nil, err
	}
	return &statusErr, nil
}

// StatusCode converts error.Status to a numeric status code.
//
// For tests only.
func (er *JsonErrorResponse) StatusCode() code.Code {
	if er.Error.Status == "" {
		return code.Code_OK
	}
	c, ok := code.Code_value[er.Error.Status]
	if !ok {
		return code.Code_UNKNOWN
	}
	return code.Code(c)
}

// BadRequestFromDetail converts *json.RawMessage from an index of JsonErrorResponse.Error.Details
// into a parsed errdetails.BadRequest.
//
// For tests only.
func (er *JsonErrorResponse) BadRequestFromDetail(idx int) (*errdetails.BadRequest, error) {
	var errDet errdetails.BadRequest
	err := protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(er.Error.Details[idx], &errDet)
	if err != nil {
		return nil, err
	}
	return &errDet, nil
}
