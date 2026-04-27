// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1beta1

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
				Images: []ZarfImage{{Name: "docker.io/library/alpine:latest"}},
			},
		},
	}
	require.True(t, pkg.HasImages())
}

func TestZarfPackageIsSBOMable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		images        []ZarfImage
		files         []ZarfFile
		imageArchives []ImageArchive
		expected      bool
	}{
		{
			name:     "empty component",
			expected: false,
		},
		{
			name:     "only images",
			images:   []ZarfImage{{Name: "alpine"}},
			expected: true,
		},
		{
			name:     "only files",
			files:    []ZarfFile{{}},
			expected: true,
		},
		{
			name:          "only image archives",
			imageArchives: []ImageArchive{{Path: "archive.tar", Images: []string{"img"}}},
			expected:      true,
		},
		{
			name:          "all three set",
			images:        []ZarfImage{{Name: "alpine"}},
			files:         []ZarfFile{{}},
			imageArchives: []ImageArchive{{Path: "archive.tar", Images: []string{"img"}}},
			expected:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pkg := ZarfPackage{
				Components: []ZarfComponent{
					{
						Name:          "test-component",
						Images:        tt.images,
						Files:         tt.files,
						ImageArchives: tt.imageArchives,
					},
				},
			}
			require.Equal(t, tt.expected, pkg.IsSBOMAble())
		})
	}
}
