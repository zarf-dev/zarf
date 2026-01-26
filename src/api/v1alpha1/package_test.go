// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package v1alpha1 holds the definition of the v1alpha1 Zarf Package
package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestZarfPackageIsInitPackage(t *testing.T) {
	t.Parallel()

	pkg := ZarfPackage{
		Kind: ZarfInitConfig,
	}
	require.True(t, pkg.IsInitConfig())
	pkg = ZarfPackage{
		Kind: ZarfPackageConfig,
	}
	require.False(t, pkg.IsInitConfig())
}

func TestZarfPackageHasImages(t *testing.T) {
	t.Parallel()

	pkg := ZarfPackage{
		Components: []ZarfComponent{
			{
				Name: "without images",
			},
		},
	}
	require.False(t, pkg.HasImages())
	pkg = ZarfPackage{
		Components: []ZarfComponent{
			{
				Name:   "with images",
				Images: []string{"docker.io/library/alpine:latest"},
			},
		},
	}
	require.True(t, pkg.HasImages())
}

func TestUniqueNamespaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pkg      ZarfPackage
		expected []string
	}{
		{
			name:     "empty package",
			pkg:      ZarfPackage{},
			expected: []string{},
		},
		{
			name: "single chart namespace",
			pkg: ZarfPackage{
				Components: []ZarfComponent{
					{
						Charts: []ZarfChart{
							{Name: "test", Namespace: "test-ns"},
						},
					},
				},
			},
			expected: []string{"test-ns"},
		},
		{
			name: "single manifest namespace",
			pkg: ZarfPackage{
				Components: []ZarfComponent{
					{
						Manifests: []ZarfManifest{
							{Name: "test", Namespace: "manifest-ns"},
						},
					},
				},
			},
			expected: []string{"manifest-ns"},
		},
		{
			name: "multiple unique namespaces",
			pkg: ZarfPackage{
				Components: []ZarfComponent{
					{
						Charts: []ZarfChart{
							{Name: "chart1", Namespace: "ns-a"},
							{Name: "chart2", Namespace: "ns-b"},
						},
						Manifests: []ZarfManifest{
							{Name: "manifest1", Namespace: "ns-c"},
						},
					},
				},
			},
			expected: []string{"ns-a", "ns-b", "ns-c"},
		},
		{
			name: "duplicate namespaces are deduplicated",
			pkg: ZarfPackage{
				Components: []ZarfComponent{
					{
						Charts: []ZarfChart{
							{Name: "chart1", Namespace: "same-ns"},
							{Name: "chart2", Namespace: "same-ns"},
						},
						Manifests: []ZarfManifest{
							{Name: "manifest1", Namespace: "same-ns"},
						},
					},
				},
			},
			expected: []string{"same-ns"},
		},
		{
			name: "wait action namespaces are not included",
			pkg: ZarfPackage{
				Components: []ZarfComponent{
					{
						Charts: []ZarfChart{
							{Name: "chart1", Namespace: "chart-ns"},
						},
						Actions: ZarfComponentActions{
							OnDeploy: ZarfComponentActionSet{
								After: []ZarfComponentAction{
									{
										Wait: &ZarfComponentActionWait{
											Cluster: &ZarfComponentActionWaitCluster{
												Kind:      "Pod",
												Name:      "test",
												Namespace: "wait-ns",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: []string{"chart-ns"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.pkg.UniqueNamespaces()
			require.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestUpdateAllComponentNamespacesByName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                    string
		pkg                     ZarfPackage
		original                string
		target                  string
		expectedChartNamespaces []string
		expectedWaitNamespaces  []string
	}{
		{
			name: "updates matching chart and manifest namespaces",
			pkg: ZarfPackage{
				Components: []ZarfComponent{
					{
						Charts: []ZarfChart{
							{Name: "chart1", Namespace: "original-ns"},
						},
						Manifests: []ZarfManifest{
							{Name: "manifest1", Namespace: "original-ns"},
						},
					},
				},
			},
			original:                "original-ns",
			target:                  "new-ns",
			expectedChartNamespaces: []string{"new-ns", "new-ns"},
			expectedWaitNamespaces:  []string{},
		},
		{
			name: "does not update non-matching namespaces",
			pkg: ZarfPackage{
				Components: []ZarfComponent{
					{
						Charts: []ZarfChart{
							{Name: "chart1", Namespace: "original-ns"},
							{Name: "chart2", Namespace: "other-ns"},
						},
					},
				},
			},
			original:                "original-ns",
			target:                  "new-ns",
			expectedChartNamespaces: []string{"new-ns", "other-ns"},
			expectedWaitNamespaces:  []string{},
		},
		{
			name: "updates matching wait action namespaces",
			pkg: ZarfPackage{
				Components: []ZarfComponent{
					{
						Charts: []ZarfChart{
							{Name: "chart1", Namespace: "original-ns"},
						},
						Actions: ZarfComponentActions{
							OnDeploy: ZarfComponentActionSet{
								After: []ZarfComponentAction{
									{
										Wait: &ZarfComponentActionWait{
											Cluster: &ZarfComponentActionWaitCluster{
												Kind:      "Pod",
												Name:      "test",
												Namespace: "original-ns",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			original:                "original-ns",
			target:                  "new-ns",
			expectedChartNamespaces: []string{"new-ns"},
			expectedWaitNamespaces:  []string{"new-ns"},
		},
		{
			name: "does not update non-matching wait action namespaces",
			pkg: ZarfPackage{
				Components: []ZarfComponent{
					{
						Charts: []ZarfChart{
							{Name: "chart1", Namespace: "original-ns"},
						},
						Actions: ZarfComponentActions{
							OnDeploy: ZarfComponentActionSet{
								After: []ZarfComponentAction{
									{
										Wait: &ZarfComponentActionWait{
											Cluster: &ZarfComponentActionWaitCluster{
												Kind:      "Pod",
												Name:      "test",
												Namespace: "different-ns",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			original:                "original-ns",
			target:                  "new-ns",
			expectedChartNamespaces: []string{"new-ns"},
			expectedWaitNamespaces:  []string{"different-ns"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.pkg.UpdateAllComponentNamespacesByName(tt.original, tt.target)

			var chartNamespaces []string
			var waitNamespaces []string
			for _, comp := range tt.pkg.Components {
				for _, chart := range comp.Charts {
					chartNamespaces = append(chartNamespaces, chart.Namespace)
				}
				for _, manifest := range comp.Manifests {
					chartNamespaces = append(chartNamespaces, manifest.Namespace)
				}
				for _, action := range comp.Actions.GetAll() {
					if action.Wait != nil && action.Wait.Cluster != nil && action.Wait.Cluster.Namespace != "" {
						waitNamespaces = append(waitNamespaces, action.Wait.Cluster.Namespace)
					}
				}
			}

			require.ElementsMatch(t, tt.expectedChartNamespaces, chartNamespaces)
			require.ElementsMatch(t, tt.expectedWaitNamespaces, waitNamespaces)
		})
	}
}

func TestZarfPackageIsSBOMable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		images         []string
		imageArchives  []ImageArchive
		files          []ZarfFile
		dataInjections []ZarfDataInjection
		expected       bool
	}{
		{
			name:     "empty component",
			expected: false,
		},
		{
			name:     "only images",
			images:   []string{""},
			expected: true,
		},
		{
			name:          "only image tars",
			imageArchives: []ImageArchive{{}},
			expected:      true,
		},
		{
			name:     "only files",
			files:    []ZarfFile{{}},
			expected: true,
		},
		{
			name:           "only data injections",
			dataInjections: []ZarfDataInjection{{}},
			expected:       true,
		},
		{
			name:           "all set",
			images:         []string{""},
			files:          []ZarfFile{{}},
			imageArchives:  []ImageArchive{{}},
			dataInjections: []ZarfDataInjection{{}},
			expected:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pkg := ZarfPackage{
				Components: []ZarfComponent{
					{
						Name:           "without images",
						Images:         tt.images,
						Files:          tt.files,
						ImageArchives:  tt.imageArchives,
						DataInjections: tt.dataInjections,
					},
				},
			}
			require.Equal(t, tt.expected, pkg.IsSBOMAble())
		})
	}
}
