// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"reflect"
	"testing"
)

func TestSortDependencies(t *testing.T) {
	tests := []struct {
		name     string
		data     []DependsOn // input data: a map of nodes to their dependencies
		expected []string    // expected output: a slice of nodes in order of their precedence
	}{
		{
			name: "simple graph",
			data: []DependsOn{
				{
					Name:         "A",
					Dependencies: []string{"B", "C"},
				},
				{
					Name:         "B",
					Dependencies: []string{"C"},
				},
				{
					Name: "C",
				},
			},
			// C has no dependencies, B depends on C, and A depends on both B and C
			expected: []string{"C", "B", "A"},
		},
		{
			name: "complex graph",
			data: []DependsOn{
				{
					Name:         "A",
					Dependencies: []string{"B", "C", "D"},
				},
				{
					Name:         "B",
					Dependencies: []string{"C", "D", "E"},
				},
				{
					Name:         "C",
					Dependencies: []string{"E"},
				},
				{
					Name:         "D",
					Dependencies: []string{"E"},
				},
				{
					Name: "E",
				},
			},
			expected: []string{"E", "D", "C", "B", "A"},
		},
		{
			name: "graph with multiple roots",
			data: []DependsOn{
				{
					Name: "A",
				},
				{
					Name: "B",
				},
				{
					Name:         "C",
					Dependencies: []string{"A", "B"},
				},
				{
					Name:         "D",
					Dependencies: []string{"C", "E"},
				},
				{
					Name:         "E",
					Dependencies: []string{"F"},
				},
				{
					Name: "F",
				},
			},
			expected: []string{"F", "B", "A", "E", "C", "D"},
		},
		{
			name: "graph with multiple sinks",
			data: []DependsOn{
				{
					Name:         "A",
					Dependencies: []string{"B"},
				},
				{
					Name:         "B",
					Dependencies: []string{"C"},
				},
				{
					Name: "C",
				},
				{
					Name:         "D",
					Dependencies: []string{"E"},
				},
				{
					Name:         "E",
					Dependencies: []string{"F"},
				},
				{
					Name: "F",
				},
				{
					Name: "G",
				},
			},
			expected: []string{"F", "C", "E", "B", "G", "D", "A"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SortDependencies(tt.data)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v but got %v", tt.expected, result)
			}
		})
	}
}
