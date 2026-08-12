// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
package packager

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/pkg/variables"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func Test_generateValuesOverrides(t *testing.T) {
	tests := []struct {
		name          string
		chart         v1alpha1.ZarfChart
		componentName string
		opts          overrideOpts
		expect        map[string]any
	}{
		{
			name: "no overrides returns empty map",
			chart: v1alpha1.ZarfChart{
				Name: "test-chart",
			},
			componentName: "test-component",
			opts: overrideOpts{
				variableConfig:     variables.New("", nil, nil),
				values:             value.Values{},
				valuesOverridesMap: ValuesOverrides{},
			},
			expect: map[string]any{},
		},
		{
			name: "chart variables are applied",
			chart: v1alpha1.ZarfChart{
				Name: "test-chart",
				Variables: []v1alpha1.ZarfChartVariable{
					{
						Name: "MY_VAR",
						Path: "image.tag",
					},
				},
			},
			componentName: "test-component",
			opts: overrideOpts{
				variableConfig: func() *variables.VariableConfig {
					vc := variables.New("", nil, nil)
					vc.SetVariable("MY_VAR", "v1.0.0", false, false, "")
					return vc
				}(),
				values:             value.Values{},
				valuesOverridesMap: ValuesOverrides{},
			},
			expect: map[string]any{
				"image": map[string]any{
					"tag": "v1.0.0",
				},
			},
		},
		{
			name: "chart values are mapped from source to target",
			chart: v1alpha1.ZarfChart{
				Name: "test-chart",
				Values: []v1alpha1.ZarfChartValue{
					{
						SourcePath: ".myapp.version",
						TargetPath: ".image.tag",
					},
				},
			},
			componentName: "test-component",
			opts: overrideOpts{
				variableConfig: variables.New("", nil, nil),
				values: value.Values{
					"myapp": map[string]any{
						"version": "2.0.0",
					},
				},
				valuesOverridesMap: ValuesOverrides{},
			},
			expect: map[string]any{
				"image": map[string]any{
					"tag": "2.0.0",
				},
			},
		},
		{
			name: "exclude paths are dropped when mapping source to target",
			chart: v1alpha1.ZarfChart{
				Name: "test-chart",
				Values: []v1alpha1.ZarfChartValue{
					{
						SourcePath: ".loki",
						TargetPath: ".",
						ExcludePaths: []string{
							".loki.image",
						},
					},
				},
			},
			componentName: "test-component",
			opts: overrideOpts{
				variableConfig: variables.New("", nil, nil),
				values: value.Values{
					"loki": map[string]any{
						"replicas": 3,
						"image": map[string]any{
							"repository": "grafana/loki",
						},
					},
				},
				valuesOverridesMap: ValuesOverrides{},
			},
			expect: map[string]any{
				"replicas": 3,
			},
		},
		{
			name: "multiple exclude paths are all dropped",
			chart: v1alpha1.ZarfChart{
				Name: "test-chart",
				Values: []v1alpha1.ZarfChartValue{
					{
						SourcePath: ".loki",
						TargetPath: ".",
						ExcludePaths: []string{
							".loki.image",
							".loki.secret",
						},
					},
				},
			},
			componentName: "test-component",
			opts: overrideOpts{
				variableConfig: variables.New("", nil, nil),
				values: value.Values{
					"loki": map[string]any{
						"replicas": 3,
						"image":    map[string]any{"repository": "grafana/loki"},
						"secret":   "do-not-map",
					},
				},
				valuesOverridesMap: ValuesOverrides{},
			},
			expect: map[string]any{
				"replicas": 3,
			},
		},
		{
			name: "exclude path pointing at a leaf scalar drops only that key",
			chart: v1alpha1.ZarfChart{
				Name: "test-chart",
				Values: []v1alpha1.ZarfChartValue{
					{
						SourcePath: ".loki",
						TargetPath: ".",
						ExcludePaths: []string{
							".loki.secret",
						},
					},
				},
			},
			componentName: "test-component",
			opts: overrideOpts{
				variableConfig: variables.New("", nil, nil),
				values: value.Values{
					"loki": map[string]any{
						"replicas": 3,
						"secret":   "do-not-map",
					},
				},
				valuesOverridesMap: ValuesOverrides{},
			},
			expect: map[string]any{
				"replicas": 3,
			},
		},
		{
			name: "values overrides map is applied",
			chart: v1alpha1.ZarfChart{
				Name: "test-chart",
			},
			componentName: "test-component",
			opts: overrideOpts{
				variableConfig: variables.New("", nil, nil),
				values:         value.Values{},
				valuesOverridesMap: ValuesOverrides{
					"test-component": {
						"test-chart": {
							"replicas": 3,
						},
					},
				},
			},
			expect: map[string]any{
				"replicas": 3,
			},
		},
		{
			name: "all overrides merge with correct precedence",
			chart: v1alpha1.ZarfChart{
				Name: "test-chart",
				Variables: []v1alpha1.ZarfChartVariable{
					{
						Name: "REPLICAS",
						Path: "replicas",
					},
				},
				Values: []v1alpha1.ZarfChartValue{
					{
						SourcePath: ".config.image",
						TargetPath: ".image.repository",
					},
				},
			},
			componentName: "test-component",
			opts: overrideOpts{
				variableConfig: func() *variables.VariableConfig {
					vc := variables.New("", nil, nil)
					vc.SetVariable("REPLICAS", "2", false, false, "")
					return vc
				}(),
				values: value.Values{
					"config": map[string]any{
						"image": "nginx",
					},
				},
				valuesOverridesMap: ValuesOverrides{
					"test-component": {
						"test-chart": {
							"replicas": 5,
							"service": map[string]any{
								"type": "LoadBalancer",
							},
						},
					},
				},
			},
			expect: map[string]any{
				"replicas": 5, // valuesOverridesMap takes precedence over variable
				"image": map[string]any{
					"repository": "nginx",
				},
				"service": map[string]any{
					"type": "LoadBalancer",
				},
			},
		},
		{
			name: "nested variables are set correctly",
			chart: v1alpha1.ZarfChart{
				Name: "test-chart",
				Variables: []v1alpha1.ZarfChartVariable{
					{
						Name: "CPU_LIMIT",
						Path: "resources.limits.cpu",
					},
				},
			},
			componentName: "test-component",
			opts: overrideOpts{
				variableConfig: func() *variables.VariableConfig {
					vc := variables.New("", nil, nil)
					vc.SetVariable("CPU_LIMIT", "500m", false, false, "")
					return vc
				}(),
				values:             value.Values{},
				valuesOverridesMap: ValuesOverrides{},
			},
			expect: map[string]any{
				"resources": map[string]any{
					"limits": map[string]any{
						"cpu": "500m",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)

			result, err := generateValuesOverrides(ctx, tt.chart, tt.componentName, tt.opts)
			require.NoError(t, err)
			require.Equal(t, tt.expect, result)
		})
	}
}

func Test_generateValuesOverrides_Errors(t *testing.T) {
	tests := []struct {
		name          string
		chart         v1alpha1.ZarfChart
		componentName string
		opts          overrideOpts
		errSubstr     string
	}{
		{
			name: "empty source path returns error",
			chart: v1alpha1.ZarfChart{
				Name: "test-chart",
				Values: []v1alpha1.ZarfChartValue{
					{
						SourcePath: "",
						TargetPath: ".image.tag",
					},
				},
			},
			componentName: "test-component",
			opts: overrideOpts{
				variableConfig:     variables.New("", nil, nil),
				values:             value.Values{},
				valuesOverridesMap: ValuesOverrides{},
			},
			errSubstr: "must not be empty",
		},
		{
			name: "empty target path returns error",
			chart: v1alpha1.ZarfChart{
				Name: "test-chart",
				Values: []v1alpha1.ZarfChartValue{
					{
						SourcePath: ".config.image",
						TargetPath: "",
					},
				},
			},
			componentName: "test-component",
			opts: overrideOpts{
				variableConfig:     variables.New("", nil, nil),
				values:             value.Values{},
				valuesOverridesMap: ValuesOverrides{},
			},
			errSubstr: "must not be empty",
		},
		{
			name: "source path without leading dot returns error",
			chart: v1alpha1.ZarfChart{
				Name: "test-chart",
				Values: []v1alpha1.ZarfChartValue{
					{
						SourcePath: "config.image",
						TargetPath: ".image.tag",
					},
				},
			},
			componentName: "test-component",
			opts: overrideOpts{
				variableConfig:     variables.New("", nil, nil),
				values:             value.Values{},
				valuesOverridesMap: ValuesOverrides{},
			},
			errSubstr: "must start with a dot",
		},
		{
			name: "target path without leading dot returns error",
			chart: v1alpha1.ZarfChart{
				Name: "test-chart",
				Values: []v1alpha1.ZarfChartValue{
					{
						SourcePath: ".config.image",
						TargetPath: "image.tag",
					},
				},
			},
			componentName: "test-component",
			opts: overrideOpts{
				variableConfig:     variables.New("", nil, nil),
				values:             value.Values{},
				valuesOverridesMap: ValuesOverrides{},
			},
			errSubstr: "must start with a dot",
		},
		{
			name: "excluding the entire source path leaves nothing to extract",
			chart: v1alpha1.ZarfChart{
				Name: "test-chart",
				Values: []v1alpha1.ZarfChartValue{
					{
						SourcePath:   ".loki",
						TargetPath:   ".",
						ExcludePaths: []string{".loki"},
					},
				},
			},
			componentName: "test-component",
			opts: overrideOpts{
				variableConfig: variables.New("", nil, nil),
				values: value.Values{
					"loki": map[string]any{"replicas": 3},
				},
				valuesOverridesMap: ValuesOverrides{},
			},
			errSubstr: "unable to extract value source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)

			result, err := generateValuesOverrides(ctx, tt.chart, tt.componentName, tt.opts)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errSubstr)
			require.Nil(t, result)
		})
	}
}

func Test_generateValuesOverrides_ExcludePathsDoNotMutateSource(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	vals := value.Values{
		"loki": map[string]any{
			"replicas": 3,
			"image":    map[string]any{"repository": "grafana/loki"},
		},
	}
	chart := v1alpha1.ZarfChart{
		Name: "test-chart",
		Values: []v1alpha1.ZarfChartValue{
			{SourcePath: ".loki", TargetPath: ".", ExcludePaths: []string{".loki.image"}},
		},
	}
	opts := overrideOpts{
		variableConfig:     variables.New("", nil, nil),
		values:             vals,
		valuesOverridesMap: ValuesOverrides{},
	}

	_, err := generateValuesOverrides(ctx, chart, "test-component", opts)
	require.NoError(t, err)

	// The excluded key must still be present in the shared source values.
	loki, ok := vals["loki"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, map[string]any{"repository": "grafana/loki"}, loki["image"])
}

// Two charts share one value.Values: the first excludes a path, the second maps the
// same source without excluding it. The second must still receive the excluded subtree,
// proving the first chart's exclusion does not leak into a sibling chart via shared state.
func Test_generateValuesOverrides_ExcludeDoesNotAffectOtherCharts(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	vals := value.Values{
		"loki": map[string]any{
			"replicas": 3,
			"image":    map[string]any{"repository": "grafana/loki"},
		},
	}
	newOpts := func() overrideOpts {
		return overrideOpts{
			variableConfig:     variables.New("", nil, nil),
			values:             vals,
			valuesOverridesMap: ValuesOverrides{},
		}
	}

	excludingChart := v1alpha1.ZarfChart{
		Name: "chart-a",
		Values: []v1alpha1.ZarfChartValue{
			{SourcePath: ".loki", TargetPath: ".", ExcludePaths: []string{".loki.image"}},
		},
	}
	mappingChart := v1alpha1.ZarfChart{
		Name: "chart-b",
		Values: []v1alpha1.ZarfChartValue{
			{SourcePath: ".loki", TargetPath: "."},
		},
	}

	aResult, err := generateValuesOverrides(ctx, excludingChart, "test-component", newOpts())
	require.NoError(t, err)
	require.Equal(t, map[string]any{"replicas": 3}, aResult)

	bResult, err := generateValuesOverrides(ctx, mappingChart, "test-component", newOpts())
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"replicas": 3,
		"image":    map[string]any{"repository": "grafana/loki"},
	}, bResult)
}
