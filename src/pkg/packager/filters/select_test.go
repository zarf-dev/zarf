// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/types"
)

func Test_selectStateFilter_Apply(t *testing.T) {
	tests := []struct {
		name                string
		requestedComponents string
		components          []types.ZarfComponent
		expectedResult      []types.ZarfComponent
		expectedError       error
	}{
		{
			name:                "Test when requestedComponents is empty",
			requestedComponents: "",
			components: []types.ZarfComponent{
				{Name: "component1"},
				{Name: "component2"},
				{Name: "component3"},
			},
			expectedResult: []types.ZarfComponent{
				{Name: "component1"},
				{Name: "component2"},
				{Name: "component3"},
			},
			expectedError: nil,
		},
		{
			name:                "Test when requestedComponents contains a valid component name",
			requestedComponents: "component2",
			components: []types.ZarfComponent{
				{Name: "component1"},
				{Name: "component2"},
				{Name: "component3"},
			},
			expectedResult: []types.ZarfComponent{
				{Name: "component2"},
			},
			expectedError: nil,
		},
		{
			name:                "Test when requestedComponents contains an excluded component name",
			requestedComponents: "comp*, -component2",
			components: []types.ZarfComponent{
				{Name: "component1"},
				{Name: "component2"},
				{Name: "component3"},
			},
			expectedResult: []types.ZarfComponent{
				{Name: "component1"},
				{Name: "component3"},
			},
			expectedError: nil,
		},
		{
			name:                "Test when requestedComponents contains a glob pattern",
			requestedComponents: "comp*",
			components: []types.ZarfComponent{
				{Name: "component1"},
				{Name: "component2"},
				{Name: "other"},
			},
			expectedResult: []types.ZarfComponent{
				{Name: "component1"},
				{Name: "component2"},
			},
			expectedError: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filter := BySelectState(tc.requestedComponents)

			result, err := filter.Apply(types.ZarfPackage{
				Components: tc.components,
			})

			require.Equal(t, tc.expectedResult, result)
			require.Equal(t, tc.expectedError, err)
		})
	}
}
