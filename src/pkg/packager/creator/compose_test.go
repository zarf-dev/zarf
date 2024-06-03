// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"testing"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
)

func TestComposeComponents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		pkg         types.ZarfPackage
		flavor      string
		expectedPkg types.ZarfPackage
		expectedErr string
	}{
		{
			name: "filter by architecture match",
			pkg: types.ZarfPackage{
				Metadata: types.ZarfMetadata{Architecture: "amd64"},
				Components: []types.ZarfComponent{
					{
						Name: "component1",
						Only: types.ZarfComponentOnlyTarget{
							Cluster: types.ZarfComponentOnlyCluster{
								Architecture: "amd64",
							},
						},
					},
					{
						Name: "component2",
						Only: types.ZarfComponentOnlyTarget{
							Cluster: types.ZarfComponentOnlyCluster{
								Architecture: "amd64",
							},
						},
					},
				},
			},
			expectedPkg: types.ZarfPackage{
				Components: []types.ZarfComponent{
					{Name: "component1"},
					{Name: "component2"},
				},
			},
			expectedErr: "",
		},
		{
			name: "filter by architecture mismatch",
			pkg: types.ZarfPackage{
				Metadata: types.ZarfMetadata{Architecture: "amd64"},
				Components: []types.ZarfComponent{
					{
						Name: "component1",
						Only: types.ZarfComponentOnlyTarget{
							Cluster: types.ZarfComponentOnlyCluster{
								Architecture: "amd64",
							},
						},
					},
					{
						Name: "component2",
						Only: types.ZarfComponentOnlyTarget{
							Cluster: types.ZarfComponentOnlyCluster{
								Architecture: "arm64",
							},
						},
					},
				},
			},
			expectedPkg: types.ZarfPackage{
				Components: []types.ZarfComponent{
					{Name: "component1"},
				},
			},
			expectedErr: "",
		},
		{
			name: "filter by flavor match",
			pkg: types.ZarfPackage{
				Metadata: types.ZarfMetadata{Architecture: "amd64"},
				Components: []types.ZarfComponent{
					{
						Name: "component1",
						Only: types.ZarfComponentOnlyTarget{
							Flavor: "default",
						},
					},
					{
						Name: "component2",
						Only: types.ZarfComponentOnlyTarget{
							Flavor: "default",
						},
					},
				},
			},
			flavor: "default",
			expectedPkg: types.ZarfPackage{
				Components: []types.ZarfComponent{
					{Name: "component1"},
					{Name: "component2"},
				},
			},
			expectedErr: "",
		},
		{
			name: "filter by flavor mismatch",
			pkg: types.ZarfPackage{
				Metadata: types.ZarfMetadata{Architecture: "amd64"},
				Components: []types.ZarfComponent{
					{
						Name: "component1",
						Only: types.ZarfComponentOnlyTarget{
							Flavor: "default",
						},
					},
					{
						Name: "component2",
						Only: types.ZarfComponentOnlyTarget{
							Flavor: "special",
						},
					},
				},
			},
			flavor: "default",
			expectedPkg: types.ZarfPackage{
				Components: []types.ZarfComponent{
					{Name: "component1"},
				},
			},
			expectedErr: "",
		},
		{
			name: "no architecture set error",
			pkg: types.ZarfPackage{
				Components: []types.ZarfComponent{
					{
						Name: "component1",
						Only: types.ZarfComponentOnlyTarget{
							Flavor: "default",
						},
					},
				},
			},
			flavor:      "default",
			expectedPkg: types.ZarfPackage{},
			expectedErr: "cannot build import chain: architecture must be provided",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pkg, _, err := ComposeComponents(tt.pkg, tt.flavor)

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
