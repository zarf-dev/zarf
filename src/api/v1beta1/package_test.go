// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetOriginalAPIVersion(t *testing.T) {
	t.Parallel()

	var unset BuildData
	require.Equal(t, APIVersion, unset.GetOriginalAPIVersion())

	var recorded BuildData
	recorded.SetOriginalAPIVersion("zarf.dev/v1alpha1")
	require.Equal(t, "zarf.dev/v1alpha1", recorded.GetOriginalAPIVersion())
}

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

func TestGetComponent(t *testing.T) {
	t.Parallel()

	pkg := Package{
		Components: []Component{
			{
				Name:          "first",
				ComponentSpec: ComponentSpec{Images: []Image{{Name: "docker.io/library/nginx:latest"}}},
			},
			{Name: "second"},
		},
	}

	t.Run("returns matching component", func(t *testing.T) {
		t.Parallel()
		got, err := pkg.GetComponent("first")
		require.NoError(t, err)
		require.Equal(t, "first", got.Name)
		require.Equal(t, []Image{{Name: "docker.io/library/nginx:latest"}}, got.Images)
	})

	t.Run("errors when component is absent", func(t *testing.T) {
		t.Parallel()
		_, err := pkg.GetComponent("missing")
		require.Error(t, err)
	})
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
