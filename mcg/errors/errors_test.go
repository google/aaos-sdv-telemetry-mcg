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

package errors_test

import (
	"testing"

	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"

	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
)

func getMainFlatError() *mcgerrors.StatusError {
	return &mcgerrors.StatusError{
		Status: &statuspb.Status{
			Code:    int32(code.Code_INVALID_ARGUMENT),
			Message: "SUPER_MSG",
		},
	}
}

func getBadRequestErrDetail(detail protoreflect.ProtoMessage) *errdetails.BadRequest {
	badReq, ok := detail.(*errdetails.BadRequest)
	if !ok {
		return nil
	}
	return badReq
}

func getDebugInfoErrDetail(detail protoreflect.ProtoMessage) *errdetails.DebugInfo {
	debugInfo, ok := detail.(*errdetails.DebugInfo)
	if !ok {
		return nil
	}
	return debugInfo
}

func TestFormatErrorToFlatErrorFormatsCorrectly(t *testing.T) {
	flatErr := getMainFlatError()
	errs := []*mcgerrors.StatusError{mcgerrors.InvalidArgument("MAIN_MSG").WithFieldViolation("FIELD", "DESC")}
	mcgerrors.FormatErrorDetailsToFlatError(flatErr, errs)

	fieldViol0 := getBadRequestErrDetail(flatErr.Details[0]).FieldViolations[0]
	if fieldViol0 == nil {
		t.Fatal("This should not happen as reading a `errdetails.BadRequest` from flatErr.Details[0] means that there's error in the test code logic.")
	}
	if got, want := fieldViol0.Field, "FIELD"; got != want {
		t.Fatalf("Flattening failed, got: %s, want: %s", got, want)
	}
	if got, want := fieldViol0.Description, "MAIN_MSG. Reason: DESC."; got != want {
		t.Fatalf("Flattening failed, got: %s, want: %s", got, want)
	}
}

func TestErrorWithoutDetailsToFlatErrorMessageFormatsCorrectly(t *testing.T) {
	flatErr := getMainFlatError()
	errs := []*mcgerrors.StatusError{mcgerrors.InvalidArgument("MAIN_MSG")}
	mcgerrors.FormatErrorDetailsToFlatError(flatErr, errs)

	errorDetail0 := getDebugInfoErrDetail(flatErr.Details[0])
	if got, want := errorDetail0.Detail, "MAIN_MSG"; got != want {
		t.Fatalf("Flattening failed, got: %s, want: %s", got, want)
	}
}

func TestErrorWithMultipleDetailsToFlatErrorMessageFormatsCorrectly(t *testing.T) {
	flatErr := getMainFlatError()
	errs := []*mcgerrors.StatusError{
		mcgerrors.InvalidArgument("MAIN_MSG1").WithFieldViolation("FIELD1", "DESC1").WithFieldViolation("FIELD2", "DESC2"),
		mcgerrors.InvalidArgument("MAIN_MSG2").WithFieldViolation("FIELD3", "DESC3").WithFieldViolation("FIELD4", "DESC4"),
	}
	mcgerrors.FormatErrorDetailsToFlatError(flatErr, errs)

	fieldViolations := getBadRequestErrDetail(flatErr.Details[0]).FieldViolations
	if len(fieldViolations) != 4 {
		t.Fatal("Flattening should have returned 4 field violations.")
	}

	for _, testCase := range []struct {
		viol            *errdetails.BadRequest_FieldViolation
		wantMain        string
		wantField       string
		wantDescription string
	}{
		{viol: fieldViolations[0], wantField: "FIELD1", wantDescription: "MAIN_MSG1. Reason: DESC1."},
		{viol: fieldViolations[1], wantField: "FIELD2", wantDescription: "MAIN_MSG1. Reason: DESC2."},
		{viol: fieldViolations[2], wantField: "FIELD3", wantDescription: "MAIN_MSG2. Reason: DESC3."},
		{viol: fieldViolations[3], wantField: "FIELD4", wantDescription: "MAIN_MSG2. Reason: DESC4."},
	} {
		if got := testCase.viol.Field; got != testCase.wantField {
			t.Fatalf("Flattening failed, got: %s, want: %s", got, testCase.wantField)
		}
		if got := testCase.viol.Description; got != testCase.wantDescription {
			t.Fatalf("Flattening failed, got: %s, want: %s", got, testCase.wantDescription)
		}
	}
}

func TestErrorWithMixedFieldViolationAndDebufInfoToFlatErrorMessageFormatsCorrectly(t *testing.T) {
	flatErr := getMainFlatError()
	errs := []*mcgerrors.StatusError{
		mcgerrors.InvalidArgument("MAIN_MSG1").WithFieldViolation("FIELD1", "DESC1").WithFieldViolation("FIELD2", "DESC2").WithDebugDetail("DEBUG1"),
		mcgerrors.InvalidArgument("MAIN_MSG2").WithFieldViolation("FIELD3", "DESC3").WithFieldViolation("FIELD4", "DESC4").WithDebugDetail("DEBUG2"),
	}
	mcgerrors.FormatErrorDetailsToFlatError(flatErr, errs)

	if len(flatErr.Details) != 3 {
		t.Fatal("Flattening should have returned 3 details, 1 of type bad request and 2 of type debug info.")
	}

	fieldViolations := getBadRequestErrDetail(flatErr.Details[0]).FieldViolations
	if len(fieldViolations) != 4 {
		t.Fatal("Flattening should have returned 4 field violations.")
	}

	for _, testCase := range []struct {
		viol            *errdetails.BadRequest_FieldViolation
		wantMain        string
		wantField       string
		wantDescription string
	}{
		{viol: fieldViolations[0], wantField: "FIELD1", wantDescription: "MAIN_MSG1. Reason: DESC1."},
		{viol: fieldViolations[1], wantField: "FIELD2", wantDescription: "MAIN_MSG1. Reason: DESC2."},
		{viol: fieldViolations[2], wantField: "FIELD3", wantDescription: "MAIN_MSG2. Reason: DESC3."},
		{viol: fieldViolations[3], wantField: "FIELD4", wantDescription: "MAIN_MSG2. Reason: DESC4."},
	} {
		if got := testCase.viol.Field; got != testCase.wantField {
			t.Fatalf("Flattening failed, got: %s, want: %s", got, testCase.wantField)
		}
		if got := testCase.viol.Description; got != testCase.wantDescription {
			t.Fatalf("Flattening failed, got: %s, want: %s", got, testCase.wantDescription)
		}
	}

	debugInfo1 := getDebugInfoErrDetail(flatErr.Details[1])
	if got, want := debugInfo1.Detail, "MAIN_MSG1. Reason: DEBUG1."; got != want {
		t.Fatalf("Flattening failed, got: %s, want: %s", got, want)
	}
	debugInfo2 := getDebugInfoErrDetail(flatErr.Details[2])
	if got, want := debugInfo2.Detail, "MAIN_MSG2. Reason: DEBUG2."; got != want {
		t.Fatalf("Flattening failed, got: %s, want: %s", got, want)
	}
}

func TestErrorWithDebugDetailsToFlatErrorMessageFormatsCorrectly(t *testing.T) {
	flatErr := getMainFlatError()
	errs := []*mcgerrors.StatusError{mcgerrors.InvalidArgument("MAIN_MSG").WithDebugDetail("DEBUG")}
	mcgerrors.FormatErrorDetailsToFlatError(flatErr, errs)

	errorDetail0 := getDebugInfoErrDetail(flatErr.Details[0])
	if got, want := errorDetail0.Detail, "MAIN_MSG. Reason: DEBUG."; got != want {
		t.Fatalf("Flattening failed, got: %s, want: %s", got, want)
	}
}
