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

package validators

var (
	// struct methods
	// go/keep-sorted start
	PrintErrorList                                   = (*McValidator).printErrorList
	Validate                                         = (*McValidator).validate
	ValidateExpressionNodes                          = (*McValidator).validateExpressionNodes
	ValidateHasStartTriggerIfHasEndTrigger           = (*McValidator).validateHasStartTriggerIfHasEndTrigger
	ValidateLifeCycleTriggersReferToExistingTriggers = (*McValidator).validateLifeCycleTriggersReferToExistingTriggers
	ValidateMetricsReportConfigs                     = (*McValidator).validateMetricsReportConfigs
	ValidateNoSourceTriggerCycles                    = (*McValidator).validateNoSourceTriggerCycles
	ValidateSources                                  = (*McValidator).validateSources
	ValidateTriggers                                 = (*McValidator).validateTriggers
	ValidateUuid                                     = (*McValidator).validateUuid
	// go/keep-sorted end

	// free functions
	// go/keep-sorted start
	FieldAssExpressionNodeReferenceIsValid = fieldAssExpressionNodeReferenceIsValid
	NewMcValidator                         = newMcValidator
	ShallowValidateProtoMessageBuilder     = shallowValidateProtoMessageBuilder
	// go/keep-sorted end
)
