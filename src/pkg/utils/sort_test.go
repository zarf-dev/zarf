// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortDependencies(t *testing.T) {
	tests := []struct {
		name     string
		data     []DependsOn // input data: a map of nodes to their dependencies
		expected []string    // expected output: a slice of nodes in order of their precedence
		success  bool        // whether the test should succeed or fail
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
			success:  true,
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
			success:  true,
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
			success:  true,
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
			success:  true,
		},
		{
			name: "graph with circular dependencies",
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
					Name:         "C",
					Dependencies: []string{"A"},
				},
			},
			expected: []string{},
			success:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SortDependencies(tt.data)
			if tt.success {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v but got %v", tt.expected, result)
			}
		})
	}
}
