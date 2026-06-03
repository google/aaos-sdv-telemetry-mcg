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

package errors

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"

	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
)

var canonicalOkStatus *StatusError = &StatusError{Status: &statuspb.Status{
	Code: int32(code.Code_OK),
}}

func OK() *StatusError {
	return canonicalOkStatus
}

func AlreadyExists(msg string) *StatusError {
	return &StatusError{Status: &statuspb.Status{
		Code:    int32(code.Code_ALREADY_EXISTS),
		Message: msg,
	}}
}

func FailedPrecondition(msg string) *StatusError {
	return &StatusError{Status: &statuspb.Status{
		Code:    int32(code.Code_FAILED_PRECONDITION),
		Message: msg,
	}}
}

func InvalidArgument(msg string) *StatusError {
	return &StatusError{Status: &statuspb.Status{
		Code:    int32(code.Code_INVALID_ARGUMENT),
		Message: msg,
	}}
}

func Internal(msg string) *StatusError {
	return &StatusError{Status: &statuspb.Status{
		Code:    int32(code.Code_INTERNAL),
		Message: msg,
	}}
}

func NotFound(msg string) *StatusError {
	return &StatusError{Status: &statuspb.Status{
		Code:    int32(code.Code_NOT_FOUND),
		Message: msg,
	}}
}

func Unimplemented(msg string) *StatusError {
	return &StatusError{Status: &statuspb.Status{
		Code:    int32(code.Code_UNIMPLEMENTED),
		Message: msg,
	}}
}

func InternalFromError(err error) *StatusError {
	if sErr, ok := err.(*StatusError); ok {
		return sErr
	}
	return &StatusError{Status: &statuspb.Status{
		Code:    int32(code.Code_INTERNAL),
		Message: err.Error(),
	}}
}

func InvalidArgumentFromError(err error) *StatusError {
	if sErr, ok := err.(*StatusError); ok {
		return sErr
	}
	return &StatusError{Status: &statuspb.Status{
		Code:    int32(code.Code_INVALID_ARGUMENT),
		Message: err.Error(),
	}}
}

func InternalFromPanic(err any) *StatusError {
	status := &StatusError{Status: &statuspb.Status{
		Code:    int32(code.Code_INTERNAL),
		Message: "Internal server error (panic)",
	}}
	by := debug.Stack()
	status.Details = append(status.Details, &errdetails.DebugInfo{
		StackEntries: strings.Split(string(by), "\n"),
		Detail:       fmt.Sprintf("%#v", err),
	})
	return status
}

// WithFieldViolation augments an InvalidArgument status with the BadRequest proto.
func (s *StatusError) WithFieldViolation(path, message string) *StatusError {
	var fv *errdetails.BadRequest
	for _, v := range s.Details {
		if br, ok := v.(*errdetails.BadRequest); ok {
			fv = br
		}
	}
	if fv == nil {
		fv = &errdetails.BadRequest{}
		s.Details = append(s.Details, fv)
	}
	fv.FieldViolations = append(fv.FieldViolations, &errdetails.BadRequest_FieldViolation{
		Field:       path,
		Description: message,
	})
	return s
}

func (s *StatusError) WithDebugDetail(extraMessage string) *StatusError {
	di := &errdetails.DebugInfo{
		Detail: extraMessage,
	}
	s.Details = append(s.Details, di)
	return s
}

func CodeToHTTP(c code.Code) int32 {
	switch c {
	// go/keep-sorted start
	case code.Code_ABORTED:
		return http.StatusConflict
	case code.Code_ALREADY_EXISTS:
		return http.StatusConflict
	case code.Code_CANCELLED:
		return 499 // Note: Client should never see this, but our monitoring will
	case code.Code_DATA_LOSS:
		return http.StatusInternalServerError
	case code.Code_DEADLINE_EXCEEDED:
		return http.StatusGatewayTimeout
	case code.Code_FAILED_PRECONDITION:
		return http.StatusBadRequest
	case code.Code_INTERNAL:
		return http.StatusInternalServerError
	case code.Code_INVALID_ARGUMENT:
		return http.StatusBadRequest
	case code.Code_NOT_FOUND:
		return http.StatusNotFound
	case code.Code_OK:
		return http.StatusOK
	case code.Code_OUT_OF_RANGE:
		return http.StatusBadRequest
	case code.Code_PERMISSION_DENIED:
		return http.StatusForbidden
	case code.Code_RESOURCE_EXHAUSTED:
		return http.StatusTooManyRequests
	case code.Code_UNAUTHENTICATED:
		return http.StatusUnauthorized
	case code.Code_UNAVAILABLE:
		return http.StatusServiceUnavailable
	case code.Code_UNIMPLEMENTED:
		return http.StatusNotImplemented
	case code.Code_UNKNOWN:
		return http.StatusInternalServerError
	// go/keep-sorted end
	default:
		panic("corrupt status code")
	}
}
