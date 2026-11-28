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
		wantRequestedComponent  string
		expectedErr             error
	}{
		{
			name:                    "Test when component is excluded",
			componentName:           "example",
			requestedComponentNames: []string{"-example"},
			wantState:               excluded,
			wantRequestedComponent:  "-example",
		},
		{
			name:                    "Test when component is included",
			componentName:           "example",
			requestedComponentNames: []string{"example"},
			wantState:               included,
			wantRequestedComponent:  "example",
		},
		{
			name:                    "Test when component is not included or excluded",
			componentName:           "example",
			requestedComponentNames: []string{"other"},
			wantState:               unknown,
			wantRequestedComponent:  "",
		},
		{
			name:                    "Test when a later inclusion overrides an exclusion",
			componentName:           "example",
			requestedComponentNames: []string{"-example", "example"},
			wantState:               included,
			wantRequestedComponent:  "-example",
		},
		{
			name:                    "Test when component is included and excluded",
			componentName:           "example",
			requestedComponentNames: []string{"example", "-example"},
			wantState:               excluded,
			wantRequestedComponent:  "-example",
		},
		{
			name:                    "Test when component is included via glob",
			componentName:           "example",
			requestedComponentNames: []string{"ex*"},
			wantState:               included,
			wantRequestedComponent:  "ex*",
		},
		{
			name:                    "Test when component is excluded via glob",
			componentName:           "example",
			requestedComponentNames: []string{"-ex*"},
			wantState:               excluded,
			wantRequestedComponent:  "-ex*",
		},
		{
			name:                    "Test when component is not found via glob",
			componentName:           "example",
			requestedComponentNames: []string{"other*"},
			wantState:               unknown,
			wantRequestedComponent:  "",
		},
		{
			name:                    "Test error",
			componentName:           "example",
			requestedComponentNames: []string{"["},
			wantState:               unknown,
			wantRequestedComponent:  "",
			expectedErr:             path.ErrBadPattern,
		},
		{
			name:                    "Test error",
			componentName:           "example",
			requestedComponentNames: []string{"-["},
			wantState:               unknown,
			wantRequestedComponent:  "",
			expectedErr:             path.ErrBadPattern,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matchedRequests := map[string]bool{}
			gotState, err := includedOrExcluded(tc.componentName, tc.requestedComponentNames, matchedRequests)
			require.ErrorIs(t, err, tc.expectedErr)
			require.Equal(t, tc.wantState, gotState)
			if tc.wantRequestedComponent != "" {
				require.True(t, matchedRequests[tc.wantRequestedComponent])
			}
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
