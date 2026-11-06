// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/convert"
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

			pkg := v1alpha1.ZarfPackage{
				Components: tc.components,
			}
			components, err := filter.Apply(convert.PackageFromV1alpha1(pkg))

			require.Equal(t, convert.PackageFromV1alpha1(v1alpha1.ZarfPackage{Components: tc.expectedResult}).Components, components)
			require.Equal(t, tc.expectedError, err)
		})
	}
}

func TestByComponentSelectors(t *testing.T) {
	t.Parallel()

	pkg := api.Package{Components: []api.Component{{Name: "one"}, {Name: "two"}}}
	for _, tt := range []struct {
		name      string
		selectors []string
		want      []api.Component
	}{
		{"nil selects all", nil, pkg.Components},
		{"empty selects all", []string{}, pkg.Components},
		{"blank entries select all", []string{" ", ""}, pkg.Components},
		{"later inclusion wins", []string{" TWO ", "-*", "two"}, pkg.Components[1:]},
		{"later exclusion wins", []string{"two", "-*"}, []api.Component{}},
		{"comma inside pattern", []string{"[o,t]*", "-two"}, pkg.Components[:1]},
	} {
		t.Run(tt.name, func(t *testing.T) {
			components, err := ByComponentSelectors(tt.selectors).Apply(pkg)
			require.NoError(t, err)
			require.Equal(t, tt.want, components)
		})
	}
}

func TestByComponentSelectorsRetainsInputSnapshot(t *testing.T) {
	t.Parallel()

	selectors := []string{" ONE ", ""}
	filter := ByComponentSelectors(selectors)
	require.Equal(t, []string{" ONE ", ""}, selectors)
	selectors[0] = "two"
	pkg := api.Package{Components: []api.Component{{Name: "one"}, {Name: "two"}}}
	components, err := filter.Apply(pkg)
	require.NoError(t, err)
	require.Equal(t, pkg.Components[:1], components)
}

func TestByComponentSelectorsRejectsInvalidPattern(t *testing.T) {
	t.Parallel()

	_, err := ByComponentSelectors([]string{"["}).Apply(api.Package{})
	require.ErrorIs(t, err, path.ErrBadPattern)
}
