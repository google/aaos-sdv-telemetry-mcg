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

package requests_test

import (
	"testing"

	"sdv.googlesource.com/mcg/mcg/requests"
)

func TestValidFieldAssignmentRequestValidationsPass(t *testing.T) {
	maxLen := uint32(5)

	for _, val := range []requests.AggregationRequest{
		{Type: requests.AggregationTypeCount},
		{Type: requests.AggregationTypeAvg, Expression: "1"},
		{Type: requests.AggregationTypeMin, Expression: "2"},
		{Type: requests.AggregationTypeMax, Expression: "3"},
		{Type: requests.AggregationTypeNone, Expression: "4"},
		{Type: requests.AggregationTypeDelta, Expression: "5"},
		{Type: requests.AggregationTypeStdDev, Expression: "6"},
		{Type: requests.AggregationTypeSum, Expression: "7"},
		{Type: requests.AggregationTypeVector, Expression: "8"},
		{Type: requests.AggregationTypeVector, Expression: "9", MaxLength: &maxLen},
	} {
		req := &requests.FieldAssignmentRequest{
			FieldName:     "FieldName",
			UserFieldType: "UserFieldType",
			Aggregation:   val,
		}
		if err := requests.Validate(req); err != nil {
			t.Fatal(err)
		}
	}
}

func TestValidatingInvalidExpressionsInFieldAssignmentRequestsFail(t *testing.T) {
	type TestCase struct {
		aggReq      requests.AggregationRequest
		expectedErr string
	}

	missingExpressionErrMsg := "Aggregation missing an expression"
	for _, val := range []TestCase{
		{
			aggReq:      requests.AggregationRequest{Type: requests.AggregationTypeCount, Expression: "1"},
			expectedErr: "Count aggregation must not have expression",
		},
		{
			aggReq:      requests.AggregationRequest{Type: requests.AggregationTypeAvg},
			expectedErr: missingExpressionErrMsg,
		},
		{
			aggReq:      requests.AggregationRequest{Type: requests.AggregationTypeMin},
			expectedErr: missingExpressionErrMsg,
		},
		{
			aggReq:      requests.AggregationRequest{Type: requests.AggregationTypeMax},
			expectedErr: missingExpressionErrMsg,
		},
		{
			aggReq:      requests.AggregationRequest{Type: requests.AggregationTypeNone},
			expectedErr: missingExpressionErrMsg,
		},
		{
			aggReq:      requests.AggregationRequest{Type: requests.AggregationTypeDelta},
			expectedErr: missingExpressionErrMsg,
		},
		{
			aggReq:      requests.AggregationRequest{Type: requests.AggregationTypeStdDev},
			expectedErr: missingExpressionErrMsg,
		},
		{
			aggReq:      requests.AggregationRequest{Type: requests.AggregationTypeSum},
			expectedErr: missingExpressionErrMsg,
		},
	} {
		req := &requests.FieldAssignmentRequest{
			FieldName:     "FieldName",
			UserFieldType: "UserFieldType",
			Aggregation:   val.aggReq,
		}
		if err := requests.Validate(req); val.expectedErr != err.Status.Message {
			t.Errorf("With aggregation_type %s wanted %s, but got %s", val.aggReq.Type, val.expectedErr, err.Status.Message)
		}
	}
}

func TestValidatingInvalidMaxLengthsInFieldAssignmentRequestsFail(t *testing.T) {
	maxLen := uint32(5)
	want := "Non-vector aggregation must not have max_length"

	for _, val := range []requests.AggregationRequest{
		{Type: requests.AggregationTypeCount, MaxLength: &maxLen},
		{Type: requests.AggregationTypeAvg, Expression: "1", MaxLength: &maxLen},
		{Type: requests.AggregationTypeMin, Expression: "1", MaxLength: &maxLen},
		{Type: requests.AggregationTypeMax, Expression: "1", MaxLength: &maxLen},
		{Type: requests.AggregationTypeNone, Expression: "1", MaxLength: &maxLen},
		{Type: requests.AggregationTypeDelta, Expression: "1", MaxLength: &maxLen},
		{Type: requests.AggregationTypeStdDev, Expression: "1", MaxLength: &maxLen},
		{Type: requests.AggregationTypeSum, Expression: "1", MaxLength: &maxLen},
	} {
		req := &requests.FieldAssignmentRequest{
			FieldName:     "FieldName",
			UserFieldType: "UserFieldType",
			Aggregation:   val,
		}
		if err := requests.Validate(req); want != err.Status.Message {
			t.Errorf("With aggregation_type %s wanted %s, but got %s", val.Type, want, err.Status.Message)
		}
	}
}

func TestValidatingEmptyAggregationTypeInFieldAssignmentRequestsFail(t *testing.T) {
	req := &requests.FieldAssignmentRequest{
		FieldName:     "FieldName",
		UserFieldType: "UserFieldType",
		Aggregation:   requests.AggregationRequest{},
	}

	want := "Field assignment aggregation missing"
	if err := requests.Validate(req); want != err.Status.Message {
		t.Errorf("wanted %s, but got %s", want, err.Status.Message)
	}
}

func TestValidateFieldAssignmentRequestWithMessageTypeDefinedPasses(t *testing.T) {
	req := &requests.MessageBuilderRequest{
		MessageType: "MessageType",
		FieldAssignments: []requests.FieldAssignmentRequest{
			{
				FieldName:   "FieldName",
				Aggregation: requests.AggregationRequest{},
			},
		},
	}

	if err := requests.ValidateFieldAssignmentRequest(req, &req.FieldAssignments[0]); err != nil {
		t.Fatal(err)
	}
}

func TestValidateFieldAssignmentRequestWithUserFieldTypeDefinedPasses(t *testing.T) {
	req := &requests.MessageBuilderRequest{
		FieldAssignments: []requests.FieldAssignmentRequest{
			{
				FieldName:     "FieldName",
				UserFieldType: "UserFieldType",
				Aggregation:   requests.AggregationRequest{},
			},
		},
	}

	if err := requests.ValidateFieldAssignmentRequest(req, &req.FieldAssignments[0]); err != nil {
		t.Fatal(err)
	}
}

func TestValidateFieldAssignmentRequestWithBothMessageTypeAndUserFieldTypeDefinedFails(t *testing.T) {
	req := &requests.MessageBuilderRequest{
		MessageType: "MessageType",
		FieldAssignments: []requests.FieldAssignmentRequest{
			{
				FieldName:     "FieldName",
				UserFieldType: "UserFieldType",
				Aggregation:   requests.AggregationRequest{},
			},
		},
	}

	want := "Cannot specify both field_assignment[*].field_type and message_builder.message_type"
	if err := requests.ValidateFieldAssignmentRequest(req, &req.FieldAssignments[0]); want != err.Status.Message {
		t.Errorf("wanted %s, but got %s", want, err.Status.Message)
	}
}
