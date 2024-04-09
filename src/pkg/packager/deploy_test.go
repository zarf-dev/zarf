// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"reflect"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/variables"
	"github.com/defenseunicorns/zarf/src/types"
)

func TestGenerateValuesOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		chartVariables []types.ZarfChartVariable
		setVariableMap variables.SetVariableMap
		deployOpts     types.ZarfDeployOptions
		componentName  string
		chartName      string
		want           map[string]any
	}{
		{
			name:           "Empty inputs",
			chartVariables: []types.ZarfChartVariable{},
			setVariableMap: variables.SetVariableMap{},
			deployOpts:     types.ZarfDeployOptions{},
			componentName:  "",
			chartName:      "",
			want:           map[string]any{},
		},
		{
			name:           "Single variable",
			chartVariables: []types.ZarfChartVariable{{Name: "testVar", Path: "testVar"}},
			setVariableMap: variables.SetVariableMap{"testVar": {Value: "testValue"}},
			deployOpts:     types.ZarfDeployOptions{},
			componentName:  "testComponent",
			chartName:      "testChart",
			want:           map[string]any{"testVar": "testValue"},
		},
		{
			name:           "Non-matching setVariable",
			chartVariables: []types.ZarfChartVariable{{Name: "expectedVar", Path: "path.to.expectedVar"}},
			setVariableMap: variables.SetVariableMap{"unexpectedVar": {Value: "unexpectedValue"}},
			deployOpts:     types.ZarfDeployOptions{},
			componentName:  "testComponent",
			chartName:      "testChart",
			want:           map[string]any{},
		},
		{
			name: "Nested 3 level setVariableMap",
			chartVariables: []types.ZarfChartVariable{
				{Name: "level1.level2.level3Var", Path: "level1.level2.level3Var"},
			},
			setVariableMap: variables.SetVariableMap{
				"level1.level2.level3Var": {Value: "nestedValue"},
			},
			deployOpts:    types.ZarfDeployOptions{},
			componentName: "nestedComponent",
			chartName:     "nestedChart",
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
			chartVariables: []types.ZarfChartVariable{
				{Name: "NESTED_VAR_LEVEL2", Path: "nestedVar.level2"},
				{Name: "simpleVar", Path: "simpleVar"},
			},
			setVariableMap: variables.SetVariableMap{
				"NESTED_VAR_LEVEL2": {Value: "distinctNestedValue"},
				"simpleVar":         {Value: "distinctSimpleValue"},
			},
			deployOpts:    types.ZarfDeployOptions{},
			componentName: "mixedComponent",
			chartName:     "mixedChart",
			want: map[string]any{
				"nestedVar": map[string]any{
					"level2": "distinctNestedValue",
				},
				"simpleVar": "distinctSimpleValue",
			},
		},
		{
			name: "Values override test",
			chartVariables: []types.ZarfChartVariable{
				{Name: "overrideVar", Path: "path"},
			},
			setVariableMap: variables.SetVariableMap{
				"path": {Value: "overrideValue"},
			},
			deployOpts: types.ZarfDeployOptions{
				ValuesOverridesMap: map[string]map[string]map[string]any{
					"testComponent": {
						"testChart": {
							"path": "deployOverrideValue",
						},
					},
				},
			},
			componentName: "testComponent",
			chartName:     "testChart",
			want: map[string]any{
				"path": "deployOverrideValue",
			},
		},
		{
			name: "Missing variable in setVariableMap but present in ValuesOverridesMap",
			chartVariables: []types.ZarfChartVariable{
				{Name: "missingVar", Path: "missingVarPath"},
			},
			setVariableMap: variables.SetVariableMap{},
			deployOpts: types.ZarfDeployOptions{
				ValuesOverridesMap: map[string]map[string]map[string]any{
					"testComponent": {
						"testChart": {
							"missingVarPath": "overrideValue",
						},
					},
				},
			},
			componentName: "testComponent",
			chartName:     "testChart",
			want: map[string]any{
				"missingVarPath": "overrideValue",
			},
		},
		{
			name:           "Non-existent component or chart",
			chartVariables: []types.ZarfChartVariable{{Name: "someVar", Path: "someVar"}},
			setVariableMap: variables.SetVariableMap{"someVar": {Value: "value"}},
			deployOpts: types.ZarfDeployOptions{
				ValuesOverridesMap: map[string]map[string]map[string]any{
					"nonExistentComponent": {
						"nonExistentChart": {
							"someVar": "overrideValue",
						},
					},
				},
			},
			componentName: "actualComponent",
			chartName:     "actualChart",
			want:          map[string]any{"someVar": "value"},
		},
		{
			name:           "Variable in setVariableMap but not in chartVariables",
			chartVariables: []types.ZarfChartVariable{},
			setVariableMap: variables.SetVariableMap{
				"orphanVar": {Value: "orphanValue"},
			},
			deployOpts:    types.ZarfDeployOptions{},
			componentName: "orphanComponent",
			chartName:     "orphanChart",
			want:          map[string]any{},
		},
		{
			name: "Empty ValuesOverridesMap with non-empty setVariableMap and chartVariables",
			chartVariables: []types.ZarfChartVariable{
				{Name: "var1", Path: "path.to.var1"},
				{Name: "var2", Path: "path.to.var2"},
				{Name: "var3", Path: "path.to3.var3"},
			},
			setVariableMap: variables.SetVariableMap{
				"var1": {Value: "value1"},
				"var2": {Value: "value2"},
				"var3": {Value: "value3"},
			},
			deployOpts: types.ZarfDeployOptions{
				ValuesOverridesMap: map[string]map[string]map[string]any{},
			},
			componentName: "componentWithVars",
			chartName:     "chartWithVars",
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
			name:           "Empty chartVariables and non-empty setVariableMap",
			chartVariables: []types.ZarfChartVariable{},
			setVariableMap: variables.SetVariableMap{
				"var1": {Value: "value1"},
				"var2": {Value: "value2"},
			},
			deployOpts:    types.ZarfDeployOptions{},
			componentName: "componentWithVars",
			chartName:     "chartWithVars",
			want:          map[string]any{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateValuesOverrides(tt.chartVariables, tt.setVariableMap, tt.deployOpts, tt.componentName, tt.chartName)
			if err != nil {
				t.Errorf("%s: generateValuesOverrides() error = %v", tt.name, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("%s: generateValuesOverrides() got = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
