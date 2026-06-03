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
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
	"sdv.googlesource.com/mcg/mcg/session"
	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

func msToDuration(ms float64) *durationpb.Duration {
	return durationpb.New(time.Duration(float64(time.Millisecond) * ms))
}

// PeriodicTriggerRequest specifies a periodic trigger.
type PeriodicTriggerRequest struct {
	PeriodMs float64  `json:"period_ms"`
	Triggers []string `json:"triggers,omitempty"`
	Count    *uint32  `json:"count,omitempty"`
}

// DataTriggerRequest specifies a data trigger.
type DataTriggerRequest struct {
	// Deprecated: Use Source instead.
	Publisher string `json:"publisher_name"`
	Source    string `json:"source_name"`
}

type EdgeOptionsRequest struct {
	MinDurationMs float64 `json:"min_duration_ms"`
	RequireExact  bool    `json:"require_exact"`
}

type RisingEdgeRequest struct {
	RisingOptions        *EdgeOptionsRequest `json:"rising_options,omitempty"`
	FireInitial          bool                `json:"fire_initial,omitempty"`
	InitializeExpression string              `json:"initialize_expression,omitempty"`
}

type FallingEdgeRequest struct {
	FallingOptions       *EdgeOptionsRequest `json:"falling_options,omitempty"`
	FireInitial          bool                `json:"fire_initial,omitempty"`
	InitializeExpression string              `json:"initialize_expression,omitempty"`
}

type AllChangesRequest struct {
	RisingOptions        *EdgeOptionsRequest `json:"rising_options,omitempty"`
	FallingOptions       *EdgeOptionsRequest `json:"falling_options,omitempty"`
	FireInitial          bool                `json:"fire_initial,omitempty"`
	InitializeExpression string              `json:"initialize_expression,omitempty"`
}

type ConditionTypeRequest struct {
	RisingEdge  *RisingEdgeRequest  `json:"rising_edge,omitempty"`
	FallingEdge *FallingEdgeRequest `json:"falling_edge,omitempty"`
	AllChanges  *AllChangesRequest  `json:"all_changes,omitempty"`
	IsTrue      *struct{}           `json:"is_true,omitempty"`
	IsFalse     *struct{}           `json:"is_false,omitempty"`
}

// ConditionalTriggerRequest specifies a conditional trigger.
type ConditionalTriggerRequest struct {
	// List of "parent" trigger names.
	Triggers      []string              `json:"triggers"`
	ConditionType *ConditionTypeRequest `json:"condition_type"`
	Expression    string                `json:"expression"`
}

// TriggerRequest specifies a trigger for create or update.
type TriggerRequest struct {
	Name        string                     `json:"name"`
	Periodic    *PeriodicTriggerRequest    `json:"periodic,omitempty"`
	Data        *DataTriggerRequest        `json:"data,omitempty"`
	Conditional *ConditionalTriggerRequest `json:"conditional,omitempty"`
}

func (req *TriggerRequest) isData() bool {
	return req.Data != nil
}

func (req *TriggerRequest) isPeriodic() bool {
	return req.Periodic != nil
}

func (req *TriggerRequest) isConditional() bool {
	return req.Conditional != nil
}

func (req *TriggerRequest) checkDoesNotHaveMultipleTriggers() *mcgerrors.StatusError {
	if (req.isConditional() && req.isData()) || (req.isData() && req.isPeriodic()) || (req.isPeriodic() && req.isConditional()) {
		e := mcgerrors.MultipleTriggers
		if req.isConditional() {
			e.WithFieldViolation("conditional", "only one format can be chosen")
		}
		if req.isData() {
			e.WithFieldViolation("data", "only one format can be chosen")
		}
		if req.isPeriodic() {
			e.WithFieldViolation("periodic", "only one format can be chosen")
		}
		return e
	}

	return nil
}

func parseEdgeOptions(e *EdgeOptionsRequest) *pb.ConditionalTrigger_EdgeOptions {
	if e == nil {
		return nil
	}

	return pb.ConditionalTrigger_EdgeOptions_builder{
		MinDuration:  msToDuration(e.MinDurationMs),
		RequireExact: e.RequireExact,
	}.Build()
}

// getConditionType returns the condition type of a conditional trigger, which can be either int32
// or string. Returns error in case the enum value is out of range for `ConditionType`.
func (ctreq *ConditionalTriggerRequest) parseConditionType(t *pb.ConditionalTrigger_builder, sess *session.Session) *mcgerrors.StatusError {
	if ctreq.ConditionType == nil {
		return mcgerrors.ConditionalTriggerConditionTypeInvalid(fmt.Errorf("field 'condition_type' is required for conditional triggers"))
	}

	foundConditionType := false

	if ctreq.ConditionType.AllChanges != nil {
		if foundConditionType {
			return mcgerrors.ConditionalTriggerConditionTypeInvalid(fmt.Errorf("more than one condition type specified"))
		}
		foundConditionType = true
		acBuilder := pb.ConditionalTrigger_ConditionTypeAllChanges_builder{
			RisingOptions:  parseEdgeOptions(ctreq.ConditionType.AllChanges.RisingOptions),
			FallingOptions: parseEdgeOptions(ctreq.ConditionType.AllChanges.FallingOptions),
			FireInitial:    ctreq.ConditionType.AllChanges.FireInitial,
		}
		if expr := ctreq.ConditionType.AllChanges.InitializeExpression; expr != "" {
			exprId := new(uint32)
			sess.ExprStash(exprId, expr)
			acBuilder.InitializeNodeIndex = exprId
		}
		t.AllChanges = acBuilder.Build()
	}
	if ctreq.ConditionType.RisingEdge != nil {
		if foundConditionType {
			return mcgerrors.ConditionalTriggerConditionTypeInvalid(fmt.Errorf("more than one condition type specified"))
		}
		foundConditionType = true
		reBuilder := pb.ConditionalTrigger_ConditionTypeRisingEdge_builder{
			RisingOptions: parseEdgeOptions(ctreq.ConditionType.RisingEdge.RisingOptions),
			FireInitial:   ctreq.ConditionType.RisingEdge.FireInitial,
		}
		if expr := ctreq.ConditionType.RisingEdge.InitializeExpression; expr != "" {
			exprId := new(uint32)
			sess.ExprStash(exprId, expr)
			reBuilder.InitializeNodeIndex = exprId
		}
		t.RisingEdge = reBuilder.Build()
	}
	if ctreq.ConditionType.FallingEdge != nil {
		if foundConditionType {
			return mcgerrors.ConditionalTriggerConditionTypeInvalid(fmt.Errorf("more than one condition type specified"))
		}
		foundConditionType = true
		feBuilder := pb.ConditionalTrigger_ConditionTypeFallingEdge_builder{
			FallingOptions: parseEdgeOptions(ctreq.ConditionType.FallingEdge.FallingOptions),
			FireInitial:    ctreq.ConditionType.FallingEdge.FireInitial,
		}
		if expr := ctreq.ConditionType.FallingEdge.InitializeExpression; expr != "" {
			exprId := new(uint32)
			sess.ExprStash(exprId, expr)
			feBuilder.InitializeNodeIndex = exprId
		}
		t.FallingEdge = feBuilder.Build()
	}
	if ctreq.ConditionType.IsTrue != nil {
		if foundConditionType {
			return mcgerrors.ConditionalTriggerConditionTypeInvalid(fmt.Errorf("more than one condition type specified"))
		}
		foundConditionType = true
		t.IsTrue = pb.ConditionalTrigger_ConditionTypeIsTrue_builder{}.Build()
	}
	if ctreq.ConditionType.IsFalse != nil {
		if foundConditionType {
			return mcgerrors.ConditionalTriggerConditionTypeInvalid(fmt.Errorf("more than one condition type specified"))
		}
		foundConditionType = true
		t.IsFalse = pb.ConditionalTrigger_ConditionTypeIsFalse_builder{}.Build()
	}

	if !foundConditionType {
		return mcgerrors.ConditionalTriggerConditionTypeInvalid(fmt.Errorf("no condition type specified"))
	}
	return nil
}

// Converts the request to a protobuf Trigger with potentially missing fields.
func (req *TriggerRequest) ToProto(sess *session.Session) (*pb.Trigger, *mcgerrors.StatusError) {
	if err := req.checkDoesNotHaveMultipleTriggers(); err != nil {
		return nil, err
	}

	switch true {
	case req.isConditional():
		ctBuilder := pb.ConditionalTrigger_builder{
			TriggerNames: req.Conditional.Triggers,
		}
		if err := req.Conditional.parseConditionType(&ctBuilder, sess); err != nil {
			return nil, err
		}

		exprId := new(uint32)
		sess.ExprStash(exprId, req.Conditional.Expression)
		ctBuilder.SelectorNodeIndex = exprId

		return pb.Trigger_builder{
			Name:               req.Name,
			ConditionalTrigger: ctBuilder.Build(),
		}.Build(), nil

	case req.isData():
		if req.Data.Publisher != "" && req.Data.Source != "" {
			return nil, mcgerrors.InvalidArgument("DataTriggerRequest: cannot specify both 'publisher_name' (deprecated) and 'source_name'. Use 'source_name'.")
		}
		sourceName := req.Data.Publisher
		if req.Data.Source != "" {
			sourceName = req.Data.Source
		}
		if sourceName == "" {
			return nil, mcgerrors.InvalidArgument("DataTriggerRequest: 'source_name' is required.")
		}
		return pb.Trigger_builder{
			Name: req.Name,
			DataTrigger: pb.DataTrigger_builder{
				SourceName: sourceName,
			}.Build(),
		}.Build(), nil

	case req.isPeriodic():
		ptBuilder := pb.PeriodicTrigger_builder{
			Interval: msToDuration(req.Periodic.PeriodMs),
		}
		if len(req.Periodic.Triggers) > 0 {
			ptBuilder.TriggerNames = req.Periodic.Triggers
		}
		if req.Periodic.Count != nil {
			ptBuilder.Count = req.Periodic.Count
		}
		return pb.Trigger_builder{
			Name:            req.Name,
			PeriodicTrigger: ptBuilder.Build(),
		}.Build(), nil

	default:
		return nil, mcgerrors.NoTriggerProvided
	}
}
