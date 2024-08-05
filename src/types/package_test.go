// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package types

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

func TestZarfPackageIsSBOMable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		images         []string
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
			name:           "all three set",
			images:         []string{""},
			files:          []ZarfFile{{}},
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
						DataInjections: tt.dataInjections,
					},
				},
			}
			require.Equal(t, tt.expected, pkg.IsSBOMAble())
		})
	}
}
