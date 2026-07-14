// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cluster

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

func TestAdoptZarfManagedLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name:  "nil labels gets managed-by and mutate",
			input: nil,
			expected: map[string]string{
				state.ZarfManagedByLabel: "zarf",
				AgentLabel:               "mutate",
			},
		},
		{
			name:  "empty labels gets managed-by and mutate",
			input: map[string]string{},
			expected: map[string]string{
				state.ZarfManagedByLabel: "zarf",
				AgentLabel:               "mutate",
			},
		},
		{
			name:  "existing ignore label is preserved",
			input: map[string]string{AgentLabel: "ignore"},
			expected: map[string]string{
				state.ZarfManagedByLabel: "zarf",
				AgentLabel:               "ignore",
			},
		},
		{
			name:  "existing skip label is preserved",
			input: map[string]string{AgentLabel: "skip"},
			expected: map[string]string{
				state.ZarfManagedByLabel: "zarf",
				AgentLabel:               "skip",
			},
		},
		{
			name:  "existing mutate label is preserved",
			input: map[string]string{AgentLabel: "mutate"},
			expected: map[string]string{
				state.ZarfManagedByLabel: "zarf",
				AgentLabel:               "mutate",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := AdoptZarfManagedLabels(tt.input)
			require.Equal(t, tt.expected, got)
		})
	}
}
