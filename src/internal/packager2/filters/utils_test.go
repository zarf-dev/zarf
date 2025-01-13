// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
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
			name:                    "Test when component is excluded and included",
			componentName:           "example",
			requestedComponentNames: []string{"-example", "example"},
			wantState:               excluded,
			wantRequestedComponent:  "-example",
		},
		// interesting case, excluded wins
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotState, gotRequestedComponent := includedOrExcluded(tc.componentName, tc.requestedComponentNames)
			require.Equal(t, tc.wantState, gotState)
			require.Equal(t, tc.wantRequestedComponent, gotRequestedComponent)
		})
	}
}
