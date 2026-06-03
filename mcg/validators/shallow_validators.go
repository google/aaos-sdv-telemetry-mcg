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

// Package validators collects metrics config related validations.

package validators

import (
	"fmt"
	"strings"
	"time"

	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
	"sdv.googlesource.com/mcg/mcg/session"
	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

// Check a `ProtoMessageBuilder` for basic errors in isolation, without reference to the rest of the MetricsConfig.
func shallowValidateProtoMessageBuilder(msgBuilderLoc session.MessageBuilderLocation, pmb *pb.ProtoMessageBuilder) *mcgerrors.StatusError {
	// TODO(b/350777804): Adjust accordingly depending on whether the dot should or shouldn't be there
	if pmb.GetMessageType() != "" && !strings.HasPrefix(pmb.GetMessageType(), ".") {
		return mcgerrors.MessageBuilderMessageTypeNotStartingWithDot(msgBuilderLoc.ContainerPath(), pmb.GetMessageType())
	}

	for idx, v := range pmb.GetFieldAssignments() {
		if v.GetFieldName() == "" {
			return mcgerrors.MessageBuilderFieldAssignmentFieldNameMissing(msgBuilderLoc.ContainerPath(), idx)
		}
		if !v.HasAggregatedFieldValue() {
			return mcgerrors.MessageBuilderFieldAssignmentAggregationMissing(msgBuilderLoc.ContainerPath(), idx)
		}
	}

	return nil
}

// Check a `MetricsReportConfig` for basic errors in isolation, without reference to the rest of the MetricsConfig.
func ShallowValidateMetricsReportConfig(mrc *pb.MetricsReportConfig) *mcgerrors.StatusError {
	if mrc.GetName() == "" {
		return mcgerrors.MetricsReportConfigNameMissing
	}

	if len(mrc.GetTriggerNames()) == 0 && !mrc.GetReportIncomplete() {
		return mcgerrors.MetricsReportConfigTriggersMissingAndNotSetIncomplete
	}

	return shallowValidateProtoMessageBuilder(session.MessageBuilderLocation{
		IsSource:      false,
		ContainerName: mrc.GetName(),
	}, mrc.GetMessageBuilder())
}

func ShallowValidateEdgeOptions(e *pb.ConditionalTrigger_EdgeOptions) *mcgerrors.StatusError {
	if minDuration := e.GetMinDuration().AsDuration(); minDuration < time.Duration(0) {
		return mcgerrors.ConditionalTriggerConditionTypeInvalidEdgeOptions(fmt.Errorf("min_duration must be >= 0, but is %v", minDuration))
	}
	return nil
}

// Check a `Trigger` for basic errors in isolation, without reference to the rest of the MetricsConfig.
func ShallowValidateTrigger(trig *pb.Trigger) *mcgerrors.StatusError {
	if trig.GetName() == "" {
		return mcgerrors.TriggerNameMissing
	}

	switch trig.WhichTriggerType() {
	// go/keep-sorted start
	case pb.Trigger_ConditionalTrigger_case:
		ct := trig.GetConditionalTrigger()

		if triggerNames := ct.GetTriggerNames(); len(triggerNames) == 0 {
			return mcgerrors.ConditionalTriggerParentTriggersMissing
		}

		switch ct.WhichConditionType() {
		case pb.ConditionalTrigger_RisingEdge_case:
			re := ct.GetRisingEdge()
			if err := ShallowValidateEdgeOptions(re.GetRisingOptions()); err != nil {
				return err
			}
			if re.GetRisingOptions() != nil && re.GetFireInitial() {
				return mcgerrors.ConditionTypeEdgeOptionWithFireInitial
			}
			if re.GetFireInitial() && re.HasInitializeNodeIndex() {
				return mcgerrors.ConditionTypeInitializeNodeIndexWithFireInitial
			}
		case pb.ConditionalTrigger_FallingEdge_case:
			fe := ct.GetFallingEdge()
			if err := ShallowValidateEdgeOptions(fe.GetFallingOptions()); err != nil {
				return err
			}
			if fe.GetFallingOptions() != nil && fe.GetFireInitial() {
				return mcgerrors.ConditionTypeEdgeOptionWithFireInitial
			}
			if fe.GetFireInitial() && fe.HasInitializeNodeIndex() {
				return mcgerrors.ConditionTypeInitializeNodeIndexWithFireInitial
			}
		case pb.ConditionalTrigger_AllChanges_case:
			ac := ct.GetAllChanges()
			if err := ShallowValidateEdgeOptions(ac.GetRisingOptions()); err != nil {
				return err
			}
			if err := ShallowValidateEdgeOptions(ac.GetFallingOptions()); err != nil {
				return err
			}
			if (ac.GetRisingOptions() != nil || ac.GetFallingOptions() != nil) && ac.GetFireInitial() {
				return mcgerrors.ConditionTypeEdgeOptionWithFireInitial
			}
			if ac.GetFireInitial() && ac.HasInitializeNodeIndex() {
				return mcgerrors.ConditionTypeInitializeNodeIndexWithFireInitial
			}
		case pb.ConditionalTrigger_IsTrue_case, pb.ConditionalTrigger_IsFalse_case:
			// Nothing to validate
		case pb.ConditionalTrigger_ConditionType_not_set_case:
			return mcgerrors.ConditionalTriggerConditionTypeMissing
		}

		if !ct.HasSelectorNodeIndex() {
			return mcgerrors.ConditionalTriggerExpressionIdMissing
		}
	case pb.Trigger_DataTrigger_case:
		dt := trig.GetDataTrigger()

		if dt.GetSourceName() == "" {
			return mcgerrors.DataTriggerSourceMissing
		}
	case pb.Trigger_PeriodicTrigger_case:
		pt := trig.GetPeriodicTrigger()

		if pt.GetInterval().AsDuration() == time.Duration(0) {
			return mcgerrors.PeriodicTriggerIntervalMissing
		}
		if pt.GetInterval().AsDuration() < time.Duration(0) {
			return mcgerrors.PeriodicTriggerIntervalNegative
		}
		if pt.HasCount() && pt.GetCount() == 0 {
			return mcgerrors.PeriodicTriggerInvalidCount
		}
	// go/keep-sorted end
	default:
		return mcgerrors.TriggerTypeUnknown
	}

	return nil
}

func ShallowValidateAggregator(name string, ap *pb.Aggregator) *mcgerrors.StatusError {
	if len(ap.GetTriggerNames()) == 0 {
		return mcgerrors.AggregatorTriggersMissing
	}

	if ap.GetMessageBuilder() == nil {
		return mcgerrors.AggregatorWithBlankMessageBuilder
	}

	return shallowValidateProtoMessageBuilder(session.MessageBuilderLocation{
		IsSource:      true,
		ContainerName: name,
	}, ap.GetMessageBuilder())
}

func ShallowValidateDataSource(sp *pb.DataSource) *mcgerrors.StatusError {
	if sp.GetSourceIdentifier() == "" {
		return mcgerrors.DataSourceIdentifierMissing
	}
	return nil
}

// Check a `Source` for basic errors in isolation, without reference to the rest of the MetricsConfig.
func ShallowValidateSource(pub *pb.Source) *mcgerrors.StatusError {
	if pub.GetName() == "" {
		return mcgerrors.SourceNameMissing
	}

	switch pub.WhichSourceType() {
	// go/keep-sorted start
	case pb.Source_Aggregator_case:
		return ShallowValidateAggregator(pub.GetName(), pub.GetAggregator())
	case pb.Source_DataSource_case:
		return ShallowValidateDataSource(pub.GetDataSource())
	// go/keep-sorted end
	default:
		return mcgerrors.SourceTypeUnknown
	}
}
