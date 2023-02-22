// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bigbang contains the logic for installing Big Bang and Flux
package bigbang

// dfsVisit performs a depth-first search on the dependency graph and adds
// visited nodes to a stack in reverse order of their finish times.
func dfsVisit(node string, visited map[string]bool, data map[string][]string, stack *[]string) {
	// If the node has already been visited, do nothing.
	if visited[node] {
		return
	}

	// Mark the node as visited.
	visited[node] = true

	// Visit each child node.
	for _, dependency := range data[node] {
		dfsVisit(dependency, visited, data, stack)
	}

	// Add the node to the stack.
	*stack = append(*stack, node)
}

// sortDependencies performs a topological sort on a dependency graph and
// returns a slice of the nodes in order of their precedence.
func sortDependencies(data map[string][]string) []string {
	// Initialize the visited map and the stack.
	visited := make(map[string]bool)
	stack := make([]string, 0)

	// Perform a depth-first search on each node in the graph.
	for node := range data {
		dfsVisit(node, visited, data, &stack)
	}

	return stack
}
