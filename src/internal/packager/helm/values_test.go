// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helm

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

func TestGenerateChartOverridesPrecedence(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		mappings  []api.ChartValue
		overrides map[string]any
		want      string
	}{
		{name: "variable", want: "variable"},
		{
			name:     "mapped value overrides variable",
			mappings: []api.ChartValue{{SourcePath: ".tag", TargetPath: ".image.tag"}},
			want:     "mapped",
		},
		{
			name:      "direct override overrides mapped value",
			mappings:  []api.ChartValue{{SourcePath: ".tag", TargetPath: ".image.tag"}},
			overrides: map[string]any{"image": map[string]any{"tag": "direct"}},
			want:      "direct",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			vc := variables.New("", nil, nil)
			vc.SetVariable("TAG", "variable", false, false, "")
			chart := api.Chart{
				Variables: []api.ZarfChartVariable{
					{Name: "TAG", Path: "image.tag"},
					{Name: "REPOSITORY", Path: "image.repository"},
				},
				Values: tt.mappings,
			}
			vc.SetVariable("REPOSITORY", "nginx", false, false, "")

			got, err := GenerateChartOverrides(chart, vc, value.Values{"tag": "mapped"}, tt.overrides)
			require.NoError(t, err)
			require.Equal(t, map[string]any{
				"image": map[string]any{"tag": tt.want, "repository": "nginx"},
			}, got)
		})
	}
}

func TestGenerateChartOverrides(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		chart api.Chart
		vals  value.Values
		want  map[string]any
	}{
		{name: "no overrides", want: map[string]any{}},
		{
			name: "nested variables and unset variable",
			chart: api.Chart{Variables: []api.ZarfChartVariable{
				{Name: "CPU_LIMIT", Path: "resources.limits.cpu"},
				{Name: "UNSET", Path: "resources.limits.memory"},
			}},
			want: map[string]any{"resources": map[string]any{"limits": map[string]any{"cpu": "500m"}}},
		},
		{
			name: "nested source maps to nested target",
			chart: api.Chart{Values: []api.ChartValue{
				{SourcePath: ".myapp.version", TargetPath: ".image.tag"},
			}},
			vals: value.Values{"myapp": map[string]any{"version": "2.0.0"}},
			want: map[string]any{"image": map[string]any{"tag": "2.0.0"}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			vc := variables.New("", nil, nil)
			vc.SetVariable("CPU_LIMIT", "500m", false, false, "")
			got, err := GenerateChartOverrides(tt.chart, vc, tt.vals, nil)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGenerateChartOverridesErrors(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		chart     api.Chart
		errSubstr string
	}{
		{
			name:      "empty source path",
			chart:     api.Chart{Values: []api.ChartValue{{TargetPath: ".image.tag"}}},
			errSubstr: "must not be empty",
		},
		{
			name:      "empty target path",
			chart:     api.Chart{Values: []api.ChartValue{{SourcePath: ".tag"}}},
			errSubstr: "must not be empty",
		},
		{
			name:      "source path without leading dot",
			chart:     api.Chart{Values: []api.ChartValue{{SourcePath: "tag", TargetPath: ".image.tag"}}},
			errSubstr: `sourcePath "tag" must start with a dot`,
		},
		{
			name:      "target path without leading dot",
			chart:     api.Chart{Values: []api.ChartValue{{SourcePath: ".tag", TargetPath: "image.tag"}}},
			errSubstr: `targetPath "image.tag" must start with a dot`,
		},
		{
			name: "excluding entire source leaves nothing to extract",
			chart: api.Chart{Values: []api.ChartValue{
				{SourcePath: ".loki", TargetPath: ".", ExcludePaths: []string{".loki"}},
			}},
			errSubstr: "unable to extract value source",
		},
		{
			name: "invalid exclusion",
			chart: api.Chart{Values: []api.ChartValue{
				{SourcePath: ".loki", TargetPath: ".", ExcludePaths: []string{"."}},
			}},
			errSubstr: "unable to exclude path .: cannot delete root path",
		},
		{
			name:      "scalar cannot map to root",
			chart:     api.Chart{Values: []api.ChartValue{{SourcePath: ".tag", TargetPath: "."}}},
			errSubstr: "unable to map value from .tag to .: cannot merge non-map value at root path",
		},
		{
			name:      "variable cannot set root to scalar",
			chart:     api.Chart{Variables: []api.ZarfChartVariable{{Name: "TAG"}}},
			errSubstr: "unable to set value at path .: cannot merge non-map value at root path",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			vc := variables.New("", nil, nil)
			vc.SetVariable("TAG", "v1.0.0", false, false, "")
			got, err := GenerateChartOverrides(tt.chart, vc, value.Values{
				"tag":  "v1.0.0",
				"loki": map[string]any{"replicas": 3},
			}, nil)
			require.ErrorContains(t, err, tt.errSubstr)
			require.Nil(t, got)
		})
	}
}

// Two charts share one value.Values: the first excludes a path, the second maps the
// same source without excluding it. The second must still receive the excluded subtree,
// proving the first chart's exclusion does not leak into a sibling chart via shared state.
func TestGenerateChartOverridesExclusionsPreserveSource(t *testing.T) {
	t.Parallel()
	vals := value.Values{
		"loki": map[string]any{
			"replicas": 3,
			"image":    map[string]any{"repository": "grafana/loki"},
			"secret":   "do-not-map",
		},
	}
	vc := variables.New("", nil, nil)
	excludingChart := api.Chart{Values: []api.ChartValue{
		{SourcePath: ".loki", TargetPath: ".", ExcludePaths: []string{".loki.image", ".loki.secret"}},
	}}
	got, err := GenerateChartOverrides(excludingChart, vc, vals, nil)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"replicas": 3}, got)

	// The excluded key must still be present in the shared source values.
	wantSource := map[string]any{
		"replicas": 3,
		"image":    map[string]any{"repository": "grafana/loki"},
		"secret":   "do-not-map",
	}
	require.Equal(t, wantSource, vals["loki"])

	mappingChart := api.Chart{Values: []api.ChartValue{{SourcePath: ".loki", TargetPath: "."}}}
	got, err = GenerateChartOverrides(mappingChart, vc, vals, nil)
	require.NoError(t, err)
	require.Equal(t, wantSource, got)
}
