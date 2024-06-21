// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type TestDependency struct {
	name         string
	dependencies []string
}

func (t TestDependency) Name() string {
	return t.name
}

func (t TestDependency) Dependencies() []string {
	return t.dependencies
}

func TestSortDependencies(t *testing.T) {
	tests := []struct {
		name     string
		data     []TestDependency // input data: a map of nodes to their dependencies
		expected []string         // expected output: a slice of nodes in order of their precedence
		success  bool             // whether the test should succeed or fail
	}{
		{
			name: "simple graph",
			data: []TestDependency{
				{
					name:         "A",
					dependencies: []string{"B", "C"},
				},
				{
					name:         "B",
					dependencies: []string{"C"},
				},
				{
					name: "C",
				},
			},
			// C has no dependencies, B depends on C, and A depends on both B and C
			expected: []string{"C", "B", "A"},
			success:  true,
		},
		{
			name: "complex graph",
			data: []TestDependency{
				{
					name:         "A",
					dependencies: []string{"B", "C", "D"},
				},
				{
					name:         "B",
					dependencies: []string{"C", "D", "E"},
				},
				{
					name:         "C",
					dependencies: []string{"E"},
				},
				{
					name:         "D",
					dependencies: []string{"E"},
				},
				{
					name: "E",
				},
			},
			expected: []string{"E", "D", "C", "B", "A"},
			success:  true,
		},
		{
			name: "graph with multiple roots",
			data: []TestDependency{
				{
					name: "A",
				},
				{
					name: "B",
				},
				{
					name:         "C",
					dependencies: []string{"A", "B"},
				},
				{
					name:         "D",
					dependencies: []string{"C", "E"},
				},
				{
					name:         "E",
					dependencies: []string{"F"},
				},
				{
					name: "F",
				},
			},
			expected: []string{"F", "B", "A", "E", "C", "D"},
			success:  true,
		},
		{
			name: "graph with multiple sinks",
			data: []TestDependency{
				{
					name:         "A",
					dependencies: []string{"B"},
				},
				{
					name:         "B",
					dependencies: []string{"C"},
				},
				{
					name: "C",
				},
				{
					name:         "D",
					dependencies: []string{"E"},
				},
				{
					name:         "E",
					dependencies: []string{"F"},
				},
				{
					name: "F",
				},
				{
					name: "G",
				},
			},
			expected: []string{"F", "C", "E", "B", "G", "D", "A"},
			success:  true,
		},
		{
			name: "graph with circular dependencies",
			data: []TestDependency{
				{
					name:         "A",
					dependencies: []string{"B"},
				},
				{
					name:         "B",
					dependencies: []string{"C"},
				},
				{
					name:         "C",
					dependencies: []string{"A"},
				},
			},
			expected: []string{},
			success:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := make([]Dependency, len(tt.data))
			for i := range tt.data {
				deps[i] = tt.data[i]
			}
			result, err := SortDependencies(deps)
			if tt.success {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.Equal(t, tt.expected, result)
		})
	}
}
