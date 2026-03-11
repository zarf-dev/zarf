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

func TestOverridePackageNamespace(t *testing.T) {
	t.Parallel()

	allow := false

	tt := []struct {
		name                   string
		pkg                    v1alpha1.ZarfPackage
		namespace              string
		expectedWaitNamespaces []string
		expectedErr            string
	}{
		{
			name: "override namespace",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{
					{
						Charts: []v1alpha1.ZarfChart{
							{
								Name:      "test",
								Namespace: "test",
							},
						},
					},
				},
			},
			namespace: "test-override",
		},
		{
			name: "override namespace with wait action",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{
					{
						Charts: []v1alpha1.ZarfChart{
							{
								Name:      "test",
								Namespace: "test",
							},
						},
						Actions: v1alpha1.ZarfComponentActions{
							OnDeploy: v1alpha1.ZarfComponentActionSet{
								After: []v1alpha1.ZarfComponentAction{
									{
										Wait: &v1alpha1.ZarfComponentActionWait{
											Cluster: &v1alpha1.ZarfComponentActionWaitCluster{
												Kind:      "Pod",
												Name:      "test-pod",
												Namespace: "test",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			namespace:              "test-override",
			expectedWaitNamespaces: []string{"test-override"},
		},
		{
			name: "multiple namespaces",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{
					{
						Charts: []v1alpha1.ZarfChart{
							{
								Name:      "test",
								Namespace: "test",
							},
							{
								Name:      "test-2",
								Namespace: "test-2",
							},
						},
					},
				},
			},
			namespace:   "test-override",
			expectedErr: "package contains 2 unique namespaces, cannot override namespace",
		},
		{
			name: "wait action with different namespace is not updated when not matching override namespace",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{
					{
						Charts: []v1alpha1.ZarfChart{
							{
								Name:      "test",
								Namespace: "test",
							},
						},
						Actions: v1alpha1.ZarfComponentActions{
							OnDeploy: v1alpha1.ZarfComponentActionSet{
								After: []v1alpha1.ZarfComponentAction{
									{
										Wait: &v1alpha1.ZarfComponentActionWait{
											Cluster: &v1alpha1.ZarfComponentActionWaitCluster{
												Kind:      "Pod",
												Name:      "test-pod",
												Namespace: "different-namespace",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			namespace: "test-override",
			// wait action namespace "different-namespace" should NOT be updated since it doesn't match "test"
			expectedWaitNamespaces: []string{"different-namespace"},
		},
		{
			name: "override manifest namespace",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{
					{
						Manifests: []v1alpha1.ZarfManifest{
							{Name: "test", Namespace: "test"},
						},
					},
				},
			},
			namespace: "test-override",
		},
		{
			name: "override namespace across multiple components",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{
					{
						Charts: []v1alpha1.ZarfChart{
							{Name: "chart1", Namespace: "test"},
						},
					},
					{
						Charts: []v1alpha1.ZarfChart{
							{Name: "chart2", Namespace: "test"},
						},
					},
				},
			},
			namespace: "test-override",
		},
		{
			name: "override empty chart namespace",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{
					{
						Charts: []v1alpha1.ZarfChart{
							{Name: "test", Namespace: ""},
						},
					},
				},
			},
			namespace: "test-override",
		},
		{
			name: "mixed empty and non-empty namespaces blocks override",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{
					{
						Charts: []v1alpha1.ZarfChart{
							{Name: "chart1", Namespace: ""},
							{Name: "chart2", Namespace: "real-ns"},
						},
					},
				},
			},
			namespace:   "test-override",
			expectedErr: "package contains 2 unique namespaces, cannot override namespace",
		},
		{
			name: "init package namespace override",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfInitConfig,
				Components: []v1alpha1.ZarfComponent{
					{
						Charts: []v1alpha1.ZarfChart{
							{
								Name:      "test",
								Namespace: "test",
							},
						},
					},
				},
			},
			namespace:   "test-override",
			expectedErr: "package kind is not a ZarfPackageConfig, cannot override namespace",
		},
		{
			name: "namespace override not allowed",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Metadata: v1alpha1.ZarfMetadata{
					AllowNamespaceOverride: &allow,
				},
				Components: []v1alpha1.ZarfComponent{
					{
						Charts: []v1alpha1.ZarfChart{
							{
								Name:      "test",
								Namespace: "test",
							},
						},
					},
				},
			},
			namespace:   "test-override",
			expectedErr: "cannot override package namespace, metadata.allowNamespaceOverride is false",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := OverridePackageNamespace(&tc.pkg, tc.namespace)
			if tc.expectedErr == "" {
				require.NoError(t, err)
				validateNamespaceUpdates(t, tc.pkg, tc.namespace, tc.expectedWaitNamespaces)
			} else {
				require.ErrorContains(t, err, tc.expectedErr)
			}
		})
	}
}

func validateNamespaceUpdates(t *testing.T, pkg v1alpha1.ZarfPackage, targetNamespace string, expectedWaitNamespaces []string) {
	t.Helper()
	actualWaitNamespaces := make([]string, 0)
	for _, component := range pkg.Components {
		for _, chart := range component.Charts {
			require.Equal(t, targetNamespace, chart.Namespace)
		}
		for _, manifest := range component.Manifests {
			require.Equal(t, targetNamespace, manifest.Namespace)
		}
		actualWaitNamespaces = append(actualWaitNamespaces, collectWaitNamespaces(component.Actions)...)
	}
	require.ElementsMatch(t, expectedWaitNamespaces, actualWaitNamespaces)
}

func collectWaitNamespaces(actions v1alpha1.ZarfComponentActions) []string {
	var ns []string
	allSets := [][]v1alpha1.ZarfComponentAction{
		actions.OnCreate.Before, actions.OnCreate.After, actions.OnCreate.OnSuccess, actions.OnCreate.OnFailure,
		actions.OnDeploy.Before, actions.OnDeploy.After, actions.OnDeploy.OnSuccess, actions.OnDeploy.OnFailure,
		actions.OnRemove.Before, actions.OnRemove.After, actions.OnRemove.OnSuccess, actions.OnRemove.OnFailure,
	}
	for _, set := range allSets {
		for _, action := range set {
			if action.Wait != nil && action.Wait.Cluster != nil {
				ns = append(ns, action.Wait.Cluster.Namespace)
			}
		}
	}
	return ns
}

func TestOverrideComponentNamespacesActions(t *testing.T) {
	t.Parallel()

	makeWaitAction := func(ns string) v1alpha1.ZarfComponentAction {
		return v1alpha1.ZarfComponentAction{
			Wait: &v1alpha1.ZarfComponentActionWait{
				Cluster: &v1alpha1.ZarfComponentActionWaitCluster{
					Kind:      "Pod",
					Name:      "test",
					Namespace: ns,
				},
			},
		}
	}

	tests := []struct {
		name     string
		pkg      v1alpha1.ZarfPackage
		original string
		target   string
		expected []string
	}{
		{
			name: "all lifecycle sets and timing slots updated",
			pkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Actions: v1alpha1.ZarfComponentActions{
							OnCreate: v1alpha1.ZarfComponentActionSet{
								Before:    []v1alpha1.ZarfComponentAction{makeWaitAction("original")},
								After:     []v1alpha1.ZarfComponentAction{makeWaitAction("original")},
								OnSuccess: []v1alpha1.ZarfComponentAction{makeWaitAction("original")},
								OnFailure: []v1alpha1.ZarfComponentAction{makeWaitAction("original")},
							},
							OnDeploy: v1alpha1.ZarfComponentActionSet{
								Before:    []v1alpha1.ZarfComponentAction{makeWaitAction("original")},
								After:     []v1alpha1.ZarfComponentAction{makeWaitAction("original")},
								OnSuccess: []v1alpha1.ZarfComponentAction{makeWaitAction("original")},
								OnFailure: []v1alpha1.ZarfComponentAction{makeWaitAction("original")},
							},
							OnRemove: v1alpha1.ZarfComponentActionSet{
								Before:    []v1alpha1.ZarfComponentAction{makeWaitAction("original")},
								After:     []v1alpha1.ZarfComponentAction{makeWaitAction("original")},
								OnSuccess: []v1alpha1.ZarfComponentAction{makeWaitAction("original")},
								OnFailure: []v1alpha1.ZarfComponentAction{makeWaitAction("original")},
							},
						},
					},
				},
			},
			original: "original",
			target:   "new",
			expected: []string{"new", "new", "new", "new", "new", "new", "new", "new", "new", "new", "new", "new"},
		},
		{
			name: "non-matching wait actions not updated",
			pkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Actions: v1alpha1.ZarfComponentActions{
							OnDeploy: v1alpha1.ZarfComponentActionSet{
								After: []v1alpha1.ZarfComponentAction{makeWaitAction("other")},
							},
						},
					},
				},
			},
			original: "original",
			target:   "new",
			expected: []string{"other"},
		},
		{
			name: "nil Wait does not panic",
			pkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Actions: v1alpha1.ZarfComponentActions{
							OnDeploy: v1alpha1.ZarfComponentActionSet{
								After: []v1alpha1.ZarfComponentAction{{Wait: nil}},
							},
						},
					},
				},
			},
			original: "original",
			target:   "new",
			expected: []string{},
		},
		{
			name: "nil Wait.Cluster does not panic",
			pkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Actions: v1alpha1.ZarfComponentActions{
							OnDeploy: v1alpha1.ZarfComponentActionSet{
								After: []v1alpha1.ZarfComponentAction{
									{Wait: &v1alpha1.ZarfComponentActionWait{Cluster: nil}},
								},
							},
						},
					},
				},
			},
			original: "original",
			target:   "new",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			overrideComponentNamespaces(&tt.pkg, tt.original, tt.target)
			var actual []string
			for _, comp := range tt.pkg.Components {
				actual = append(actual, collectWaitNamespaces(comp.Actions)...)
			}
			require.ElementsMatch(t, tt.expected, actual)
		})
	}
}

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
