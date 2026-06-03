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

// NewGraphForInferenceCycleChecks constructs a `Graph` from sources. Edges
// in the graph represent data dependencies from aggregators' message
// builders to other sources.
func NewGraphForInferenceCycleChecks(mc *pb.MetricsConfig) *graph.Graph[string] {
	g := graph.NewGraph[string]()

	for _, pub := range mc.GetSources() {
		g.AddNode(pub.GetName())
		for _, fieldAssignment := range pub.GetAggregator().GetMessageBuilder().GetFieldAssignments() {
			if fieldAssignment == nil {
				continue
			}

			if nodeIndex, ok := expressions.ExtractNodeIndex(fieldAssignment); ok {
				queue := []uint32{nodeIndex}
				for len(queue) > 0 {
					nodeIndex = queue[0]
					queue = queue[1:]

					if node := mc.GetExpressionNodes()[nodeIndex]; node != nil {
						switch node.WhichNodeType() {
						case pb.Node_FieldLeafNode_case:
							fn := node.GetFieldLeafNode()

							g.AddEdge(pub.GetName(), fn.GetSourceName())
						case pb.Node_CombinationNode_case:
							cn := node.GetCombinationNode()

							queue = append(queue, cn.GetLeftIndex())
							if !expressions.IsUnaryOperator(cn) {
								queue = append(queue, cn.GetRightIndex())
							}
						case pb.Node_FunctionLeafNode_case, pb.Node_ConstantLeafNode_case:
							// These cannot reference another source, thus we are done here.
						}
					}
				}
			}
		}
	}
	return g
}
