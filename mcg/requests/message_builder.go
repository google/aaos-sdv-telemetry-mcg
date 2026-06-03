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

package requests

import (
	"fmt"

	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
	"sdv.googlesource.com/mcg/mcg/session"
	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

const (
	AggregationTypeAvg    = "avg"
	AggregationTypeCount  = "count"
	AggregationTypeMin    = "min"
	AggregationTypeMax    = "max"
	AggregationTypeNone   = "none"
	AggregationTypeVector = "vector"
	// v0.4.0
	AggregationTypeDelta  = "delta"
	AggregationTypeStdDev = "stddev"
	AggregationTypeSum    = "sum"
)

type MessageBuilderRequest struct {
	// If message_type is omitted, the message type inference feature will be used.
	MessageType      string                   `json:"message_type,omitempty"`
	FieldAssignments []FieldAssignmentRequest `json:"field_assignments"`
}

type AggregationRequest struct {
	Type       string `json:"@type"`
	Expression string `json:"expression,omitempty"`
	// Only valid for `@type=vector`
	MaxLength *uint32 `json:"max_length,omitempty"`
}

type FieldAssignmentRequest struct {
	FieldName string `json:"field_name"`
	// When message type inference is used, this field can be used to override the results of expression type inference.
	UserFieldType string             `json:"field_type,omitempty"`
	Aggregation   AggregationRequest `json:"aggregation"`
}

func (req *FieldAssignmentRequest) validate() *mcgerrors.StatusError {
	if req.Aggregation.Type != AggregationTypeVector && req.Aggregation.MaxLength != nil {
		return mcgerrors.InvalidArgument("Non-vector aggregation must not have max_length").WithFieldViolation(
			fmt.Sprintf("field_assignments[%s].aggregation.max_length", req.FieldName), "present")
	}

	if req.Aggregation.Type == "" {
		return mcgerrors.InvalidArgument("Field assignment aggregation missing").WithFieldViolation(
			fmt.Sprintf("field_assignments[%s].aggregation", req.FieldName), "missing")
	}

	// All the other aggregation types but "count" should have the expression field set.
	if req.Aggregation.Type == AggregationTypeCount {
		if req.Aggregation.Expression != "" {
			return mcgerrors.InvalidArgument("Count aggregation must not have expression").WithFieldViolation(
				fmt.Sprintf("field_assignments[%s].aggregation.expression", req.FieldName), "present")
		}
	} else {
		if req.Aggregation.Expression == "" {
			return mcgerrors.InvalidArgument("Aggregation missing an expression").WithFieldViolation(
				fmt.Sprintf("field_assignments[%s].aggregation.expression", req.FieldName), "missing")
		}
	}

	return nil
}

func (req *FieldAssignmentRequest) toProto(path *session.MessageBuilderLocation, session *session.Session) (*pb.ProtoMessageBuilder_FieldAssignment, *mcgerrors.StatusError) {
	if err := req.validate(); err != nil {
		return nil, err
	}

	fieldAss := pb.ProtoMessageBuilder_FieldAssignment_builder{
		FieldName: req.FieldName,
	}.Build()

	expr := new(uint32)
	if req.Aggregation.Expression != "" {
		session.ExprStash(expr, req.Aggregation.Expression)
	}

	switch req.Aggregation.Type {
	// go/keep-sorted start
	case AggregationTypeAvg:
		fieldAss.SetAvgAggregation(pb.ProtoMessageBuilder_FieldAssignment_AvgAggregation_builder{
			ExpressionNodeIndex: expr,
		}.Build())
	case AggregationTypeCount:
		fieldAss.SetCountAggregation(pb.ProtoMessageBuilder_FieldAssignment_CountAggregation_builder{}.Build())
	case AggregationTypeDelta:
		fieldAss.SetDeltaAggregation(pb.ProtoMessageBuilder_FieldAssignment_DeltaAggregation_builder{
			ExpressionNodeIndex: expr,
		}.Build())
	case AggregationTypeMax:
		fieldAss.SetMaxAggregation(pb.ProtoMessageBuilder_FieldAssignment_MaxAggregation_builder{
			ExpressionNodeIndex: expr,
		}.Build())
	case AggregationTypeMin:
		fieldAss.SetMinAggregation(pb.ProtoMessageBuilder_FieldAssignment_MinAggregation_builder{
			ExpressionNodeIndex: expr,
		}.Build())
	case AggregationTypeNone:
		fieldAss.SetNoAggregation(pb.ProtoMessageBuilder_FieldAssignment_NoAggregation_builder{
			ExpressionNodeIndex: expr,
		}.Build())
	case AggregationTypeStdDev:
		fieldAss.SetStdDevAggregation(pb.ProtoMessageBuilder_FieldAssignment_StdDevAggregation_builder{
			ExpressionNodeIndex: expr,
		}.Build())
	case AggregationTypeSum:
		fieldAss.SetSumAggregation(pb.ProtoMessageBuilder_FieldAssignment_SumAggregation_builder{
			ExpressionNodeIndex: expr,
		}.Build())
	case AggregationTypeVector:
		fieldAss.SetVectorAggregation(pb.ProtoMessageBuilder_FieldAssignment_VectorAggregation_builder{
			ExpressionNodeIndex: expr,
			MaxLength:           req.Aggregation.MaxLength,
		}.Build())
	// go/keep-sorted end
	default:
		return nil, mcgerrors.InvalidArgument(fmt.Sprintf("Unrecognized aggregation type %q", req.Aggregation.Type))
	}

	return fieldAss, nil
}

func (req *MessageBuilderRequest) validateFieldAssignmentRequest(fieldAssReq *FieldAssignmentRequest) *mcgerrors.StatusError {
	if fieldAssReq.UserFieldType != "" && req.MessageType != "" {
		return mcgerrors.InvalidArgument("Cannot specify both field_assignment[*].field_type and message_builder.message_type")
	}
	return nil
}

func (req *MessageBuilderRequest) toProto(path session.MessageBuilderLocation, session *session.Session) (*pb.ProtoMessageBuilder, *mcgerrors.StatusError) {
	protoMsgBuilder := pb.ProtoMessageBuilder_builder{
		MessageType: req.MessageType,
	}.Build()

	for _, fieldAssReq := range req.FieldAssignments {
		if fieldAssReq.UserFieldType != "" {
			session.SaveFieldType(path.WithFieldName(fieldAssReq.FieldName), fieldAssReq.UserFieldType)
		}
		if err := req.validateFieldAssignmentRequest(&fieldAssReq); err != nil {
			return nil, err
		}

		fieldAss, err := fieldAssReq.toProto(&path, session)
		if err != nil {
			return nil, err
		}

		protoMsgBuilder.SetFieldAssignments(append(protoMsgBuilder.GetFieldAssignments(), fieldAss))
	}
	return protoMsgBuilder, nil
}
