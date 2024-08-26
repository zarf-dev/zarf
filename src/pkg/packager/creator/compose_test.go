// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

func TestComposeComponents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		pkg         v1alpha1.ZarfPackage
		flavor      string
		expectedPkg v1alpha1.ZarfPackage
		expectedErr string
	}{
		{
			name: "filter by architecture match",
			pkg: v1alpha1.ZarfPackage{
				Metadata: v1alpha1.ZarfMetadata{Architecture: "amd64"},
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "component1",
						Only: v1alpha1.ZarfComponentOnlyTarget{
							Cluster: v1alpha1.ZarfComponentOnlyCluster{
								Architecture: "amd64",
							},
						},
					},
					{
						Name: "component2",
						Only: v1alpha1.ZarfComponentOnlyTarget{
							Cluster: v1alpha1.ZarfComponentOnlyCluster{
								Architecture: "amd64",
							},
						},
					},
				},
			},
			expectedPkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{Name: "component1"},
					{Name: "component2"},
				},
			},
			expectedErr: "",
		},
		{
			name: "filter by architecture mismatch",
			pkg: v1alpha1.ZarfPackage{
				Metadata: v1alpha1.ZarfMetadata{Architecture: "amd64"},
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "component1",
						Only: v1alpha1.ZarfComponentOnlyTarget{
							Cluster: v1alpha1.ZarfComponentOnlyCluster{
								Architecture: "amd64",
							},
						},
					},
					{
						Name: "component2",
						Only: v1alpha1.ZarfComponentOnlyTarget{
							Cluster: v1alpha1.ZarfComponentOnlyCluster{
								Architecture: "arm64",
							},
						},
					},
				},
			},
			expectedPkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{Name: "component1"},
				},
			},
			expectedErr: "",
		},
		{
			name: "filter by flavor match",
			pkg: v1alpha1.ZarfPackage{
				Metadata: v1alpha1.ZarfMetadata{Architecture: "amd64"},
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "component1",
						Only: v1alpha1.ZarfComponentOnlyTarget{
							Flavor: "default",
						},
					},
					{
						Name: "component2",
						Only: v1alpha1.ZarfComponentOnlyTarget{
							Flavor: "default",
						},
					},
				},
			},
			flavor: "default",
			expectedPkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{Name: "component1"},
					{Name: "component2"},
				},
			},
			expectedErr: "",
		},
		{
			name: "filter by flavor mismatch",
			pkg: v1alpha1.ZarfPackage{
				Metadata: v1alpha1.ZarfMetadata{Architecture: "amd64"},
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "component1",
						Only: v1alpha1.ZarfComponentOnlyTarget{
							Flavor: "default",
						},
					},
					{
						Name: "component2",
						Only: v1alpha1.ZarfComponentOnlyTarget{
							Flavor: "special",
						},
					},
				},
			},
			flavor: "default",
			expectedPkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{Name: "component1"},
				},
			},
			expectedErr: "",
		},
		{
			name: "no architecture set error",
			pkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "component1",
						Only: v1alpha1.ZarfComponentOnlyTarget{
							Flavor: "default",
						},
					},
				},
			},
			flavor:      "default",
			expectedPkg: v1alpha1.ZarfPackage{},
			expectedErr: "cannot build import chain: architecture must be provided",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pkg, _, err := ComposeComponents(context.Background(), tt.pkg, tt.flavor)

			if tt.expectedErr == "" {
				require.NoError(t, err)
				require.Equal(t, tt.expectedPkg.Components, pkg.Components)
				return
			}

			require.EqualError(t, err, tt.expectedErr)
			require.Empty(t, tt.expectedPkg)
		})
	}
}
