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

package graph_test

import (
	"cmp"
	"slices"
	"testing"

	gcmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"sdv.googlesource.com/mcg/mcg/graph"
)

func sortEdgesOpt[Node cmp.Ordered]() gcmp.Option {
	return cmpopts.SortSlices(func(a, b graph.Edge[Node]) int {
		return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To))
	})
}

func TestNewGraph(t *testing.T) {
	g := graph.NewGraph[int]()
	if g == nil {
		t.Fatalf("NewGraph() returned nil")
	}
	if want, got := 0, len(slices.Collect(g.GetNodes())); want != got {
		t.Errorf("NewGraph() created a graph with %d nodes, want %d", got, want)
	}
}

func TestAddNode(t *testing.T) {
	tests := []struct {
		name       string
		nodesToAdd []int
		wantNodes  []int
	}{
		{
			name:       "add_single",
			nodesToAdd: []int{1},
			wantNodes:  []int{1},
		},
		{
			name:       "add_multiple",
			nodesToAdd: []int{1, 2, 3},
			wantNodes:  []int{1, 2, 3},
		},
		{
			name:       "add_duplicate",
			nodesToAdd: []int{1, 1, 2},
			wantNodes:  []int{1, 2},
		},
		{
			name:       "add_none",
			nodesToAdd: []int{},
			wantNodes:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := graph.NewGraph[int]()
			for _, node := range tc.nodesToAdd {
				g.AddNode(node)
			}

			gotNodes := slices.Collect(g.GetNodes())
			if diff := gcmp.Diff(tc.wantNodes, gotNodes, cmpopts.SortSlices(cmp.Less[int]), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("AddNode(%v) resulted in unexpected nodes diff (-want +got):\n%s", tc.nodesToAdd, diff)
			}
		})
	}
}

func TestAddEdge(t *testing.T) {
	tests := []struct {
		name       string
		setupNodes []int
		edgesToAdd []graph.Edge[int]
		wantEdges  []graph.Edge[int]
		wantNodes  []int
	}{
		{
			name:       "new_nodes",
			edgesToAdd: []graph.Edge[int]{{1, 2}},
			wantEdges:  []graph.Edge[int]{{1, 2}},
			wantNodes:  []int{1, 2},
		},
		{
			name:       "existing_nodes",
			setupNodes: []int{1, 2},
			edgesToAdd: []graph.Edge[int]{{1, 2}},
			wantEdges:  []graph.Edge[int]{{1, 2}},
			wantNodes:  []int{1, 2},
		},
		{
			name:       "one_new_node",
			setupNodes: []int{1},
			edgesToAdd: []graph.Edge[int]{{1, 2}},
			wantEdges:  []graph.Edge[int]{{1, 2}},
			wantNodes:  []int{1, 2},
		},
		{
			name:       "duplicate_edge",
			edgesToAdd: []graph.Edge[int]{{1, 2}, {1, 2}},
			wantEdges:  []graph.Edge[int]{{1, 2}},
			wantNodes:  []int{1, 2},
		},
		{
			name:       "self_loop",
			edgesToAdd: []graph.Edge[int]{{1, 1}},
			wantEdges:  []graph.Edge[int]{{1, 1}},
			wantNodes:  []int{1},
		},
		{
			name:       "multiple_edges",
			edgesToAdd: []graph.Edge[int]{{1, 2}, {1, 3}, {2, 3}},
			wantEdges:  []graph.Edge[int]{{1, 2}, {1, 3}, {2, 3}},
			wantNodes:  []int{1, 2, 3},
		},
		{
			name:       "no_edges",
			setupNodes: []int{1, 2},
			edgesToAdd: []graph.Edge[int]{},
			wantEdges:  nil,
			wantNodes:  []int{1, 2},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := graph.NewGraph[int]()
			for _, node := range tc.setupNodes {
				g.AddNode(node)
			}
			for _, edge := range tc.edgesToAdd {
				g.AddEdge(edge.From, edge.To)
			}

			gotNodes := slices.Collect(g.GetNodes())
			if diff := gcmp.Diff(tc.wantNodes, gotNodes, cmpopts.SortSlices(cmp.Less[int]), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("Unexpected nodes diff (-want +got):\n%s", diff)
			}

			gotEdges := slices.Collect(g.GetEdges())
			if diff := gcmp.Diff(tc.wantEdges, gotEdges, sortEdgesOpt[int](), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("Unexpected edges diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetNodes(t *testing.T) {
	tests := []struct {
		name  string
		graph *graph.Graph[string]
		want  []string
	}{
		{
			name:  "empty",
			graph: graph.NewGraph[string](),
			want:  nil,
		},
		{
			name: "nodes_no_edges",
			graph: func() *graph.Graph[string] {
				g := graph.NewGraph[string]()
				g.AddNode("a")
				g.AddNode("b")
				return g
			}(),
			want: []string{"a", "b"},
		},
		{
			name: "nodes_with_edges",
			graph: func() *graph.Graph[string] {
				g := graph.NewGraph[string]()
				g.AddEdge("a", "b")
				g.AddEdge("b", "c")
				g.AddNode("d")
				return g
			}(),
			want: []string{"a", "b", "c", "d"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := slices.Collect(tc.graph.GetNodes())
			if diff := gcmp.Diff(tc.want, got, cmpopts.SortSlices(cmp.Less[string]), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("GetNodes() diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetNeighbors(t *testing.T) {
	g := graph.NewGraph[int]()
	g.AddEdge(1, 2)
	g.AddEdge(1, 3)
	g.AddEdge(2, 2) // Self-loop
	g.AddNode(4)    // Isolated node

	tests := []struct {
		name string
		node int
		want []int
	}{
		{
			name: "multiple_connections",
			node: 1,
			want: []int{2, 3},
		},
		{
			name: "self_loop_connection",
			node: 2,
			want: []int{2},
		},
		{
			name: "no_outgoing",
			node: 3,
			want: nil,
		},
		{
			name: "isolated_node",
			node: 4,
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := slices.Collect(g.GetNeighbors(tc.node))
			if diff := gcmp.Diff(tc.want, got, cmpopts.SortSlices(cmp.Less[int]), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("g.GetNeighbors(%d) returned unexpected result (-want +got):\n%s", tc.node, diff)
			}
		})
	}
}

func TestGetNeighborsNonExistingNode(t *testing.T) {
	g := graph.NewGraph[int]()
	g.AddEdge(1, 2)

	if got := g.GetNeighbors(99); got != nil {
		t.Errorf("g.GetNeighbors(%d) = %v, want nil", 99, got)
	}
}

func TestGetEdges(t *testing.T) {
	tests := []struct {
		name  string
		graph *graph.Graph[int]
		want  []graph.Edge[int]
	}{
		{
			name:  "empty",
			graph: graph.NewGraph[int](),
			want:  nil,
		},
		{
			name: "nodes_no_edges",
			graph: func() *graph.Graph[int] {
				g := graph.NewGraph[int]()
				g.AddNode(1)
				g.AddNode(2)
				return g
			}(),
			want: nil,
		},
		{
			name: "single_edge",
			graph: func() *graph.Graph[int] {
				g := graph.NewGraph[int]()
				g.AddEdge(1, 2)
				return g
			}(),
			want: []graph.Edge[int]{{1, 2}},
		},
		{
			name: "multiple_edges",
			graph: func() *graph.Graph[int] {
				g := graph.NewGraph[int]()
				g.AddEdge(1, 2)
				g.AddEdge(1, 3)
				g.AddEdge(2, 3)
				g.AddEdge(3, 1) // Cycle
				return g
			}(),
			want: []graph.Edge[int]{{1, 2}, {1, 3}, {2, 3}, {3, 1}},
		},
		{
			name: "with_self_loop",
			graph: func() *graph.Graph[int] {
				g := graph.NewGraph[int]()
				g.AddEdge(1, 1)
				g.AddEdge(1, 2)
				return g
			}(),
			want: []graph.Edge[int]{{1, 1}, {1, 2}},
		},
		{
			name: "disconnected_components",
			graph: func() *graph.Graph[int] {
				g := graph.NewGraph[int]()
				g.AddEdge(1, 2)
				g.AddEdge(3, 4)
				g.AddNode(5)
				return g
			}(),
			want: []graph.Edge[int]{{1, 2}, {3, 4}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := slices.Collect(tc.graph.GetEdges())

			if diff := gcmp.Diff(tc.want, got, sortEdgesOpt[int](), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("GetEdges() diff (-want +got):\n%s", diff)
			}
		})
	}
}
