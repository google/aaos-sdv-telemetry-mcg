// Copyright 2025 Google LLC
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

package graph

import (
	"cmp"
	"iter"
	"maps"
)

// Graph is a directed graph of nodes.
type Graph[Node cmp.Ordered] struct {
	// equivalent to Map<Node, Set<Node>>
	nodes map[Node]map[Node]bool
}

// NewGraph creates a new graph.
func NewGraph[Node cmp.Ordered]() *Graph[Node] {
	return &Graph[Node]{
		nodes: make(map[Node]map[Node]bool),
	}
}

// AddNode adds a new node to the graph.
func (g *Graph[Node]) AddNode(node Node) {
	if _, ok := g.nodes[node]; ok {
		// Node already exists.
		return
	}
	g.nodes[node] = make(map[Node]bool)
}

// AddEdge adds a directed edge from `nodeFrom` to `nodeTo`. It creates the
// nodes if they do not yet exist.
func (g *Graph[Node]) AddEdge(nodeFrom, nodeTo Node) {
	g.AddNode(nodeFrom)
	g.AddNode(nodeTo)
	g.nodes[nodeFrom][nodeTo] = true
}

// GetNodes returns an iterator over all nodes of the graph.
func (g *Graph[Node]) GetNodes() iter.Seq[Node] {
	return maps.Keys(g.nodes)
}

// GetNeighbors returns an iterator over all nodes that are directly connected
// to the provided node. If the node does not exist in the graph, `nil` is
// returned instead.
func (g *Graph[Node]) GetNeighbors(node Node) iter.Seq[Node] {
	neighbors, ok := g.nodes[node]
	if !ok {
		return nil
	}
	return maps.Keys(neighbors)
}

type Edge[Node cmp.Ordered] struct {
	From Node
	To   Node
}

// GetEdges returns an iterator of edges.
func (g *Graph[Node]) GetEdges() iter.Seq[Edge[Node]] {
	return func(yield func(Edge[Node]) bool) {
		for from := range g.GetNodes() {
			for to := range g.GetNeighbors(from) {
				if !yield(Edge[Node]{From: from, To: to}) {
					return
				}
			}
		}
	}
}
