// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPackageHasImages(t *testing.T) {
	t.Parallel()

	pkg := Package{
		Components: []Component{
			{
				Name: "without images",
			},
		},
	}
	require.False(t, pkg.HasImages())
	pkg = Package{
		Components: []Component{
			{
				Name:          "with images",
				ComponentSpec: ComponentSpec{Images: []Image{{Name: "docker.io/library/alpine:latest"}}},
			},
		},
	}
	require.True(t, pkg.HasImages())
}

func TestPackageIsSBOMable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		images        []Image
		files         []File
		imageArchives []ImageArchive
		expected      bool
	}{
		{
			name:     "empty component",
			expected: false,
		},
		{
			name:     "only images",
			images:   []Image{{Name: "alpine"}},
			expected: true,
		},
		{
			name:     "only files",
			files:    []File{{}},
			expected: true,
		},
		{
			name:          "only image archives",
			imageArchives: []ImageArchive{{Path: "archive.tar", Images: []string{"img"}}},
			expected:      true,
		},
		{
			name:          "all three set",
			images:        []Image{{Name: "alpine"}},
			files:         []File{{}},
			imageArchives: []ImageArchive{{Path: "archive.tar", Images: []string{"img"}}},
			expected:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pkg := Package{
				Components: []Component{
					{
						Name: "test-component",
						ComponentSpec: ComponentSpec{
							Images:        tt.images,
							Files:         tt.files,
							ImageArchives: tt.imageArchives,
						},
					},
				},
			}
			require.Equal(t, tt.expected, pkg.IsSBOMAble())
		})
	}
}
