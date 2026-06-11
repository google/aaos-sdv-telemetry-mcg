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
	"fmt"
	"iter"
	"slices"
	"strings"
)

// CyclicGraphError indicates that a cycle was found in the graph.
type CyclicGraphError[Node any] struct {
	cycle []Node
}

func (e *CyclicGraphError[Node]) Error() string {
	const arrow = " → "

	s := new(strings.Builder)
	for _, n := range e.cycle {
		s.WriteString(fmt.Sprintf("%v", n))
		s.WriteString(arrow)
	}
	// Close the cycle by printing the first node again.
	s.WriteString(fmt.Sprintf("%v", e.cycle[0]))

	return fmt.Sprintf("cycle detected: %s", s.String())
}

const (
	unvisited = iota
	visiting
	visited
)

func (g *Graph[Node]) topologicalOrdering(
	from Node,
	states map[Node]int,
	path *[]Node,
	result *[]Node,
	getNeighbors func(Node) iter.Seq[Node],
) error {
	states[from] = visiting
	*path = append(*path, from)

	for to := range getNeighbors(from) {
		switch states[to] {
		case unvisited:
			if err := g.topologicalOrdering(to, states, path, result, getNeighbors); err != nil {
				return err
			}
		case visiting:
			// The cycle starts from the first time 'to' appears in the current path.
			return &CyclicGraphError[Node]{cycle: (*path)[slices.Index(*path, to):]}
		case visited:
			// Already visited, nothing to do.
		}
	}

	states[from] = visited
	*path = (*path)[:len(*path)-1]
	*result = slices.Insert(*result, 0, from)

	return nil
}

// TopologicalOrdering returns a topological ordering of the graph. That is, if
// there is an edge from node A to node B, then A will occur before B in the
// resulting ordering. If the graph contains cycles, a `CyclicGraphError` is
// returned instead.
//
// Note: This function is not guaranteed to return stable results.
func (g *Graph[Node]) TopologicalOrdering() ([]Node, error) {
	nodes := slices.Collect(g.GetNodes())

	result := []Node{}
	states := make(map[Node]int, len(nodes))
	for _, node := range nodes {
		states[node] = unvisited
	}

	for _, node := range nodes {
		if states[node] == unvisited {
			if err := g.topologicalOrdering(node, states, &[]Node{}, &result, g.GetNeighbors); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

// ReverseTopologicalOrdering returns a reversed topological ordering of the
// graph. That is, if there is an edge from node A to node B, then B will occur
// before A in the resulting ordering. If the graph contains cycles, a
// `CyclicGraphError` is returned instead.
//
// Note: This function is not guaranteed to return stable results.
func (g *Graph[Node]) ReverseTopologicalOrdering() ([]Node, error) {
	nodes, err := g.TopologicalOrdering()
	if err != nil {
		return nil, err
	}
	slices.Reverse(nodes)
	return nodes, nil
}

// StableTopologicalOrdering returns a stable topological ordering of the graph.
// That is, if there is an edge from node A to node B, then A will occur before
// B in the resulting ordering. If the graph contains cycles, a
// `CyclicGraphError` is returned instead.
func (g *Graph[Node]) StableTopologicalOrdering() ([]Node, error) {
	nodes := slices.Collect(g.GetNodes())
	slices.Sort(nodes)

	result := []Node{}
	states := make(map[Node]int, len(nodes))
	for _, node := range nodes {
		states[node] = unvisited
	}

	getNeighborsStable := func(n Node) iter.Seq[Node] {
		neighbors := slices.Collect(g.GetNeighbors(n))
		slices.Sort(neighbors)
		return slices.Values(neighbors)
	}

	for _, node := range nodes {
		if states[node] == unvisited {
			if err := g.topologicalOrdering(node, states, &[]Node{}, &result, getNeighborsStable); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

// StableReverseTopologicalOrdering returns a reversed stable topological
// ordering of the graph. That is, if there is an edge from node A to node B,
// then B will occur before A in the resulting ordering. If the graph contains
// cycles, a `CyclicGraphError` is returned instead.
func (g *Graph[Node]) StableReverseTopologicalOrdering() ([]Node, error) {
	nodes, err := g.StableTopologicalOrdering()
	if err != nil {
		return nil, err
	}
	slices.Reverse(nodes)
	return nodes, nil
}
