// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/packager/sources"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
)

func TestGenerateValuesOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		chart         v1alpha1.ZarfChart
		setVariables  map[string]string
		deployOpts    types.ZarfDeployOptions
		componentName string
		want          map[string]any
	}{
		{
			name:          "Empty inputs",
			chart:         v1alpha1.ZarfChart{},
			setVariables:  map[string]string{},
			deployOpts:    types.ZarfDeployOptions{},
			componentName: "",
			want:          map[string]any{},
		},
		{
			name: "Single variable",
			chart: v1alpha1.ZarfChart{
				Name:      "test-chart",
				Variables: []v1alpha1.ZarfChartVariable{{Name: "TEST_VAR", Path: "testVar"}},
			},
			setVariables:  map[string]string{"TEST_VAR": "testValue"},
			deployOpts:    types.ZarfDeployOptions{},
			componentName: "test-component",
			want:          map[string]any{"testVar": "testValue"},
		},
		{
			name: "Non-matching setVariable",
			chart: v1alpha1.ZarfChart{
				Name:      "test-chart",
				Variables: []v1alpha1.ZarfChartVariable{{Name: "EXPECTED_VAR", Path: "path.to.expectedVar"}},
			},
			setVariables:  map[string]string{"UNEXPECTED_VAR": "unexpectedValue"},
			deployOpts:    types.ZarfDeployOptions{},
			componentName: "test-component",
			want:          map[string]any{},
		},
		{
			name: "Nested 3 level setVariables",
			chart: v1alpha1.ZarfChart{
				Name: "nested-chart",
				Variables: []v1alpha1.ZarfChartVariable{
					{Name: "LEVEL1_LEVEL2_LEVEL3_VAR", Path: "level1.level2.level3Var"},
				},
			},
			setVariables:  map[string]string{"LEVEL1_LEVEL2_LEVEL3_VAR": "nestedValue"},
			deployOpts:    types.ZarfDeployOptions{},
			componentName: "nested-component",
			want: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"level3Var": "nestedValue",
					},
				},
			},
		},
		{
			name: "Multiple variables with nested and non-nested paths, distinct values",
			chart: v1alpha1.ZarfChart{
				Name: "mixed-chart",
				Variables: []v1alpha1.ZarfChartVariable{
					{Name: "NESTED_VAR_LEVEL2", Path: "nestedVar.level2"},
					{Name: "SIMPLE_VAR", Path: "simpleVar"},
				},
			},
			setVariables: map[string]string{
				"NESTED_VAR_LEVEL2": "distinctNestedValue",
				"SIMPLE_VAR":        "distinctSimpleValue",
			},
			deployOpts:    types.ZarfDeployOptions{},
			componentName: "mixed-component",
			want: map[string]any{
				"nestedVar": map[string]any{
					"level2": "distinctNestedValue",
				},
				"simpleVar": "distinctSimpleValue",
			},
		},
		{
			name: "Values override test",
			chart: v1alpha1.ZarfChart{
				Name: "test-chart",
				Variables: []v1alpha1.ZarfChartVariable{
					{Name: "OVERRIDE_VAR", Path: "path"},
				},
			},
			setVariables: map[string]string{"OVERRIDE_VAR": "overrideValue"},
			deployOpts: types.ZarfDeployOptions{
				ValuesOverridesMap: map[string]map[string]map[string]any{
					"test-component": {
						"test-chart": {
							"path": "deployOverrideValue",
						},
					},
				},
			},
			componentName: "test-component",
			want: map[string]any{
				"path": "deployOverrideValue",
			},
		},
		{
			name: "Missing variable in setVariables but present in ValuesOverridesMap",
			chart: v1alpha1.ZarfChart{
				Name: "test-chart",
				Variables: []v1alpha1.ZarfChartVariable{
					{Name: "MISSING_VAR", Path: "missingVarPath"},
				},
			},
			setVariables: map[string]string{},
			deployOpts: types.ZarfDeployOptions{
				ValuesOverridesMap: map[string]map[string]map[string]any{
					"test-component": {
						"test-chart": {
							"missingVarPath": "overrideValue",
						},
					},
				},
			},
			componentName: "test-component",
			want: map[string]any{
				"missingVarPath": "overrideValue",
			},
		},
		{
			name: "Non-existent component or chart",
			chart: v1alpha1.ZarfChart{
				Name:      "actual-chart",
				Variables: []v1alpha1.ZarfChartVariable{{Name: "SOME_VAR", Path: "someVar"}},
			},
			setVariables: map[string]string{"SOME_VAR": "value"},
			deployOpts: types.ZarfDeployOptions{
				ValuesOverridesMap: map[string]map[string]map[string]any{
					"non-existent-component": {
						"non-existent-chart": {
							"someVar": "overrideValue",
						},
					},
				},
			},
			componentName: "actual-component",
			want:          map[string]any{"someVar": "value"},
		},
		{
			name:          "Variable in setVariables but not in chartVariables",
			chart:         v1alpha1.ZarfChart{Name: "orphan-chart"},
			setVariables:  map[string]string{"ORPHAN_VAR": "orphanValue"},
			deployOpts:    types.ZarfDeployOptions{},
			componentName: "orphan-component",
			want:          map[string]any{},
		},
		{
			name: "Empty ValuesOverridesMap with non-empty setVariableMap and chartVariables",
			chart: v1alpha1.ZarfChart{
				Name: "chart-with-vars",
				Variables: []v1alpha1.ZarfChartVariable{
					{Name: "VAR1", Path: "path.to.var1"},
					{Name: "VAR2", Path: "path.to.var2"},
					{Name: "VAR3", Path: "path.to3.var3"},
				},
			},
			setVariables: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
				"VAR3": "value3",
			},
			deployOpts:    types.ZarfDeployOptions{},
			componentName: "component-with-vars",
			want: map[string]any{
				"path": map[string]any{
					"to": map[string]any{
						"var1": "value1",
						"var2": "value2",
					},
					"to3": map[string]any{
						"var3": "value3",
					},
				},
			},
		},
		{
			name:  "Empty chartVariables and non-empty setVariableMap",
			chart: v1alpha1.ZarfChart{Name: "chart-with-vars"},
			setVariables: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
			},
			deployOpts:    types.ZarfDeployOptions{},
			componentName: "component-with-vars",
			want:          map[string]any{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(testutil.TestContext(t), &types.PackagerConfig{DeployOpts: tt.deployOpts}, WithSource(&sources.TarballSource{}))
			require.NoError(t, err)
			for k, v := range tt.setVariables {
				p.variableConfig.SetVariable(k, v, false, false, v1alpha1.RawVariableType)
			}

			got, err := p.generateValuesOverrides(tt.chart, tt.componentName)
			if err != nil {
				t.Errorf("%s: generateValuesOverrides() error = %v", tt.name, err)
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func TestServiceInfoFromServiceURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		serviceURL        string
		expectedErr       string
		expectedNamespace string
		expectedName      string
		expectedPort      int
	}{
		{
			name:        "no port",
			serviceURL:  "http://example.com",
			expectedErr: `strconv.Atoi: parsing "": invalid syntax`,
		},
		{
			name:        "normal domain",
			serviceURL:  "http://example.com:8080",
			expectedErr: "unable to match against example.com",
		},
		{
			name:              "valid url",
			serviceURL:        "http://foo.bar.svc.cluster.local:9090",
			expectedNamespace: "bar",
			expectedName:      "foo",
			expectedPort:      9090,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			namespace, name, port, err := serviceInfoFromServiceURL(tt.serviceURL)
			if tt.expectedErr != "" {
				require.EqualError(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedNamespace, namespace)
			require.Equal(t, tt.expectedName, name)
			require.Equal(t, tt.expectedPort, port)
		})
	}
}
