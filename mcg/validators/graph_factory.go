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

import (
	"fmt"
	"strconv"
	"strings"

	"sdv.googlesource.com/mcg/mcg/expressions"
	"sdv.googlesource.com/mcg/mcg/graph"

	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

// newGraphForSourceAndTriggerDepsCycleChecks constructs a `Graph` from the
// triggers and sources on the passed metrics configs. Other than data
// triggers, conditional triggers or aggregators cannot form cycles
// because they cannot refer to other sources/triggers so those are ignored
// from the sort.
func newGraphForSourceAndTriggerDepsCycleChecks(mc *pb.MetricsConfig) *graph.Graph[string] {
	g := graph.NewGraph[string]()

	for _, trigger := range mc.GetTriggers() {
		if dataTrigger := trigger.GetDataTrigger(); dataTrigger != nil {
			g.AddEdge(trigger.GetName(), dataTrigger.GetSourceName())
		} else if conditionalTrigger := trigger.GetConditionalTrigger(); conditionalTrigger != nil {
			for _, parentTriggerName := range conditionalTrigger.GetTriggerNames() {
				g.AddEdge(trigger.GetName(), parentTriggerName)
			}
		} else if periodicTrigger := trigger.GetPeriodicTrigger(); periodicTrigger != nil {
			for _, parentTriggerName := range periodicTrigger.GetTriggerNames() {
				g.AddEdge(trigger.GetName(), parentTriggerName)
			}
		}
	}

	for _, pub := range mc.GetSources() {
		if aggPub := pub.GetAggregator(); aggPub != nil {
			for _, triggerName := range aggPub.GetTriggerNames() {
				g.AddEdge(pub.GetName(), triggerName)
			}
		}
	}
	return g
}

// InferenceNode represents a node in the inference dependency graph.
type InferenceNode string

// SourceName constructs an InferenceNode for a named source.
func SourceName(name string) InferenceNode {
	return InferenceNode("source:" + name)
}

// MessageBuilderNode constructs an InferenceNode for a message builder expression node by its index.
func MessageBuilderNode(idx uint32) InferenceNode {
	return InferenceNode("message_builder_node:" + strconv.FormatUint(uint64(idx), 10))
}

// SourceName returns the source name if n represents a source.
func (n InferenceNode) SourceName() (string, bool) {
	return strings.CutPrefix(string(n), "source:")
}

// MessageBuilderIndex returns the expression node index if n represents a message builder node.
func (n InferenceNode) MessageBuilderIndex() (uint32, bool) {
	idxStr, ok := strings.CutPrefix(string(n), "message_builder_node:")
	if !ok {
		return 0, false
	}
	idx, err := strconv.ParseUint(idxStr, 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(idx), true
}

// NewGraphForInferenceCycleChecks constructs a `Graph` from sources and message
// builder nodes. Edges in the graph represent data dependencies between
// aggregators' message builders, expression node message builders, and other
// sources or expression node message builders.
func NewGraphForInferenceCycleChecks(mc *pb.MetricsConfig) *graph.Graph[InferenceNode] {
	g := graph.NewGraph[InferenceNode]()

	for _, source := range mc.GetSources() {
		g.AddNode(SourceName(source.GetName()))
	}

	for idx, node := range mc.GetExpressionNodes() {
		if node.GetMessageBuilderNode() != nil {
			g.AddNode(MessageBuilderNode(uint32(idx)))
		}
	}

	findDeps := func(rootIndex uint32) []InferenceNode {
		var deps []InferenceNode
		visited := make(map[uint32]bool)
		queue := []uint32{rootIndex}

		for len(queue) > 0 {
			currIdx := queue[0]
			queue = queue[1:]

			if visited[currIdx] {
				continue
			}
			visited[currIdx] = true

			if int(currIdx) >= len(mc.GetExpressionNodes()) {
				continue
			}
			node := mc.GetExpressionNodes()[currIdx]
			switch node.WhichNodeType() {
			case pb.Node_FieldLeafNode_case:
				node := node.GetFieldLeafNode()
				if node.HasExpressionNodeIndex() {
					queue = append(queue, node.GetExpressionNodeIndex())
				} else if sourceName := node.GetSourceName(); sourceName != "" {
					deps = append(deps, SourceName(sourceName))
				}
			case pb.Node_CombinationNode_case:
				node := node.GetCombinationNode()
				if node.HasLeftIndex() {
					queue = append(queue, node.GetLeftIndex())
				}
				if !expressions.IsUnaryOperator(node) && node.HasRightIndex() {
					queue = append(queue, node.GetRightIndex())
				}
			case pb.Node_MessageBuilderNode_case:
				deps = append(deps, MessageBuilderNode(currIdx))
			case pb.Node_FunctionLeafNode_case, pb.Node_ConstantLeafNode_case:
				// These cannot reference another source or message builder.
			}
		}
		return deps
	}

	for _, source := range mc.GetSources() {
		for _, fieldAssignment := range source.GetAggregator().GetMessageBuilder().GetFieldAssignments() {
			if nodeIndex, ok := expressions.ExtractNodeIndex(fieldAssignment); ok {
				for _, dep := range findDeps(nodeIndex) {
					g.AddEdge(SourceName(source.GetName()), dep)
				}
			}
		}
	}

	for idx, node := range mc.GetExpressionNodes() {
		for _, fa := range node.GetMessageBuilderNode().GetFieldAssignments() {
			if fa.HasExpressionNodeIndex() {
				for _, dep := range findDeps(fa.GetExpressionNodeIndex()) {
					g.AddEdge(MessageBuilderNode(uint32(idx)), dep)
				}
			}
		}
	}

	return g
}

// Define a custom node that implements `fmt.Stringer`, so that error messages print a nicer representation than just
// the index of an expression node.
type ExpressionNode uint32

func (e ExpressionNode) String() string {
	return fmt.Sprintf("expression_nodes[%d]", e)
}

var _ fmt.Stringer = ExpressionNode(0)

func NewGraphForExpressionNodeCyclesChecks(mc *pb.MetricsConfig) *graph.Graph[ExpressionNode] {
	g := graph.NewGraph[ExpressionNode]()

	for nodeIdx, node := range mc.GetExpressionNodes() {
		nodeIdx := ExpressionNode(nodeIdx)
		g.AddNode(nodeIdx)

		switch node.WhichNodeType() {
		case pb.Node_CombinationNode_case:
			node := node.GetCombinationNode()
			if node.HasLeftIndex() {
				g.AddEdge(nodeIdx, ExpressionNode(node.GetLeftIndex()))
			}
			if node.HasRightIndex() {
				g.AddEdge(nodeIdx, ExpressionNode(node.GetRightIndex()))
			}
		case pb.Node_FieldLeafNode_case:
			node := node.GetFieldLeafNode()
			if node.HasExpressionNodeIndex() {
				g.AddEdge(nodeIdx, ExpressionNode(node.GetExpressionNodeIndex()))
			}
		case pb.Node_MessageBuilderNode_case:
			node := node.GetMessageBuilderNode()
			for _, fa := range node.GetFieldAssignments() {
				if !fa.HasExpressionNodeIndex() {
					continue
				}
				g.AddEdge(nodeIdx, ExpressionNode(fa.GetExpressionNodeIndex()))
			}
		default:
		}
	}

	return g
}
