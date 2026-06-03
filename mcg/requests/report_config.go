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
	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
	"sdv.googlesource.com/mcg/mcg/session"
	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

type ReportConfigRequest struct {
	Name             string                `json:"name,omitempty"`
	Triggers         []string              `json:"trigger_names"`
	ReportIncomplete bool                  `json:"report_incomplete"`
	ReportInitial    bool                  `json:"report_initial"`
	MessageBuilder   MessageBuilderRequest `json:"message_builder"`
}

func (req *ReportConfigRequest) ToProto(s *session.Session) (*pb.MetricsReportConfig, *mcgerrors.StatusError) {
	pmb, err := req.MessageBuilder.toProto(session.MessageBuilderLocation{
		IsSource: false, ContainerName: req.Name,
	}, s)
	if err != nil {
		return nil, err
	}

	return pb.MetricsReportConfig_builder{
		Name:             req.Name,
		TriggerNames:     req.Triggers,
		ReportIncomplete: req.ReportIncomplete,
		ReportInitial:    req.ReportInitial,
		MessageBuilder:   pmb,
	}.Build(), nil
}
