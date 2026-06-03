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
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"

	"sdv.googlesource.com/mcg/mcg/expressions"
	"sdv.googlesource.com/mcg/mcg/requests"
	"sdv.googlesource.com/mcg/mcg/session"
	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

func TestParseConditionType(t *testing.T) {
	testCases := []struct {
		name        string
		got         requests.ConditionTypeRequest
		wantBuilder func(*pb.ConditionalTrigger_builder)
	}{
		{
			name: "allChanges",
			got: requests.ConditionTypeRequest{
				AllChanges: &requests.AllChangesRequest{},
			},
			wantBuilder: func(ctb *pb.ConditionalTrigger_builder) {
				ctb.AllChanges = pb.ConditionalTrigger_ConditionTypeAllChanges_builder{}.Build()
			},
		},
		{
			name: "allChangesWithEdgeoptions",
			got: requests.ConditionTypeRequest{
				AllChanges: &requests.AllChangesRequest{
					RisingOptions: &requests.EdgeOptionsRequest{
						MinDurationMs: 123,
						RequireExact:  false,
					},
					FallingOptions: &requests.EdgeOptionsRequest{
						MinDurationMs: 456,
						RequireExact:  true,
					},
				},
			},
			wantBuilder: func(ctb *pb.ConditionalTrigger_builder) {
				ctb.AllChanges = pb.ConditionalTrigger_ConditionTypeAllChanges_builder{
					RisingOptions: pb.ConditionalTrigger_EdgeOptions_builder{
						MinDuration:  durationpb.New(time.Duration(123 * time.Millisecond)),
						RequireExact: false,
					}.Build(),
					FallingOptions: pb.ConditionalTrigger_EdgeOptions_builder{
						MinDuration:  durationpb.New(time.Duration(456 * time.Millisecond)),
						RequireExact: true,
					}.Build(),
				}.Build()
			},
		},
		{
			name: "allChangesWithFireInitial",
			got: requests.ConditionTypeRequest{
				AllChanges: &requests.AllChangesRequest{
					FireInitial: true,
				},
			},
			wantBuilder: func(ctb *pb.ConditionalTrigger_builder) {
				ctb.AllChanges = pb.ConditionalTrigger_ConditionTypeAllChanges_builder{
					FireInitial: true,
				}.Build()
			},
		},
		{
			name: "allChangesWithInitializeExpression",
			got: requests.ConditionTypeRequest{
				AllChanges: &requests.AllChangesRequest{
					InitializeExpression: "expr1",
				},
			},
			wantBuilder: func(ctb *pb.ConditionalTrigger_builder) {
				exprId := uint32(0)
				ctb.AllChanges = pb.ConditionalTrigger_ConditionTypeAllChanges_builder{
					InitializeNodeIndex: &exprId,
				}.Build()
			},
		},
		{
			name: "risingEdge",
			got: requests.ConditionTypeRequest{
				RisingEdge: &requests.RisingEdgeRequest{},
			},
			wantBuilder: func(ctb *pb.ConditionalTrigger_builder) {
				ctb.RisingEdge = pb.ConditionalTrigger_ConditionTypeRisingEdge_builder{}.Build()
			},
		},
		{
			name: "risingEdgeWithEdgeoptions",
			got: requests.ConditionTypeRequest{
				RisingEdge: &requests.RisingEdgeRequest{
					RisingOptions: &requests.EdgeOptionsRequest{
						MinDurationMs: 123,
						RequireExact:  false,
					},
				},
			},
			wantBuilder: func(ctb *pb.ConditionalTrigger_builder) {
				ctb.RisingEdge = pb.ConditionalTrigger_ConditionTypeRisingEdge_builder{
					RisingOptions: pb.ConditionalTrigger_EdgeOptions_builder{
						MinDuration:  durationpb.New(time.Duration(123 * time.Millisecond)),
						RequireExact: false,
					}.Build(),
				}.Build()
			},
		},
		{
			name: "risingEdgeWithFireInitial",
			got: requests.ConditionTypeRequest{
				RisingEdge: &requests.RisingEdgeRequest{
					FireInitial: true,
				},
			},
			wantBuilder: func(ctb *pb.ConditionalTrigger_builder) {
				ctb.RisingEdge = pb.ConditionalTrigger_ConditionTypeRisingEdge_builder{
					FireInitial: true,
				}.Build()
			},
		},
		{
			name: "risingEdgeWithInitializeExpression",
			got: requests.ConditionTypeRequest{
				RisingEdge: &requests.RisingEdgeRequest{
					InitializeExpression: "expr1",
				},
			},
			wantBuilder: func(ctb *pb.ConditionalTrigger_builder) {
				exprId := uint32(0)
				ctb.RisingEdge = pb.ConditionalTrigger_ConditionTypeRisingEdge_builder{
					InitializeNodeIndex: &exprId,
				}.Build()
			},
		},
		{
			name: "fallingEdge",
			got: requests.ConditionTypeRequest{
				FallingEdge: &requests.FallingEdgeRequest{},
			},
			wantBuilder: func(ctb *pb.ConditionalTrigger_builder) {
				ctb.FallingEdge = pb.ConditionalTrigger_ConditionTypeFallingEdge_builder{}.Build()
			},
		},
		{
			name: "fallingEdgeWithEdgeoptions",
			got: requests.ConditionTypeRequest{
				FallingEdge: &requests.FallingEdgeRequest{
					FallingOptions: &requests.EdgeOptionsRequest{
						MinDurationMs: 123,
						RequireExact:  true,
					},
				},
			},
			wantBuilder: func(ctb *pb.ConditionalTrigger_builder) {
				ctb.FallingEdge = pb.ConditionalTrigger_ConditionTypeFallingEdge_builder{
					FallingOptions: pb.ConditionalTrigger_EdgeOptions_builder{
						MinDuration:  durationpb.New(time.Duration(123 * time.Millisecond)),
						RequireExact: true,
					}.Build(),
				}.Build()
			},
		},
		{
			name: "fallingEdgeWithFireInitial",
			got: requests.ConditionTypeRequest{
				FallingEdge: &requests.FallingEdgeRequest{
					FireInitial: true,
				},
			},
			wantBuilder: func(ctb *pb.ConditionalTrigger_builder) {
				ctb.FallingEdge = pb.ConditionalTrigger_ConditionTypeFallingEdge_builder{
					FireInitial: true,
				}.Build()
			},
		},
		{
			name: "fallingEdgeWithInitializeExpression",
			got: requests.ConditionTypeRequest{
				FallingEdge: &requests.FallingEdgeRequest{
					InitializeExpression: "expr1",
				},
			},
			wantBuilder: func(ctb *pb.ConditionalTrigger_builder) {
				exprId := uint32(0)
				ctb.FallingEdge = pb.ConditionalTrigger_ConditionTypeFallingEdge_builder{
					InitializeNodeIndex: &exprId,
				}.Build()
			},
		},
		{
			name: "isTrue",
			got: requests.ConditionTypeRequest{
				IsTrue: &struct{}{},
			},
			wantBuilder: func(ctb *pb.ConditionalTrigger_builder) {
				ctb.IsTrue = pb.ConditionalTrigger_ConditionTypeIsTrue_builder{}.Build()
			},
		},
		{
			name: "isFalse",
			got: requests.ConditionTypeRequest{
				IsFalse: &struct{}{},
			},
			wantBuilder: func(ctb *pb.ConditionalTrigger_builder) {
				ctb.IsFalse = pb.ConditionalTrigger_ConditionTypeIsFalse_builder{}.Build()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctreq := &requests.ConditionalTriggerRequest{
				ConditionType: &tc.got,
			}

			sess := &session.Session{
				Expressions:                make(map[uint32]expressions.Text),
				NextUncompiledExpressionID: 0,
			}

			ctBuilder := pb.ConditionalTrigger_builder{}
			if err := requests.ParseConditionType(ctreq, &ctBuilder, sess); err != nil {
				t.Fatalf("requests.ParseConditionType(ctreq, &ctBuilder) = %v, want nil", err)
			}

			got := ctBuilder.Build()
			ctb := pb.ConditionalTrigger_builder{}
			tc.wantBuilder(&ctb)
			want := ctb.Build()
			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Errorf("Conditional trigger is unexpectedly different (-want +got):\n%s", diff)
			}
		})
	}
}

func getDefaultConditionalTriggerRequest() *requests.ConditionalTriggerRequest {
	return &requests.ConditionalTriggerRequest{
		Triggers: []string{"hello"},
		ConditionType: &requests.ConditionTypeRequest{
			AllChanges: &requests.AllChangesRequest{},
		},
		Expression: "expression",
	}
}

func getDefaultPeriodicTriggerRequest() *requests.PeriodicTriggerRequest {
	return &requests.PeriodicTriggerRequest{PeriodMs: 100}
}

func getDefaultDataTriggerRequest() *requests.DataTriggerRequest {
	return &requests.DataTriggerRequest{Source: "source_name"}
}

func TestTriggerRequestWithSingleTriggersPasses(t *testing.T) {
	for _, req := range []*requests.TriggerRequest{
		{
			Name: "DataTrigger",
			Data: getDefaultDataTriggerRequest(),
		},
		{
			Name:     "PeriodicTrigger",
			Periodic: getDefaultPeriodicTriggerRequest(),
		},
		{
			Name:        "ConditionalTrigger",
			Conditional: getDefaultConditionalTriggerRequest(),
		},
	} {
		if err := requests.CheckDoesNotHaveMultipleTriggers(req); err != nil {
			t.Fatal("Defining a single trigger should be allowed.")
		}
	}
}

func TestTriggerRequestWithMultipleTriggersFails(t *testing.T) {
	for _, req := range []*requests.TriggerRequest{
		{
			Name:        "AllThreeTriggers",
			Periodic:    getDefaultPeriodicTriggerRequest(),
			Data:        getDefaultDataTriggerRequest(),
			Conditional: getDefaultConditionalTriggerRequest(),
		},
		{
			Name:     "PeriodicDataTrigger",
			Periodic: getDefaultPeriodicTriggerRequest(),
			Data:     getDefaultDataTriggerRequest(),
		},
		{
			Name:        "PeriodicConditionalTrigger",
			Periodic:    getDefaultPeriodicTriggerRequest(),
			Conditional: getDefaultConditionalTriggerRequest(),
		},
		{
			Name:        "ConditionalDataTrigger",
			Data:        getDefaultDataTriggerRequest(),
			Conditional: getDefaultConditionalTriggerRequest(),
		},
	} {
		err := requests.CheckDoesNotHaveMultipleTriggers(req)
		if err == nil {
			t.Fatal("Defining multiple triggers should not be allowed.")
		}
	}
}

func TestDataTriggerSourceAndDeprecatedPublisher(t *testing.T) {
	testCases := []struct {
		name        string
		req         *requests.DataTriggerRequest
		expectError bool
		wantName    string
	}{
		{
			name:        "DeprecatedPublisherNameOnly",
			req:         &requests.DataTriggerRequest{Publisher: "p1"},
			expectError: false,
			wantName:    "p1",
		},
		{
			name:        "SourceNameOnly",
			req:         &requests.DataTriggerRequest{Source: "s1"},
			expectError: false,
			wantName:    "s1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tr := &requests.TriggerRequest{
				Name: "trigger1",
				Data: tc.req,
			}
			sess := &session.Session{
				Triggers: make(map[string]*pb.Trigger),
			}

			pbTrigger, err := tr.ToProto(sess)

			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if pbTrigger.GetDataTrigger().GetSourceName() != tc.wantName {
					t.Errorf("SourceName = %q, want %q", pbTrigger.GetDataTrigger().GetSourceName(), tc.wantName)
				}
			}
		})
	}
}

func TestPeriodicTriggerToProto(t *testing.T) {
	countVal := uint32(5)
	testCases := []struct {
		name        string
		req         *requests.PeriodicTriggerRequest
		wantBuilder func(*pb.PeriodicTrigger_builder)
	}{
		{
			name: "Basic",
			req: &requests.PeriodicTriggerRequest{
				PeriodMs: 100,
			},
			wantBuilder: func(b *pb.PeriodicTrigger_builder) {
				b.Interval = durationpb.New(100 * time.Millisecond)
			},
		},
		{
			name: "WithTriggersAndCount",
			req: &requests.PeriodicTriggerRequest{
				PeriodMs: 100,
				Triggers: []string{"t1", "t2"},
				Count:    &countVal,
			},
			wantBuilder: func(b *pb.PeriodicTrigger_builder) {
				b.Interval = durationpb.New(100 * time.Millisecond)
				b.TriggerNames = []string{"t1", "t2"}
				b.Count = &countVal
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tr := &requests.TriggerRequest{
				Name:     "periodic_trigger",
				Periodic: tc.req,
			}
			sess := &session.Session{
				Triggers: make(map[string]*pb.Trigger),
			}

			pbTrigger, err := tr.ToProto(sess)
			if err != nil {
				t.Fatalf("ToProto failed: %v", err)
			}

			got := pbTrigger.GetPeriodicTrigger()
			builder := pb.PeriodicTrigger_builder{}
			tc.wantBuilder(&builder)
			want := builder.Build()

			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Errorf("PeriodicTrigger is unexpectedly different (-want +got):\n%s", diff)
			}
		})
	}
}
