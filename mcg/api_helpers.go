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

package mcg

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	txtpbfmtast "github.com/protocolbuffers/txtpbfmt/ast"
	txtpbfmt "github.com/protocolbuffers/txtpbfmt/parser"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"

	"sdv.googlesource.com/mcg/mcg/constants"
	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
	"sdv.googlesource.com/mcg/mcg/expressions"
	"sdv.googlesource.com/mcg/mcg/inference"
	"sdv.googlesource.com/mcg/mcg/session"
	"sdv.googlesource.com/mcg/mcg/validators"
	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

const (
	// Used, e.g., in the HTTP `Accept` header, to request a binary proto format.
	MediaTypeBinaryProto = "application/x-protobuf"
	// Used, e.g., in the HTTP `Accept` header, to request a text proto format.
	MediaTypeTextProto = "text/x-protobuf"
)

var (
	// Used in the HTTP `Content-Type` header to respond with a binary proto
	// format Metrics Config.
	ContentTypeBinaryProtoMetricsConfig string = fmt.Sprintf("%s; proto=google.sdv.telemetry.MetricsConfig", MediaTypeBinaryProto)
	// Used in the HTTP `Content-Type` header to respond with a text proto
	// format Metrics Config.
	ContentTypeTextProtoMetricsConfig string = fmt.Sprintf("%s; proto=google.sdv.telemetry.MetricsConfig; charset=utf-8", MediaTypeTextProto)
)

// checkBind wraps the Gin Bind functionality and returns false if the handler should abort.
//
// It creates a better error message and stores the error in c.Errors without writing the response headers.
func checkBind(c *gin.Context, v any) bool {
	if err := c.ShouldBind(v); err != nil {
		contentTypeName := binding.Default(c.Request.Method, c.ContentType()).Name()
		c.Error(mcgerrors.InvalidArgument(fmt.Sprintf("Unable to parse request body as %s: %v", contentTypeName, err)))
		return false
	}
	return true
}

func walkAst(nodes []*txtpbfmtast.Node, visit func(*txtpbfmtast.Node)) {
	for _, n := range nodes {
		visit(n)
		if n.Children != nil {
			walkAst(n.Children, visit)
		}
	}
}

type pathPair struct {
	legacy    []string
	canonical []string
}

var fieldRenames = []pathPair{
	// Naming conversions between legacy field names and new proto fields.
	// Rename children before parents.

	// 3 levels deep
	{[]string{"publishers", "service_publisher", "service_name"}, []string{"sources", "data_source", "source_identifier"}},

	// 2 levels deep
	{[]string{"expression_nodes", "field_leaf_node", "publisher_name"}, []string{"expression_nodes", "field_leaf_node", "source_name"}},
	{[]string{"triggers", "data_trigger", "publisher_name"}, []string{"triggers", "data_trigger", "source_name"}},
	{[]string{"metrics_report_configs", "publisher_name"}, []string{"metrics_report_configs", "source_name"}},
	{[]string{"publishers", "service_publisher"}, []string{"sources", "data_source"}},
	{[]string{"publishers", "aggregation_publisher"}, []string{"sources", "aggregator"}},
	{[]string{"metrics_report_configs", "publishers"}, []string{"metrics_report_configs", "sources"}},

	// 1 level deep
	{[]string{"end_trigger_name"}, []string{"stop_trigger_name"}},
	{[]string{"finish_trigger_name"}, []string{"deactivate_trigger_name"}},
	{[]string{"publishers"}, []string{"sources"}},
}

type valueRename struct {
	legacyPath     []string
	canonicalPath  []string
	legacyValue    string
	canonicalValue string
}

var valueRenames = []valueRename{
	{
		legacyPath:     []string{"publishers", "service_publisher", "connection_type"},
		canonicalPath:  []string{"sources", "data_source", "connection_type"},
		legacyValue:    "GETTER",
		canonicalValue: "ON_DEMAND",
	},
}

func applyCanonicalToLegacyRenames(nodes []*txtpbfmtast.Node) {
	for _, r := range valueRenames {
		for _, n := range txtpbfmtast.GetFromPath(nodes, r.canonicalPath) {
			for _, v := range n.Values {
				if v.Value == r.canonicalValue {
					v.Value = r.legacyValue
				}
			}
		}
	}

	for _, r := range fieldRenames {
		for _, n := range txtpbfmtast.GetFromPath(nodes, r.canonical) {
			n.Name = r.legacy[len(r.legacy)-1]
		}
	}
}

func applyLegacyToCanonicalRenames(nodes []*txtpbfmtast.Node) {
	for _, r := range valueRenames {
		for _, n := range txtpbfmtast.GetFromPath(nodes, r.legacyPath) {
			for _, v := range n.Values {
				if v.Value == r.legacyValue {
					v.Value = r.canonicalValue
				}
			}
		}
	}

	for _, r := range fieldRenames {
		for _, n := range txtpbfmtast.GetFromPath(nodes, r.legacy) {
			n.Name = r.canonical[len(r.canonical)-1]
		}
	}
}

func validateFormat(nodes []*txtpbfmtast.Node, apiVersion constants.APIVersion) error {
	var invalidFields []string
	if apiVersion == constants.APIVersionV1 {
		for _, pair := range fieldRenames {
			if len(txtpbfmtast.GetFromPath(nodes, pair.canonical)) > 0 {
				invalidFields = append(invalidFields, fmt.Sprintf("- %q (should be %q)", strings.Join(pair.canonical, "."), strings.Join(pair.legacy, ".")))
			}
		}
		if len(invalidFields) > 0 {
			return mcgerrors.InvalidArgument(fmt.Sprintf("The provided textproto uses the canonical format, which is not supported by the v1 API. Please use the legacy format or the v2 API.\nIncorrect fields:\n%s", strings.Join(invalidFields, "\n")))
		}
	} else if apiVersion == constants.APIVersionV2 {
		for _, pair := range fieldRenames {
			if len(txtpbfmtast.GetFromPath(nodes, pair.legacy)) > 0 {
				invalidFields = append(invalidFields, fmt.Sprintf("- %q (should be %q)", strings.Join(pair.legacy, "."), strings.Join(pair.canonical, ".")))
			}
		}
		if len(invalidFields) > 0 {
			return mcgerrors.InvalidArgument(fmt.Sprintf("The provided textproto uses the legacy format, which is not supported by the v2 API. Please use the canonical format or the v1 API.\nIncorrect fields:\n%s", strings.Join(invalidFields, "\n")))
		}
	}

	return nil
}

// The Rust textproto parser does not support the optional colons produced by
// the default `prototext` marshalling. Therefore, we use txtpbfmt to
// post-process the generated textproto and remove the colons. We also add
// comments to all expression nodes that contain the expression node index to
// aid debugging.
func textprotoMarshal(p proto.Message, legacyFormat bool) ([]byte, error) {
	by, err := (prototext.MarshalOptions{
		// This indent is ignored/overwritten by the `txtpbfmt.ParseWithConfig`
		// below: Regardless of what indent we specify here, the resulting textproto
		// will always be indented using `"  "`.
		Indent: "  ",
		// Explicitly override the type resolver with an empty resolver, so that
		// `Any` protos are not marshalled in their special expanded form
		// syntax, which rust-protobuf doesn't support (see b/434171834 and
		// github.com/stepancheg/rust-protobuf/issues/628).
		//
		// In other words, we want:
		//
		// ```
		// any_value {
		//   type_url: "type.googleapis.com/com.example.SomeType"
		//   value: "\x0a\x05hello"
		// }
		// ```
		//
		// and not:
		//
		// ```
		// any_value {
		//   [type.googleapis.com/com.example.SomeType] {
		//     field1: "hello"
		//   }
		// }
		// ```
		Resolver: new(protoregistry.Types),
	}).Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal into textproto: %w", err)
	}

	nodes, err := txtpbfmt.ParseWithConfig(by, txtpbfmt.Config{
		SkipAllColons: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse textproto: %w", err)
	}

	if legacyFormat {
		applyCanonicalToLegacyRenames(nodes)
	}

	for i, expressionNode := range txtpbfmtast.GetFromPath(nodes, []string{"expression_nodes"}) {
		expressionNode.PreComments = append([]string{fmt.Sprintf("# Expression Node %d", i)}, expressionNode.PreComments...)
	}

	return txtpbfmt.PrettyBytes(nodes, 0), nil
}

func chooseRenderFunc(c *gin.Context, apiVersion constants.APIVersion) (func(c *gin.Context, mc *pb.MetricsConfig), *mcgerrors.StatusError) {
	legacyTextproto := apiVersion == constants.APIVersionV1
	switch c.NegotiateFormat(ContentTypeBinaryProtoMetricsConfig, ContentTypeTextProtoMetricsConfig) {
	case ContentTypeBinaryProtoMetricsConfig:
		return func(c *gin.Context, mc *pb.MetricsConfig) {
			by, err := proto.Marshal(mc)
			if err != nil {
				c.Error(mcgerrors.InternalFromError(err))
				return
			}
			c.Data(http.StatusOK, ContentTypeBinaryProtoMetricsConfig, by)
		}, nil

	case ContentTypeTextProtoMetricsConfig:
		return func(c *gin.Context, mc *pb.MetricsConfig) {
			by, err := textprotoMarshal(mc, legacyTextproto)
			if err != nil {
				c.Error(mcgerrors.InternalFromError(err))
				return
			}
			c.Data(http.StatusOK, ContentTypeTextProtoMetricsConfig, by)
		}, nil

	default:
		return nil, mcgerrors.InvalidArgument(fmt.Sprintf("Requested `Content-Type` not supported, use %q or %q in the `Accept` header.", MediaTypeBinaryProto, MediaTypeTextProto))
	}
}

func translateMessageBuilderExpressionNodes(mapping map[uint32]uint32, target, source *pb.ProtoMessageBuilder) {
	for i := range source.GetFieldAssignments() {
		targetFa := target.GetFieldAssignments()[i]
		sessNID, ok := expressions.ExtractNodeIndex(source.GetFieldAssignments()[i])
		if ok {
			expressions.SetNodeIndex(targetFa, proto.Uint32(mapping[sessNID]))
		}
	}
}

func GetSortedStringKeys[T any](sessMap map[string]*T) []string {
	names := make([]string, 0, len(sessMap))
	for k := range sessMap {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// getTriggersWithReplacedIndices returns a list of triggers with the expression node indices
// replaced by the indices in the mapping.
func getTriggersWithReplacedIndices(mapping map[uint32]uint32, sessTriggers map[string]*pb.Trigger) []*pb.Trigger {
	var names []string = GetSortedStringKeys[pb.Trigger](sessTriggers)
	var resTriggers []*pb.Trigger = make([]*pb.Trigger, 0, len(sessTriggers))

	for _, name := range names {
		v := sessTriggers[name]
		switch v.WhichTriggerType() {
		// go/keep-sorted start
		case pb.Trigger_ConditionalTrigger_case:
			ct := v.GetConditionalTrigger()

			cloned := proto.Clone(v).(*pb.Trigger)
			clonedCt := cloned.GetConditionalTrigger()
			clonedCt.SetSelectorNodeIndex(mapping[ct.GetSelectorNodeIndex()])

			switch clonedCt.WhichConditionType() {
			case pb.ConditionalTrigger_RisingEdge_case:
				re := clonedCt.GetRisingEdge()
				if re.HasInitializeNodeIndex() {
					re.SetInitializeNodeIndex(mapping[re.GetInitializeNodeIndex()])
				}
			case pb.ConditionalTrigger_FallingEdge_case:
				fe := clonedCt.GetFallingEdge()
				if fe.HasInitializeNodeIndex() {
					fe.SetInitializeNodeIndex(mapping[fe.GetInitializeNodeIndex()])
				}
			case pb.ConditionalTrigger_AllChanges_case:
				ac := clonedCt.GetAllChanges()
				if ac.HasInitializeNodeIndex() {
					ac.SetInitializeNodeIndex(mapping[ac.GetInitializeNodeIndex()])
				}
			}

			resTriggers = append(resTriggers, cloned)
		case pb.Trigger_DataTrigger_case:
			resTriggers = append(resTriggers, &*v)
		case pb.Trigger_PeriodicTrigger_case:
			resTriggers = append(resTriggers, &*v)
			// go/keep-sorted end
		}
	}

	return resTriggers
}

// getSourcesWithReplacedIndices returns a list of sources with the expression node indices
// replaced by the indices in the mapping.
func getSourcesWithReplacedIndices(mapping map[uint32]uint32, sessSources map[string]*pb.Source) []*pb.Source {
	var names []string = GetSortedStringKeys[pb.Source](sessSources)
	var resSources []*pb.Source = make([]*pb.Source, 0, len(sessSources))

	for _, name := range names {
		v := sessSources[name]
		switch v.WhichSourceType() {
		// go/keep-sorted start
		case pb.Source_Aggregator_case:
			ap := v.GetAggregator()

			cloned := proto.Clone(v).(*pb.Source)
			translateMessageBuilderExpressionNodes(mapping, cloned.GetAggregator().GetMessageBuilder(), ap.GetMessageBuilder())
			resSources = append(resSources, cloned)
		case pb.Source_DataSource_case:
			resSources = append(resSources, &*v)
			// go/keep-sorted end
		}
	}

	return resSources
}

// getReportConfigsWithReplacedIndices returns a list of triggers with the expression node indices
// replaced by the indices in the mapping.
func getReportConfigsWithReplacedIndices(mapping map[uint32]uint32, sessReports map[string]*pb.MetricsReportConfig) []*pb.MetricsReportConfig {
	var names []string = GetSortedStringKeys[pb.MetricsReportConfig](sessReports)
	var resReports []*pb.MetricsReportConfig = make([]*pb.MetricsReportConfig, 0, len(sessReports))

	for _, name := range names {
		v := sessReports[name]
		cloned := proto.Clone(v).(*pb.MetricsReportConfig)
		translateMessageBuilderExpressionNodes(mapping, cloned.GetMessageBuilder(), v.GetMessageBuilder())
		resReports = append(resReports, cloned)
	}

	return resReports
}

// compileSession compiles a Session into a MetricsConfig.
//
// Requirements:
//   - s.ConfigUUID is set
//   - None of the maps in the session are nil
func compileSession(sess *session.Session) (*pb.MetricsConfig, []*mcgerrors.StatusError, string) {
	exprParser := expressions.NewParserShunt()

	mapping, nodes, err := exprParser.CompileAll(sess.Expressions)
	if err != nil {
		return nil, []*mcgerrors.StatusError{mcgerrors.InvalidArgumentFromError(err)}, "Parsing expressions failed."
	}

	mc := pb.MetricsConfig_builder{
		Uuid:    sess.ConfigUUID.String(),
		Version: constants.MetricsConfigVersion,

		StartTriggerName:      sess.StartTrigger,
		StopTriggerName:       sess.StopTrigger,
		DeactivateTriggerName: sess.DeactivateTrigger,

		ExpressionNodes: nodes,

		Triggers:             getTriggersWithReplacedIndices(mapping, sess.Triggers),
		Sources:              getSourcesWithReplacedIndices(mapping, sess.Sources),
		MetricsReportConfigs: getReportConfigsWithReplacedIndices(mapping, sess.ReportConfigs),

		RetainAggregationsOnStop: sess.RetainAggregationsOnStop,
	}.Build()

	if sess.NoMessageInference {
		// Copy input descriptors verbatim when inference is skipped.
		mc.SetDescriptorProtos(sess.InputDescriptors)
	} else {
		if errs := inference.Infer(mc, sess.ParsedTypes, sess.DataSourceMessageTypes); len(errs) > 0 {
			var errorList []*mcgerrors.StatusError
			for _, err := range errs {
				errorList = append(errorList, mcgerrors.InvalidArgumentFromError(err))
			}
			return nil, errorList, "Inference failed."
		}
	}

	if !sess.IgnoreValidations {
		// Validating the metrics configs is an expensive operation, so only validate them if necessary.
		if errorList := validators.ValidateWithoutShallowValidations(mc); len(errorList) > 0 {
			return nil, errorList, "Validation failed."
		}
	}

	return mc, []*mcgerrors.StatusError{}, ""
}

func getIgnoreValidationQueryParamValue(c *gin.Context) (bool, *mcgerrors.StatusError) {
	return getQueryParamBoolValue(c, "ignore_validation")
}

func getNoInferenceQueryParamValue(c *gin.Context) (bool, *mcgerrors.StatusError) {
	return getQueryParamBoolValue(c, "no_inference")
}

func getReturnConfigQueryParamValue(c *gin.Context) (bool, *mcgerrors.StatusError) {
	return getQueryParamBoolValue(c, "return_config")
}

func getQueryParamBoolValue(c *gin.Context, parameter string) (bool, *mcgerrors.StatusError) {
	if value, exists := c.GetQuery(parameter); exists {
		if boolVal, err := strconv.ParseBool(value); err != nil {
			return false, mcgerrors.InvalidArgument(fmt.Sprintf("Failed to parse boolean value from %s=%s", parameter, value))
		} else {
			return boolVal, nil
		}
	}

	return false, nil
}

// Parses the `bytes` as a Metrics Config depending on the content type.
func parseMetricsConfig(bytes []byte, contentType string, apiVersion constants.APIVersion) (*pb.MetricsConfig, error) {
	mc := new(pb.MetricsConfig)

	switch contentType {
	case CONTENT_TYPE_APP_X_PROTOBUF:
		if err := proto.Unmarshal(bytes, mc); err != nil {
			return nil, err
		}
	case CONTENT_TYPE_TEXT_X_PROTOBUF:
		nodes, err := txtpbfmt.Parse(bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse textproto: %w", err)
		}

		if err := validateFormat(nodes, apiVersion); err != nil {
			return nil, err
		}

		if apiVersion == constants.APIVersionV1 {
			applyLegacyToCanonicalRenames(nodes)
			bytes = txtpbfmt.PrettyBytes(nodes, 0)
		}

		if err := prototext.Unmarshal(bytes, mc); err != nil {
			return nil, err
		}
	default:
		return nil, mcgerrors.InvalidArgument(fmt.Sprintf("Unsupported content type: %s", contentType))
	}

	return mc, nil
}

func addMcSizeResponseHeader(mc *pb.MetricsConfig, c *gin.Context) error {
	mcBytes, err := proto.Marshal(mc)
	if err != nil {
		return err
	}
	c.Header("Metrics-Config-Size", fmt.Sprintf("%d", len(mcBytes)))
	return nil
}
