// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
package packager

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/feature"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/pkg/variables"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestMain(m *testing.M) {
	if err := feature.Set([]feature.Feature{
		{Name: feature.Values, Enabled: true},
		{Name: feature.V1Beta1, Enabled: true},
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func Test_generateValuesOverrides(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		overrides ValuesOverrides
		want      map[string]any
	}{
		{name: "absent overrides", want: map[string]any{}},
		{
			name: "other component is ignored",
			overrides: ValuesOverrides{
				"other-component": {"test-chart": {"replicas": 9}},
			},
			want: map[string]any{},
		},
		{
			name: "other chart is ignored",
			overrides: ValuesOverrides{
				"test-component": {"other-chart": {"replicas": 9}},
			},
			want: map[string]any{},
		},
		{
			name: "matching component and chart are selected",
			overrides: ValuesOverrides{
				"test-component": {
					"test-chart":  {"replicas": 3},
					"other-chart": {"replicas": 9, "wrongChart": true},
				},
				"other-component": {"test-chart": {"replicas": 9, "wrongComponent": true}},
			},
			want: map[string]any{"replicas": 3},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			vc := variables.New("", nil, nil)
			vc.SetVariable("TAG", "v1.0.0", false, false, "")
			chart := api.Chart{
				Name:      "test-chart",
				Variables: []api.ZarfChartVariable{{Name: "TAG", Path: "image.tag"}},
				Values:    []api.ChartValue{{SourcePath: ".repository", TargetPath: ".image.repository"}},
			}
			got, err := generateValuesOverrides(testutil.TestContext(t), chart, "test-component", overrideOpts{
				variableConfig:     vc,
				values:             value.Values{"repository": "nginx"},
				valuesOverridesMap: tt.overrides,
			})
			require.NoError(t, err)
			tt.want["image"] = map[string]any{"tag": "v1.0.0", "repository": "nginx"}
			require.Equal(t, tt.want, got)
		})
	}

	t.Run("lower error is propagated", func(t *testing.T) {
		t.Parallel()
		chart := api.Chart{Values: []api.ChartValue{{TargetPath: ".image.tag"}}}
		got, err := generateValuesOverrides(testutil.TestContext(t), chart, "test-component", overrideOpts{
			variableConfig: variables.New("", nil, nil),
		})
		require.EqualError(t, err, `sourcePath "" and targetPath ".image.tag" must not be empty`)
		require.Nil(t, got)
	})
}
