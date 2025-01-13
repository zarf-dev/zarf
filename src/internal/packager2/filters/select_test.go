// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

func Test_selectStateFilter_Apply(t *testing.T) {
	tests := []struct {
		name                string
		requestedComponents string
		components          []v1alpha1.ZarfComponent
		expectedResult      []v1alpha1.ZarfComponent
		expectedError       error
	}{
		{
			name:                "Test when requestedComponents is empty",
			requestedComponents: "",
			components: []v1alpha1.ZarfComponent{
				{Name: "component1"},
				{Name: "component2"},
				{Name: "component3"},
			},
			expectedResult: []v1alpha1.ZarfComponent{
				{Name: "component1"},
				{Name: "component2"},
				{Name: "component3"},
			},
			expectedError: nil,
		},
		{
			name:                "Test when requestedComponents contains a valid component name",
			requestedComponents: "component2",
			components: []v1alpha1.ZarfComponent{
				{Name: "component1"},
				{Name: "component2"},
				{Name: "component3"},
			},
			expectedResult: []v1alpha1.ZarfComponent{
				{Name: "component2"},
			},
			expectedError: nil,
		},
		{
			name:                "Test when requestedComponents contains an excluded component name",
			requestedComponents: "comp*, -component2",
			components: []v1alpha1.ZarfComponent{
				{Name: "component1"},
				{Name: "component2"},
				{Name: "component3"},
			},
			expectedResult: []v1alpha1.ZarfComponent{
				{Name: "component1"},
				{Name: "component3"},
			},
			expectedError: nil,
		},
		{
			name:                "Test when requestedComponents contains a glob pattern",
			requestedComponents: "comp*",
			components: []v1alpha1.ZarfComponent{
				{Name: "component1"},
				{Name: "component2"},
				{Name: "other"},
			},
			expectedResult: []v1alpha1.ZarfComponent{
				{Name: "component1"},
				{Name: "component2"},
			},
			expectedError: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filter := BySelectState(tc.requestedComponents)

			result, err := filter.Apply(v1alpha1.ZarfPackage{
				Components: tc.components,
			})

			require.Equal(t, tc.expectedResult, result)
			require.Equal(t, tc.expectedError, err)
		})
	}
}
