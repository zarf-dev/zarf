// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import "fmt"

// TODO This file be deleted in v1 once the BB extension is deleted

// Dependency is an interface that represents a node in a list of dependencies.
type Dependency interface {
	Name() string
	Dependencies() []string
}

// SortDependencies performs a topological sort on a dependency graph and
// returns a slice of the nodes in order of their precedence.
// The input data is a map of nodes to a slice of its dependencies.
//
// E.g:
// A depends on B & C, B depends on C and C has no dependencies:
// {"A": ["B", "C"], "B": ["C"], "C": []string{}}
//
// Note sort order is dependent on the slice order of the input data for
// nodes with the same in-degree (i.e. the same number of dependencies).
func SortDependencies(data []Dependency) ([]string, error) {
	// Initialize the in-degree and out-degree maps.
	inDegree := make(map[string]int)
	outDegree := make(map[string][]string)

	// Populate the in-degree and out-degree maps.
	for _, d := range data {
		outDegree[d.Name()] = d.Dependencies()
		inDegree[d.Name()] = 0
	}
	for _, deps := range data {
		for _, d := range deps.Dependencies() {
			inDegree[d]++
		}
	}

	// Initialize the queue and the result list.
	queue := make([]string, 0)
	result := make([]string, 0)

	// Enqueue all nodes with zero in-degree.
	for _, d := range data {
		if inDegree[d.Name()] == 0 {
			queue = append(queue, d.Name())
		}
	}

	// Process the queue.
	for len(queue) > 0 {
		// Dequeue a node from the queue.
		node := queue[0]
		queue = queue[1:]

		// Add the node to the result list.
		result = append([]string{node}, result...)

		// Decrement the in-degree of all outgoing neighbors.
		for _, neighbor := range outDegree[node] {
			inDegree[neighbor]--
			// If the neighbor has zero in-degree, enqueue it.
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// If there are still nodes with non-zero in-degree, there is a cycle in the graph.
	// Return an empty result list to indicate this.
	for _, degree := range inDegree {
		if degree > 0 {
			return result, fmt.Errorf("dependency cycle detected")
		}
	}

	// Return the result list.
	return result, nil
}
