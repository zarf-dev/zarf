// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_includedOrExcluded(t *testing.T) {
	tests := []struct {
		name                    string
		componentName           string
		requestedComponentNames []string
		wantState               selectState
		expectedErr             error
	}{
		{
			name:                    "Test when component is excluded",
			componentName:           "example",
			requestedComponentNames: []string{"-example"},
			wantState:               excluded,
		},
		{
			name:                    "Test when component is included",
			componentName:           "example",
			requestedComponentNames: []string{"example"},
			wantState:               included,
		},
		{
			name:                    "Test when component is not included or excluded",
			componentName:           "example",
			requestedComponentNames: []string{"other"},
			wantState:               unknown,
		},
		{
			name:                    "Test when a later include overrides an exclusion",
			componentName:           "example",
			requestedComponentNames: []string{"-example", "example"},
			wantState:               included,
		},
		{
			name:                    "Test when a later exclusion overrides an include",
			componentName:           "example",
			requestedComponentNames: []string{"example", "-example"},
			wantState:               excluded,
		},
		{
			name:                    "Test when component is included via glob",
			componentName:           "example",
			requestedComponentNames: []string{"ex*"},
			wantState:               included,
		},
		{
			name:                    "Test when component is excluded via glob",
			componentName:           "example",
			requestedComponentNames: []string{"-ex*"},
			wantState:               excluded,
		},
		{
			name:                    "Test when component is not found via glob",
			componentName:           "example",
			requestedComponentNames: []string{"other*"},
			wantState:               unknown,
		},
		{
			name:                    "Test error",
			componentName:           "example",
			requestedComponentNames: []string{"["},
			wantState:               unknown,
			expectedErr:             path.ErrBadPattern,
		},
		{
			name:                    "Test error",
			componentName:           "example",
			requestedComponentNames: []string{"-["},
			wantState:               unknown,
			expectedErr:             path.ErrBadPattern,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotState, err := includedOrExcluded(tc.componentName, tc.requestedComponentNames, nil)
			require.ErrorIs(t, err, tc.expectedErr)
			require.Equal(t, tc.wantState, gotState)
		})
	}
}

func TestNormalizeComponentSelectors(t *testing.T) {
	t.Parallel()

	normalized, err := NormalizeComponentSelectors(" B, -A,b,  ")
	require.NoError(t, err)
	require.Equal(t, "b,-a,b", normalized)

	_, err = NormalizeComponentSelectors("[")
	require.ErrorIs(t, err, path.ErrBadPattern)
}
