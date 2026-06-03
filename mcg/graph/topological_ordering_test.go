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
	"fmt"
	"slices"
	"testing"

	gcmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// isValidTopologicalOrdering checks whether the provided `ordering` is a
// valid topological ordering of `g`. That is, if there is an edge from
// node A to node B, then A appears in `ordering` before B.
func isValidTopologicalOrdering[T cmp.Ordered](g *Graph[T], ordering []T) bool {
	if len(slices.Collect(g.GetNodes())) != len(ordering) {
		return false
	}

	pos := make(map[T]int, len(ordering))
	for i, node := range ordering {
		pos[node] = i
	}

	for edge := range g.GetEdges() {
		if pos[edge.From] >= pos[edge.To] {
			return false
		}
	}
	return true
}

// isValidReverseTopologicalOrdering checks whether the provided `ordering` is a
// valid reverse topological ordering of `g`. That is, if there is an edge from
// node A to node B, then B appears in `ordering` before A.
func isValidReverseTopologicalOrdering[T cmp.Ordered](g *Graph[T], ordering []T) bool {
	ordering = slices.Clone(ordering)
	slices.Reverse(ordering)
	return isValidTopologicalOrdering(g, ordering)
}

// cyclesEquivalent checks if two slices with all-unique elements are equal,
// allowing for rotation.
func cyclesEquivalent[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}

	startBIdx := slices.Index(b, a[0])
	if startBIdx == -1 {
		return false
	}

	for i := 0; i < len(a); i++ {
		if a[i] != b[(startBIdx+i)%len(b)] {
			return false
		}
	}
	return true
}

func TestTopologicalOrderingNoCycle(t *testing.T) {
	testCases := []struct {
		name       string
		g          *Graph[string]
		wantStable []string
	}{
		{
			name:       "empty_graph",
			g:          NewGraph[string](),
			wantStable: []string{},
		},
		{
			name: "graph_with_one_node",
			g: func() *Graph[string] {
				g := NewGraph[string]()
				g.AddNode("a")
				return g
			}(),
			wantStable: []string{"a"},
		},
		{
			name: "simple_dag",
			g: func() *Graph[string] {
				g := NewGraph[string]()
				g.AddEdge("a", "b")
				g.AddEdge("b", "c")
				return g
			}(),
			wantStable: []string{"a", "b", "c"},
		},
		{
			name: "complex_dag",
			g: func() *Graph[string] {
				g := NewGraph[string]()
				g.AddEdge("a", "b")
				g.AddEdge("a", "c")
				g.AddEdge("b", "d")
				g.AddEdge("c", "d")
				return g
			}(),
			wantStable: []string{"a", "c", "b", "d"},
		},
		{
			name: "disconnected_components",
			g: func() *Graph[string] {
				g := NewGraph[string]()
				g.AddEdge("a", "b")
				g.AddEdge("c", "d")
				return g
			}(),
			wantStable: []string{"c", "d", "a", "b"},
		},
		{
			name: "larger_graph",
			g: func() *Graph[string] {
				g := NewGraph[string]()
				g.AddEdge("a", "b")
				g.AddEdge("a", "c")
				g.AddEdge("b", "d")
				g.AddEdge("c", "e")
				g.AddEdge("d", "f")
				g.AddEdge("e", "f")
				return g
			}(),
			wantStable: []string{"a", "c", "e", "b", "d", "f"},
		},
	}

	for _, tc := range testCases {
		// Double-check that the `wantStable` ordering is actually a valid
		// topological ordering.
		if !isValidTopologicalOrdering(tc.g, tc.wantStable) {
			t.Fatalf("isValidTopologicalOrdering(tc.g, tc.want) = false, want true: %v is not a valid topological ordering for %v", tc.wantStable, tc.g)
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Run("stable_normal", func(t *testing.T) {
				ordering, err := tc.g.StableTopologicalOrdering()
				if err != nil {
					t.Fatalf("tc.g.StableTopologicalOrdering() failed: %v", err)
				}

				if diff := gcmp.Diff(tc.wantStable, ordering, cmpopts.EquateEmpty()); diff != "" {
					t.Errorf("tc.g.StableTopologicalOrdering() returned diff (-want, +got):\n%s", diff)
				}
			})
			t.Run("stable_reverse", func(t *testing.T) {
				ordering, err := tc.g.StableReverseTopologicalOrdering()
				if err != nil {
					t.Fatalf("tc.g.StableReverseTopologicalOrdering() failed: %v", err)
				}

				wantStableReverse := slices.Clone(tc.wantStable)
				slices.Reverse(wantStableReverse)
				if diff := gcmp.Diff(wantStableReverse, ordering, cmpopts.EquateEmpty()); diff != "" {
					t.Errorf("tc.g.StableReverseTopologicalOrdering() returned diff (-want, +got):\n%s", diff)
				}
			})
			t.Run("unstable_normal", func(t *testing.T) {
				ordering, err := tc.g.TopologicalOrdering()
				if err != nil {
					t.Fatalf("tc.g.TopologicalOrdering() failed: %v", err)
				}

				// As topological sort can have multiple valid orderings, thus, we
				// need to check if the returned ordering is any of the valid ones.
				if !isValidTopologicalOrdering(tc.g, ordering) {
					t.Errorf("tc.g.TopologicalOrdering() = %v, which is not a valid topological ordering for the graph", ordering)
				}
			})
			t.Run("unstable_reverse", func(t *testing.T) {
				ordering, err := tc.g.ReverseTopologicalOrdering()
				if err != nil {
					t.Fatalf("tc.g.ReverseTopologicalOrdering() failed: %v", err)
				}

				// As topological sort can have multiple valid orderings, thus, we
				// need to check if the returned ordering is any of the valid ones.
				if !isValidReverseTopologicalOrdering(tc.g, ordering) {
					t.Errorf("tc.g.ReverseTopologicalOrdering() = %v, which is not a valid topological ordering for the graph", ordering)
				}
			})
		})
	}
}

func TestTopologicalOrderingCycle(t *testing.T) {
	testCases := []struct {
		name      string
		g         *Graph[string]
		wantCycle []string
	}{
		{
			name: "self_reference",
			g: func() *Graph[string] {
				g := NewGraph[string]()
				g.AddEdge("a", "a")
				return g
			}(),
			wantCycle: []string{"a"},
		},
		{
			name: "two_nodes_cycle",
			g: func() *Graph[string] {
				g := NewGraph[string]()
				g.AddEdge("a", "b")
				g.AddEdge("b", "a")
				return g
			}(),
			wantCycle: []string{"a", "b"},
		},
		{
			name: "three_nodes_cycle",
			g: func() *Graph[string] {
				g := NewGraph[string]()
				g.AddEdge("a", "b")
				g.AddEdge("b", "c")
				g.AddEdge("c", "a")
				return g
			}(),
			wantCycle: []string{"a", "b", "c"},
		},
		{
			name: "cycle_not_including_all_nodes",
			g: func() *Graph[string] {
				g := NewGraph[string]()
				g.AddEdge("a", "b")
				g.AddEdge("b", "c")
				g.AddEdge("c", "b")
				return g
			}(),
			wantCycle: []string{"b", "c"},
		},
		{
			name: "complex_cycle",
			g: func() *Graph[string] {
				g := NewGraph[string]()
				g.AddEdge("a", "b")
				g.AddEdge("b", "c")
				g.AddEdge("c", "d")
				g.AddEdge("d", "b")
				g.AddEdge("d", "e")
				return g
			}(),
			wantCycle: []string{"b", "c", "d"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("stable_normal", func(t *testing.T) {
				ordering, err := tc.g.StableTopologicalOrdering()
				if err == nil {
					t.Fatalf("tc.g.StableTopologicalOrdering() = %v, nil, want error", ordering)
				}
				cyclicGraphError, ok := err.(*CyclicGraphError[string])
				if !ok {
					t.Fatalf("tc.g.StableTopologicalOrdering() returned error of type %T, want *CyclicGraphError[string]", err)
				}

				if diff := gcmp.Diff(tc.wantCycle, cyclicGraphError.cycle); diff != "" {
					t.Errorf("Incorrect cycle detected. diff (-want +got):\n%s", diff)
				}
			})
			t.Run("stable_reverse", func(t *testing.T) {
				ordering, err := tc.g.StableReverseTopologicalOrdering()
				if err == nil {
					t.Fatalf("tc.g.StableReverseTopologicalOrdering() = %v, nil, want error", ordering)
				}
				cyclicGraphError, ok := err.(*CyclicGraphError[string])
				if !ok {
					t.Fatalf("tc.g.StableReverseTopologicalOrdering() returned error of type %T, want *CyclicGraphError[string]", err)
				}

				if diff := gcmp.Diff(tc.wantCycle, cyclicGraphError.cycle); diff != "" {
					t.Errorf("Incorrect cycle detected. diff (-want +got):\n%s", diff)
				}
			})
			t.Run("unstable_normal", func(t *testing.T) {
				ordering, err := tc.g.TopologicalOrdering()
				if err == nil {
					t.Fatalf("tc.g.TopologicalOrdering() = %v, nil, want error", ordering)
				}
				cyclicGraphError, ok := err.(*CyclicGraphError[string])
				if !ok {
					t.Fatalf("tc.g.TopologicalOrdering() returned error of type %T, want *CyclicGraphError[string]", err)
				}

				if gotCycle := cyclicGraphError.cycle; !cyclesEquivalent(tc.wantCycle, gotCycle) {
					t.Errorf("Incorrect cycle detected. got %v, want %v (or a rotation of it)", gotCycle, tc.wantCycle)
				}
			})
			t.Run("unstable_reverse", func(t *testing.T) {
				ordering, err := tc.g.ReverseTopologicalOrdering()
				if err == nil {
					t.Fatalf("tc.g.ReverseTopologicalOrdering() = %v, nil, want error", ordering)
				}
				cyclicGraphError, ok := err.(*CyclicGraphError[string])
				if !ok {
					t.Fatalf("tc.g.ReverseTopologicalOrdering() returned error of type %T, want *CyclicGraphError[string]", err)
				}

				if gotCycle := cyclicGraphError.cycle; !cyclesEquivalent(tc.wantCycle, gotCycle) {
					t.Errorf("Incorrect cycle detected. got %v, want %v (or a rotation of it)", gotCycle, tc.wantCycle)
				}
			})
		})
	}
}

func TestTopologicalOrderingDeterministic(t *testing.T) {
	g := NewGraph[string]()
	for node := 'b'; node < 's'; node++ {
		g.AddEdge(string(node), "a")
	}

	// This many nodes make it extremely unlikely that a non-deterministic
	// implementation isn't caught.
	want := []string{"r", "q", "p", "o", "n", "m", "l", "k", "j", "i", "h", "g", "f", "e", "d", "c", "b", "a"}
	got, err := g.StableTopologicalOrdering()
	if err != nil {
		t.Fatalf("StableTopologicalOrdering() failed: %v", err)
	}

	if diff := gcmp.Diff(want, got); diff != "" {
		t.Errorf("StableTopologicalOrdering() returned non-deterministic result (-want +got):\n%s", diff)
	}
}

// Define a custom node that implements `fmt.Stringer`, so that we can check
// that the error message uses the custom stringer for formatting the node.
type MyNode string

func (m MyNode) String() string {
	return fmt.Sprintf("MyNode<%q>", string(m))
}

var _ fmt.Stringer = (*MyNode)(nil)

func TestCyclicGraphErrorString(t *testing.T) {
	err := &CyclicGraphError[MyNode]{
		cycle: []MyNode{MyNode("a"), MyNode("b"), MyNode("c")},
	}

	got := err.Error()
	want := `cycle detected: MyNode<"a"> → MyNode<"b"> → MyNode<"c"> → MyNode<"a">`
	if diff := gcmp.Diff(want, got); diff != "" {
		t.Errorf("err.Error() returned diff (-want, +got): %s", diff)
	}
}
