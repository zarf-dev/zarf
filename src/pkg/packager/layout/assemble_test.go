// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

func TestGetChecksum(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	files := map[string]string{
		"empty.txt":                "",
		"foo":                      "bar",
		"zarf.yaml":                "Zarf Yaml Data",
		"checksums.txt":            "Old Checksum Data",
		"nested/directory/file.md": "nested",
	}
	for k, v := range files {
		err := os.MkdirAll(filepath.Join(tmpDir, filepath.Dir(k)), 0o700)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tmpDir, k), []byte(v), 0o600)
		require.NoError(t, err)
	}

	checksumContent, checksumHash, err := getChecksum(tmpDir)
	require.NoError(t, err)

	expectedContent := `233562de1a0288b139c4fa40b7d189f806e906eeb048517aeb67f34ac0e2faf1 nested/directory/file.md
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 empty.txt
fcde2b2edba56bf408601fb721fe9b5c338d10ee429ea04fae5511b68fbf8fb9 foo
`
	require.Equal(t, expectedContent, checksumContent)
	require.Equal(t, "7c554cf67e1c2b50a1b728299c368cd56d53588300c37479623f29a52812ca3f", checksumHash)
}

func TestCreateReproducibleTarballFromDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello world"), 0o600)
	require.NoError(t, err)
	tarPath := filepath.Join(t.TempDir(), "data.tar")

	err = createReproducibleTarballFromDir(tmpDir, "", tarPath, true)
	require.NoError(t, err)

	shaSum, err := helpers.GetSHA256OfFile(tarPath)
	require.NoError(t, err)
	require.Equal(t, "c09d17f612f241cdf549e5fb97c9e063a8ad18ae7a9f3af066332ed6b38556ad", shaSum)
}

func TestCheckImageDuplicate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		components     []v1alpha1.ZarfComponent
		currentArchive v1alpha1.ImageArchive
		imageRef       string
		expectError    bool
		errorContains  string
	}{
		{
			name: "no duplicates",
			components: []v1alpha1.ZarfComponent{
				{
					Name: "component1",
					ImageArchives: []v1alpha1.ImageArchive{
						{
							Path:   "/path/to/archive1.tar",
							Images: []string{"nginx:1.21"},
						},
					},
					Images: []string{"redis:6.2"},
				},
			},
			currentArchive: v1alpha1.ImageArchive{
				Path: "/path/to/archive1.tar",
			},
			imageRef:    "postgres:13",
			expectError: false,
		},
		{
			name: "duplicate in different image archive",
			components: []v1alpha1.ZarfComponent{
				{
					Name: "component1",
					ImageArchives: []v1alpha1.ImageArchive{
						{
							Path:   "/path/to/archive1.tar",
							Images: []string{"nginx:1.21"},
						},
						{
							Path:   "/path/to/archive2.tar",
							Images: []string{"postgres:13"},
						},
					},
				},
			},
			currentArchive: v1alpha1.ImageArchive{
				Path: "/path/to/archive1.tar",
			},
			imageRef:      "postgres:13",
			expectError:   true,
			errorContains: "is also pulled by archive /path/to/archive2.tar",
		},
		{
			name: "duplicate in component images",
			components: []v1alpha1.ZarfComponent{
				{
					Name:   "component1",
					Images: []string{"nginx:1.21", "postgres:13"},
				},
			},
			currentArchive: v1alpha1.ImageArchive{
				Path: "/path/to/archive1.tar",
			},
			imageRef:      "postgres:13",
			expectError:   true,
			errorContains: "is also pulled by component",
		},
		{
			name: "same archive path should be skipped",
			components: []v1alpha1.ZarfComponent{
				{
					Name: "component1",
					ImageArchives: []v1alpha1.ImageArchive{
						{
							Path:   "/path/to/archive1.tar",
							Images: []string{"nginx:1.21"},
						},
					},
				},
			},
			currentArchive: v1alpha1.ImageArchive{
				Path: "/path/to/archive1.tar",
			},
			imageRef:    "nginx:1.21",
			expectError: false,
		},
		{
			name: "duplicate across multiple components",
			components: []v1alpha1.ZarfComponent{
				{
					Name: "component1",
					ImageArchives: []v1alpha1.ImageArchive{
						{
							Path: "/path/to/archive1.tar",
						},
					},
				},
				{
					Name: "component2",
					ImageArchives: []v1alpha1.ImageArchive{
						{
							Path:   "/path/to/archive2.tar",
							Images: []string{"nginx:1.21"},
						},
					},
				},
			},
			currentArchive: v1alpha1.ImageArchive{
				Path: "/path/to/archive1.tar",
			},
			imageRef:      "nginx:1.21",
			expectError:   true,
			errorContains: "is also pulled by archive /path/to/archive2.tar",
		},
		{
			name: "empty components",
			components: []v1alpha1.ZarfComponent{
				{
					Name: "component1",
				},
			},
			currentArchive: v1alpha1.ImageArchive{
				Path: "/path/to/archive1.tar",
			},
			imageRef:    "nginx:1.21",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := checkForDuplicateImage(tt.components, tt.currentArchive, tt.imageRef)

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
