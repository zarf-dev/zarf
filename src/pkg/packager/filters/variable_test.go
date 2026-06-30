// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

func TestVariableFilter(t *testing.T) {
	t.Parallel()

	pkg := v1alpha1.ZarfPackage{
		Components: []v1alpha1.ZarfComponent{
			{Name: "no-constraint"},
			{Name: "needs-refresh", Only: v1alpha1.ZarfComponentOnlyTarget{Variable: map[string]string{"NEEDS_REFRESH": "true"}}},
			{Name: "needs-seed", Only: v1alpha1.ZarfComponentOnlyTarget{Variable: map[string]string{"NEEDS_SEED": "true"}}},
			{Name: "needs-both", Only: v1alpha1.ZarfComponentOnlyTarget{Variable: map[string]string{"NEEDS_REFRESH": "true", "NEEDS_SEED": "true"}}},
			{Name: "needs-unset", Only: v1alpha1.ZarfComponentOnlyTarget{Variable: map[string]string{"NOT_DECLARED": ""}}},
		},
	}

	tests := []struct {
		name     string
		resolved map[string]string
		want     []string
	}{
		{
			name:     "empty resolved map keeps no-constraint and unset-empty",
			resolved: map[string]string{},
			want:     []string{"no-constraint", "needs-unset"},
		},
		{
			name:     "single match",
			resolved: map[string]string{"NEEDS_REFRESH": "true"},
			want:     []string{"no-constraint", "needs-refresh", "needs-unset"},
		},
		{
			name:     "all match",
			resolved: map[string]string{"NEEDS_REFRESH": "true", "NEEDS_SEED": "true"},
			want:     []string{"no-constraint", "needs-refresh", "needs-seed", "needs-both", "needs-unset"},
		},
		{
			name:     "mismatch drops component",
			resolved: map[string]string{"NEEDS_REFRESH": "false"},
			want:     []string{"no-constraint", "needs-unset"},
		},
		{
			name:     "resolved key with non-empty value drops a component expecting empty",
			resolved: map[string]string{"NOT_DECLARED": "something"},
			want:     []string{"no-constraint"},
		},
		{
			name:     "extra resolved keys ignored when component has no constraint",
			resolved: map[string]string{"UNRELATED": "anything"},
			want:     []string{"no-constraint", "needs-unset"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := ByVariable(tc.resolved).Apply(pkg)
			require.NoError(t, err)
			got := make([]string, 0, len(result))
			for _, c := range result {
				got = append(got, c.Name)
			}
			require.Equal(t, tc.want, got)
		})
	}
}

func TestCheckVariableFilterDropsRequested(t *testing.T) {
	t.Parallel()

	refresh := v1alpha1.ZarfComponent{Name: "refresh-data", Only: v1alpha1.ZarfComponentOnlyTarget{Variable: map[string]string{"NEEDS_REFRESH": "true"}}}
	seed := v1alpha1.ZarfComponent{Name: "seed-data", Only: v1alpha1.ZarfComponentOnlyTarget{Variable: map[string]string{"NEEDS_SEED": "true"}}}
	base := v1alpha1.ZarfComponent{Name: "base"}

	tests := []struct {
		name               string
		before             []v1alpha1.ZarfComponent
		after              []v1alpha1.ZarfComponent
		optionalComponents string
		resolved           map[string]string
		wantErr            string
	}{
		{
			name:               "no requested components is a no-op",
			before:             []v1alpha1.ZarfComponent{refresh, seed, base},
			after:              []v1alpha1.ZarfComponent{base},
			optionalComponents: "",
			resolved:           map[string]string{},
		},
		{
			name:               "requested component survives filter",
			before:             []v1alpha1.ZarfComponent{refresh, base},
			after:              []v1alpha1.ZarfComponent{refresh, base},
			optionalComponents: "refresh-data",
			resolved:           map[string]string{"NEEDS_REFRESH": "true"},
		},
		{
			name:               "explicitly requested component dropped errors with mismatch detail",
			before:             []v1alpha1.ZarfComponent{refresh, base},
			after:              []v1alpha1.ZarfComponent{base},
			optionalComponents: "refresh-data",
			resolved:           map[string]string{"NEEDS_REFRESH": "false"},
			wantErr:            `"refresh-data" filtered by only.variable: NEEDS_REFRESH="false" (want "true")`,
		},
		{
			name:               "excluded request does not error when dropped",
			before:             []v1alpha1.ZarfComponent{refresh, base},
			after:              []v1alpha1.ZarfComponent{base},
			optionalComponents: "-refresh-data",
			resolved:           map[string]string{"NEEDS_REFRESH": "false"},
		},
		{
			name:               "multiple requested drops are aggregated",
			before:             []v1alpha1.ZarfComponent{refresh, seed, base},
			after:              []v1alpha1.ZarfComponent{base},
			optionalComponents: "refresh-data,seed-data",
			resolved:           map[string]string{"NEEDS_REFRESH": "false", "NEEDS_SEED": "false"},
			wantErr:            `requested component(s) excluded by only.variable: "refresh-data" filtered by only.variable: NEEDS_REFRESH="false" (want "true"); "seed-data" filtered by only.variable: NEEDS_SEED="false" (want "true")`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := CheckVariableFilterDropsRequested(tc.before, tc.after, tc.optionalComponents, tc.resolved)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
