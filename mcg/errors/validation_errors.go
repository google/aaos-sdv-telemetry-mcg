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

import "fmt"

// Shared consts
const (
	// Field violation type for an invalid field
	FIELD_INVALID = "invalid"
	// Field violation type for a missing field
	FIELD_MISSING = "missing"
	// Field violation type for a missing or an empty field
	FIELD_MISSING_OR_EMPTY = "missing or empty"
)

// Trigger related errors.
var (
	ConditionalTriggerConditionTypeMissing = InvalidArgument("Conditional trigger missing condition type").WithFieldViolation("conditional.condition_type", FIELD_MISSING)
	ConditionalTriggerConditionTypeInvalid = func(err error) *StatusError {
		return InvalidArgument(fmt.Sprintf("Conditional trigger condition type is invalid: %v", err)).WithFieldViolation("conditional.condition_type", FIELD_INVALID)
	}
	ConditionalTriggerConditionTypeInvalidEdgeOptions = func(err error) *StatusError {
		return InvalidArgument(fmt.Sprintf("Conditional trigger condition type has invalid edge options: %v", err)).WithFieldViolation("conditional.condition_type", FIELD_INVALID)
	}
	ConditionalTriggerExpressionIdMissing   = InvalidArgument("Conditional trigger missing expression ID").WithFieldViolation("triggers.expressionId", FIELD_MISSING)
	ConditionalTriggerParentTriggersMissing = InvalidArgument("Conditional trigger has empty list of parent triggers").WithFieldViolation("conditional.trigger_names", FIELD_MISSING_OR_EMPTY)
	DataTriggerSourceMissing                = InvalidArgument("Data trigger missing source name").WithFieldViolation("data.source_name", FIELD_MISSING)
	DataTriggerSourceReferenceInvalid       = func(sourceName string) *StatusError {
		return InvalidArgument("Data trigger refers to a non-existent source").WithFieldViolation(sourceName, "trigger should refer to an existing source")
	}
	DuplicateTriggerName = func(nodeName string) *StatusError {
		return InvalidArgument("There are multiple triggers with the same name").WithFieldViolation(nodeName, FIELD_INVALID)
	}
	StopTriggerSetWithoutStartTrigger             = InvalidArgument("Stop trigger cannot be set if start trigger is not set").WithFieldViolation("start_trigger", "missing")
	MultipleTriggers                              = InvalidArgument("Multiple trigger formats specified")
	NoTriggerProvided                             = InvalidArgument("No trigger data provided").WithFieldViolation("conditional", FIELD_MISSING).WithFieldViolation("data", FIELD_MISSING).WithFieldViolation("periodic", FIELD_MISSING)
	PeriodicTriggerIntervalMissing                = InvalidArgument("Periodic trigger missing interval").WithFieldViolation("periodic.interval_ms", FIELD_MISSING)
	PeriodicTriggerIntervalNegative               = InvalidArgument("Periodic trigger interval must not be negative").WithFieldViolation("periodic.interval_ms", FIELD_INVALID)
	PeriodicTriggerInvalidCount                   = InvalidArgument("Periodic trigger count must be positive").WithFieldViolation("periodic.count", FIELD_INVALID)
	TriggerNameMissing                            = InvalidArgument("Trigger missing name").WithFieldViolation("name", FIELD_MISSING)
	StartTriggerNameNotMatchingAnyExistingTrigger = func(triggerName string) *StatusError {
		return InvalidArgument("Start trigger name doesn't match with any existing trigger").WithFieldViolation(triggerName, FIELD_MISSING)
	}
	StopTriggerNameNotMatchingAnyExistingTrigger = func(triggerName string) *StatusError {
		return InvalidArgument("Stop trigger name doesn't match with any existing trigger").WithFieldViolation(triggerName, FIELD_MISSING)
	}
	DeactivateTriggerNameNotMatchingAnyExistingTrigger = func(triggerName string) *StatusError {
		return InvalidArgument("Deactivate trigger name doesn't match with any existing trigger").WithFieldViolation(triggerName, FIELD_MISSING)
	}
	ConditionalParentTriggerNameNotMatchingAnyExistingTrigger = func(triggerName string) *StatusError {
		return InvalidArgument("Conditional trigger's parent trigger name doesn't match with any existing trigger").WithFieldViolation(triggerName, FIELD_MISSING)
	}
	AggregatorParentTriggerNameNotMatchingAnyExistingTrigger = func(triggerName string) *StatusError {
		return InvalidArgument("Aggregator's parent trigger name doesn't match with any existing trigger").WithFieldViolation(triggerName, FIELD_MISSING)
	}
	PeriodicParentTriggerNameNotMatchingAnyExistingTrigger = func(triggerName string) *StatusError {
		return InvalidArgument("Periodic trigger's parent trigger name doesn't match with any existing trigger").WithFieldViolation(triggerName, FIELD_MISSING)
	}
	TriggerTypeUnknown                              = InvalidArgument("Trigger has no or unknown type")
	DataTriggerUsingOnDemandDataSource              = InvalidArgument("Data triggers cannot use data sources with ON_DEMAND connection type").WithFieldViolation("data.source_name", FIELD_INVALID)
	ConditionTypeEdgeOptionWithFireInitial          = InvalidArgument("Either fire_initial = true or edge options can be specified, but not both").WithFieldViolation("fire_initial", FIELD_INVALID)
	ConditionTypeInitializeNodeIndexWithFireInitial = InvalidArgument("You cannot specify both fire_initial and initialize_node_index because fire_initial makes initialize_node_index redundant").WithFieldViolation("fire_initial", FIELD_INVALID)
)

// Source related errors.
var (
	AggregatorTriggersMissing                    = InvalidArgument("Aggregator has empty list of trigger names").WithFieldViolation("aggregator.trigger_names", FIELD_MISSING_OR_EMPTY)
	AggregatorWithBlankMessageBuilder            = InvalidArgument("Aggregator cannot have blank MessageBuilder").WithFieldViolation("aggregator.message_builder", FIELD_MISSING)
	AggregatorWithInvalidExpressionNodeReference = func(sourceName string, expressionNodeIdx uint32) *StatusError {
		return InvalidArgument("Aggregator refers to a non-existent expression node").WithFieldViolation(fmt.Sprintf("%s.expression_nodes[%d]", sourceName, expressionNodeIdx), FIELD_INVALID)
	}
	DuplicateSourceName = func(nodeName string) *StatusError {
		return InvalidArgument("There are multiple sources with the same name").WithFieldViolation(nodeName, FIELD_INVALID)
	}
	MultipleSourceTypes             = InvalidArgument("Multiple source formats specified").WithFieldViolation("aggregator", "only one format can be chosen").WithFieldViolation("data_source", "only one format can be chosen")
	SourceNameMissing               = InvalidArgument("Source missing name").WithFieldViolation("name", FIELD_MISSING)
	SourceTypeUnknown               = InvalidArgument("No source data provided").WithFieldViolation("aggregator", FIELD_MISSING).WithFieldViolation("data_source", FIELD_MISSING)
	DataSourceIdentifierMissing     = InvalidArgument("Data source missing source identifier").WithFieldViolation("data_source.source_identifier", FIELD_MISSING)
	DataSourceConnectionTypeInvalid = func(cType string) *StatusError {
		return InvalidArgument(fmt.Sprintf("Value %v does not map to a valid ConnectionType", cType)).WithFieldViolation("data_source.connection_type", FIELD_INVALID)
	}
	DataSourceSubSamplingIntervalInvalid = func(cType float64) *StatusError {
		return InvalidArgument(fmt.Sprintf("Value %v does not map to a valid sub-sampling interval. It must be positive and less than 2^32 milliseconds.", cType)).WithFieldViolation("data_source.sub_sampling_interval_ms", FIELD_INVALID)
	}
	DataSourceConfigurationError = func(err error) *StatusError {
		return InvalidArgument(fmt.Sprintf("Failed to parse source configuration: %v", err)).WithFieldViolation("data_source.configuration", FIELD_INVALID)
	}
)

// MetricsReportConfig related errors
var (
	MetricsReportConfigTriggersMissingAndNotSetIncomplete = InvalidArgument("MetricsReportConfig has no triggers and not sending incomplete reports").
								WithFieldViolation("trigger_names", "missing or set report_incomplete").
								WithFieldViolation("report_incomplete", "required or set trigger_names")
	MetricsReportConfigNameMissing = InvalidArgument("MetricsReportConfig missing name").WithFieldViolation("name", FIELD_MISSING)
	DuplicateMetricsReportName     = func(nodeName string) *StatusError {
		return InvalidArgument("There are multiple metrics report configs with the same name").WithFieldViolation(nodeName, FIELD_INVALID)
	}
	MetricsReportConfigWithInvalidTriggerReference = func(nodeName string) *StatusError {
		return InvalidArgument("Metrics report config refers to a non-existent trigger").WithFieldViolation(nodeName, FIELD_INVALID)
	}
	MetricsReportConfigWithInvalidExpressionNodeReference = func(repName, fieldName string, expressionNodeIdx uint32) *StatusError {
		return InvalidArgument("A field assignment in metrics report config refers to a non-existent expression node").WithFieldViolation(fmt.Sprintf("%s.%s.expression_nodes[%d]", repName, fieldName, expressionNodeIdx), FIELD_INVALID)
	}
)

// MessageBuilder related errors
var (
	MessageBuilderMessageTypeNotStartingWithDot = func(path, msgType string) *StatusError {
		return InvalidArgument(fmt.Sprintf("All message types are expected to start with a dot (saw %q)", msgType)).WithFieldViolation(
			fmt.Sprintf("%s.message_type", path), "does not start with '.'")
	}
	MessageBuilderFieldAssignmentFieldNameMissing = func(path string, idx int) *StatusError {
		return InvalidArgument("Field assignment field_name missing").WithFieldViolation(
			fmt.Sprintf("%s.field_assignments[%d].field_name", path, idx), FIELD_MISSING)
	}
	MessageBuilderFieldAssignmentAggregationMissing = func(path string, idx int) *StatusError {
		return InvalidArgument("Field assignment aggregation missing").WithFieldViolation(
			fmt.Sprintf("%s.field_assignments[%d].aggregation", path, idx), FIELD_MISSING)
	}
	CombinationExpressionNodeWithInvalidExpressionNodeReference = func(expressionNodeIdx uint32) *StatusError {
		return InvalidArgument("A combination node refers to a non-existent expression node").WithFieldViolation(fmt.Sprintf("expression_nodes[%d]", expressionNodeIdx), FIELD_INVALID)
	}
	ExpressionNodeWithInvalidSourceReference = func(expressionNodeIdx int, sourceName string) *StatusError {
		return InvalidArgument("An expression node refers to a non-existent source").WithFieldViolation(fmt.Sprintf("expression_nodes[%d].SourceName:%s", expressionNodeIdx, sourceName), FIELD_INVALID)
	}
	UnknownMessageType = func(msgType string) *StatusError {
		return InvalidArgument("Message type doesn't match with a known type").WithFieldViolation(msgType, FIELD_MISSING)
	}
)

// MetricsConfig related errors
var (
	CyclicDependency = func(nodeName string) *StatusError {
		return InvalidArgument("Cyclic dependency detected").WithFieldViolation(nodeName, FIELD_INVALID)
	}
	StartTriggerMissing                = InvalidArgument("Metrics configs missing start trigger").WithFieldViolation("start_trigger", FIELD_MISSING)
	TriggerNameCollisionWithSourceName = func(nodeName string) *StatusError {
		return InvalidArgument("There is a trigger and a source which have the same name").WithFieldViolation(nodeName, FIELD_INVALID)
	}
	UuidMissing = InvalidArgument("Metrics config UUID is missing").WithFieldViolation("uuid", FIELD_MISSING)
	UuidInvalid = func(err error) *StatusError {
		return InvalidArgument(fmt.Sprintf("Metrics config UUID is invalid: %v", err)).WithFieldViolation("uuid", FIELD_INVALID)
	}
	UnaryOperatorExpressionNodeHasRightIndexSet = func(idx int) *StatusError {
		return InvalidArgument("Unary operator should only have left index set, but right index was found.").WithFieldViolation(fmt.Sprintf("expression_nodes[%d]", idx), FIELD_INVALID)
	}
	CombinationExpressionNodeDoesntHaveLeftIndexSet = func(idx int) *StatusError {
		return InvalidArgument("All combination expression nodes should have left index set.").WithFieldViolation(fmt.Sprintf("expression_nodes[%d]", idx), FIELD_MISSING)
	}
	NonUnaryCombinationExpressionNodeDoesntHaveRightIndexSet = func(idx int) *StatusError {
		return InvalidArgument("All non-unary combination expression nodes should have right index set.").WithFieldViolation(fmt.Sprintf("expression_nodes[%d]", idx), FIELD_MISSING)
	}
)

// FileDescriptorSet and FileDescriptorProto related errors.
var (
	FileDescriptorSetDescriptorProtosFailToParse = func(protoError error, idx int) *StatusError {
		return InvalidArgument(fmt.Sprintf("Failed to parse FileDescriptorSet")).WithFieldViolation(
			fmt.Sprintf("descriptor_protos[%d]", idx), protoError.Error())
	}
	FileDescriptorSetVehicleSignalsFailToParse = func(protoError error) *StatusError {
		return InvalidArgument("Failed to parse FileDescriptorSet").WithFieldViolation(
			"vehicle_signals", protoError.Error())
	}
)

// FileDescriptorSet and FileDescriptorProto related errors.
var (
	NoTypeResolver = func(messageName string) *StatusError {
		return InvalidArgument(fmt.Sprintf("No TypeResolver (protoregisry) available to look up: %v", messageName))
	}
)

// Cache Related Error
var NoCache = func() *StatusError {
	return FailedPrecondition("No cache enabled")
}

var NoCacheFailedLookup = func(version string) *StatusError {
	return FailedPrecondition(fmt.Sprintf("No cache enabled. Failed to lookup vehicle signal version: %v", version))
}

var CacheFailedLookup = func(version string) *StatusError {
	return FailedPrecondition(fmt.Sprintf("Failed to lookup vehicle signal version: %v", version))
}

var CacheRetrievalError = func() *StatusError {
	return FailedPrecondition(fmt.Sprintf("Could not retrieve cache from context."))
}

var InvalidExpressionError = func(expression string, err error) *StatusError {
	return FailedPrecondition(fmt.Errorf("Failed to parse expression %q: %w", expression, err).Error())
}
