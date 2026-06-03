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
	"math"
	"time"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
	"sdv.googlesource.com/mcg/mcg/session"
	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

// Deprecated: Use DataSourceRequest and AggregatorRequest instead.
type DeprecatedSourceRequest struct {
	Name        string                       `json:"name" x-optional:"1"`
	Service     *DeprecatedDataSourceRequest `json:"service"`
	Aggregation *DeprecatedAggregatorRequest `json:"aggregation"`
}

func (req *DeprecatedSourceRequest) ValidateDeprecated() *mcgerrors.StatusError {
	if req.Service != nil && req.Aggregation != nil {
		return mcgerrors.MultipleSourceTypes
	}
	if req.Service == nil && req.Aggregation == nil {
		return mcgerrors.SourceTypeUnknown
	}
	return nil
}

// Deprecated: Use DataSourceRequest instead.
type DeprecatedDataSourceRequest struct {
	ServiceName           string                          `json:"service_name"`
	ConnectionType        string                          `json:"connection_type"`
	Configuration         *DataSourceConfigurationRequest `json:"configuration"`
	SubSamplingIntervalMs float64                         `json:"sub_sampling_interval_ms"`
	FetchLastMessage      bool                            `json:"fetch_last_message"`
}

// Deprecated: Use AggregatorRequest instead.
type DeprecatedAggregatorRequest struct {
	Triggers       []string              `json:"trigger_names"`
	ResetOnGet     bool                  `json:"reset_on_get,omitempty"`
	MessageBuilder MessageBuilderRequest `json:"message_builder"`
}

type DataSourceRequest struct {
	Name                  string                          `json:"name"`
	SourceIdentifier      string                          `json:"source_identifier"`
	ConnectionType        string                          `json:"connection_type"`
	Configuration         *DataSourceConfigurationRequest `json:"configuration"`
	SubSamplingIntervalMs float64                         `json:"sub_sampling_interval_ms"`
	FetchLastMessage      bool                            `json:"fetch_last_message"`
}

func (req *DataSourceRequest) ValidateCanonical() *mcgerrors.StatusError {
	if req.SourceIdentifier == "" {
		return mcgerrors.Internal("DataSourceRequest missing 'source_identifier'. Ensure you are using the canonical flat format, not the deprecated nested 'service' wrapper.")
	}
	return nil
}

// Converts the request to a protobuf `pb.Source`.
func (req *DataSourceRequest) ToProto(s *session.Session) (*pb.Source, *mcgerrors.StatusError) {
	connectionType, err := getConnectionType(req.ConnectionType)
	if err != nil {
		return nil, err
	}

	subSamplingInterval, err := parseSubSamplingInterval(req.SubSamplingIntervalMs)
	if err != nil {
		return nil, err
	}

	var configuration *anypb.Any
	if req.Configuration != nil {
		var err error
		configuration, err = req.Configuration.ToProto(s.ParsedTypes)
		if err != nil {
			return nil, mcgerrors.DataSourceConfigurationError(err)
		}
	}

	return pb.Source_builder{
		Name: req.Name,
		DataSource: pb.DataSource_builder{
			SourceIdentifier:    req.SourceIdentifier,
			ConnectionType:      *connectionType,
			Configuration:       configuration,
			SubSamplingInterval: subSamplingInterval,
			FetchLastMessage:    req.FetchLastMessage,
		}.Build(),
	}.Build(), nil
}

type AggregatorRequest struct {
	Name           string                `json:"name" x-optional:"1"`
	Triggers       []string              `json:"trigger_names" validation:"required"`
	ResetOnGet     bool                  `json:"reset_on_get,omitempty"`
	MessageBuilder MessageBuilderRequest `json:"message_builder"`
}

func (req *AggregatorRequest) ValidateCanonical() *mcgerrors.StatusError {
	if len(req.Triggers) == 0 {
		return mcgerrors.Internal("AggregatorRequest missing 'trigger_names'. Ensure you are using the canonical flat format, not the deprecated nested 'aggregation' wrapper.")
	}
	return nil
}

// Converts the request to a protobuf `pb.Source`.
func (req *AggregatorRequest) ToProto(s *session.Session) (*pb.Source, *mcgerrors.StatusError) {
	msgBuilder, err := req.MessageBuilder.toProto(session.MessageBuilderLocation{
		IsSource: true, ContainerName: req.Name,
	}, s)
	if err != nil {
		return nil, err
	}

	return pb.Source_builder{
		Name: req.Name,
		Aggregator: pb.Aggregator_builder{
			TriggerNames:   req.Triggers,
			ResetOnGet:     req.ResetOnGet,
			MessageBuilder: msgBuilder,
		}.Build(),
	}.Build(), nil
}

func (req *DeprecatedSourceRequest) isAggregator() bool {
	return req.Aggregation != nil
}

func (req *DeprecatedSourceRequest) isDataSource() bool {
	return req.Service != nil
}

func (req *DeprecatedSourceRequest) checkDoesNotHaveMultipleSources() *mcgerrors.StatusError {
	if req.isAggregator() && req.isDataSource() {
		return mcgerrors.MultipleSourceTypes
	}

	return nil
}

func getConnectionType(connectionTypeString string) (*pb.DataSource_ConnectionType, *mcgerrors.StatusError) {
	// If field is not set, default to SUBSCRIPTION connection type.
	if connectionTypeString == "" {
		connectionTypeString = "SUBSCRIPTION"
	}
	if connectionTypeString == "GETTER" {
		connectionTypeString = "ON_DEMAND"
	}

	if ct, ok := pb.DataSource_ConnectionType_value[connectionTypeString]; ok {
		connectionType := pb.DataSource_ConnectionType(ct)
		return &connectionType, nil
	} else {
		return nil, mcgerrors.DataSourceConnectionTypeInvalid(connectionTypeString)
	}
}

func parseSubSamplingInterval(durationMs float64) (*durationpb.Duration, *mcgerrors.StatusError) {
	duration := time.Duration(float64(time.Millisecond) * durationMs)

	if duration < 0 || duration > (time.Millisecond*math.MaxUint32) {
		return nil, mcgerrors.DataSourceSubSamplingIntervalInvalid(durationMs)
	}
	if duration == 0 {
		// Do not set a sub-sampling interval at all when the duration is 0 - while
		// this is not technically required, it leads to cleaner protos without
		// unnecessary fields being set.
		return nil, nil
	}
	return durationpb.New(duration), nil
}

// Converts the request to a protobuf `pb.Source` with a blank name and potentially missing fields.
func (req *DeprecatedSourceRequest) ToProto(s *session.Session) (*pb.Source, *mcgerrors.StatusError) {
	if err := req.checkDoesNotHaveMultipleSources(); err != nil {
		return nil, err
	}

	switch true {
	case req.isAggregator():
		canReq := AggregatorRequest{
			Name:           req.Name,
			Triggers:       req.Aggregation.Triggers,
			ResetOnGet:     req.Aggregation.ResetOnGet,
			MessageBuilder: req.Aggregation.MessageBuilder,
		}
		return canReq.ToProto(s)

	case req.isDataSource():
		canReq := DataSourceRequest{
			Name:                  req.Name,
			SourceIdentifier:      req.Service.ServiceName,
			ConnectionType:        req.Service.ConnectionType,
			Configuration:         req.Service.Configuration,
			SubSamplingIntervalMs: req.Service.SubSamplingIntervalMs,
			FetchLastMessage:      req.Service.FetchLastMessage,
		}
		return canReq.ToProto(s)

	default:
		return nil, mcgerrors.SourceTypeUnknown
	}
}
