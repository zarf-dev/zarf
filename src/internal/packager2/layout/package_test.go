// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
)

func TestPackageLayout(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	pathToPackage := filepath.Join("..", "testdata", "load-package", "compressed")

	pkgLayout, err := LoadFromTar(ctx, filepath.Join(pathToPackage, "zarf-package-test-amd64-0.0.1.tar.zst"), PackageLayoutOptions{})
	require.NoError(t, err)

	require.Equal(t, "test", pkgLayout.Pkg.Metadata.Name)
	require.Equal(t, "0.0.1", pkgLayout.Pkg.Metadata.Version)

	tmpDir := t.TempDir()
	manifestDir, err := pkgLayout.GetComponentDir(ctx, tmpDir, "test", ManifestsComponentDir)
	require.NoError(t, err)
	expected, err := os.ReadFile(filepath.Join(pathToPackage, "deployment.yaml"))
	require.NoError(t, err)
	b, err := os.ReadFile(filepath.Join(manifestDir, "deployment-0.yaml"))
	require.NoError(t, err)
	require.Equal(t, expected, b)

	_, err = pkgLayout.GetComponentDir(ctx, t.TempDir(), "does-not-exist", ManifestsComponentDir)
	require.ErrorContains(t, err, "component does-not-exist does not exist in package")

	_, err = pkgLayout.GetComponentDir(ctx, t.TempDir(), "test", FilesComponentDir)
	require.ErrorContains(t, err, "component test could not access a files directory")

	tmpDir = t.TempDir()
	err = pkgLayout.GetSBOM(ctx, tmpDir)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(tmpDir, "compare.html"))

	files, err := pkgLayout.Files()
	require.NoError(t, err)
	expectedNames := []string{
		"checksums.txt",
		"components/test.tar",
		"images/blobs/sha256/43180c492a5e6cedd8232e8f77a454f666f247586853eecb90258b26688ad1d3",
		"images/blobs/sha256/ff221270b9fb7387b0ad9ff8f69fbbd841af263842e62217392f18c3b5226f38",
		"images/blobs/sha256/0a9a5dfd008f05ebc27e4790db0709a29e527690c21bcbcd01481eaeb6bb49dc",
		"images/index.json",
		"images/oci-layout",
		"sboms.tar",
		"zarf.yaml",
	}
	require.Len(t, expectedNames, len(files))
	for _, expectedName := range expectedNames {
		path := filepath.Join(pkgLayout.dirPath, filepath.FromSlash(expectedName))
		name := files[path]
		require.Equal(t, expectedName, name)
	}
}

func TestPackageFileName(t *testing.T) {
	t.Parallel()
	config.CLIArch = "amd64"
	tests := []struct {
		name        string
		pkg         v1alpha1.ZarfPackage
		expected    string
		expectedErr string
	}{
		{
			name: "no architecture",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfInitConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Version: "v0.55.4",
				},
			},
			expectedErr: "package must include a build architecture",
		},
		{
			name: "init package",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfInitConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Version: "v0.55.4",
				},
				Build: v1alpha1.ZarfBuildData{
					Architecture: "amd64",
				},
			},
			expected: "zarf-init-amd64-v0.55.4.tar.zst",
		},
		{
			name: "regular package with version",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Name:    "my-package",
					Version: "v0.55.4",
				},
				Build: v1alpha1.ZarfBuildData{
					Architecture: "amd64",
				},
			},
			expected: "zarf-package-my-package-amd64-v0.55.4.tar.zst",
		},
		{
			name: "regular package no version",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Name: "my-package",
				},
				Build: v1alpha1.ZarfBuildData{
					Architecture: "amd64",
				},
			},
			expected: "zarf-package-my-package-amd64.tar.zst",
		},
		{
			name: "differential package",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Name:    "my-package",
					Version: "v0.55.4",
				},
				Build: v1alpha1.ZarfBuildData{
					Differential:               true,
					Architecture:               "amd64",
					DifferentialPackageVersion: "v0.55.3",
				},
			},
			expected: "zarf-package-my-package-amd64-v0.55.3-differential-v0.55.4.tar.zst",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			layout := PackageLayout{Pkg: tt.pkg}
			actual, err := layout.FileName()
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
			}
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestSplitFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		fileSize             int
		chunkSize            int
		expectedFileSize     int64
		expectedLastFileSize int64
		expectedFileCount    int
		expectedSha256Sum    string
	}{
		{
			name:                 "split evenly",
			fileSize:             2048,
			chunkSize:            16,
			expectedFileSize:     16,
			expectedLastFileSize: 16,
			expectedFileCount:    128,
			expectedSha256Sum:    "93ecad679eff0df493aaf5d7d615211b0f1d7a919016efb15c98f0b8efb1ba43",
		},
		{
			name:                 "split with remainder",
			fileSize:             2048,
			chunkSize:            10,
			expectedFileSize:     10,
			expectedLastFileSize: 8,
			expectedFileCount:    205,
			expectedSha256Sum:    "fe8460f4d53d3578aa37191acf55b3db7bbcb706056f4b6b02a0c70f24b0d95a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			name := "random"
			p := filepath.Join(dir, name)
			f, err := os.Create(p)
			require.NoError(t, err)
			b := make([]byte, tt.fileSize)
			for i := range tt.fileSize {
				b[i] = byte(tt.chunkSize)
			}
			require.NoError(t, err)
			_, err = f.Write(b)
			require.NoError(t, err)
			err = f.Close()
			require.NoError(t, err)

			err = splitFile(context.Background(), p, tt.chunkSize)
			require.NoError(t, err)

			_, err = os.Stat(p)
			require.ErrorIs(t, err, os.ErrNotExist)
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			require.Len(t, entries, tt.expectedFileCount+1)
			for i, entry := range entries[1:] {
				require.Equal(t, fmt.Sprintf("%s.part%03d", name, i+1), entry.Name())

				fi, err := entry.Info()
				require.NoError(t, err)
				if i == len(entries)-2 {
					require.Equal(t, tt.expectedLastFileSize, fi.Size())
				} else {
					require.Equal(t, tt.expectedFileSize, fi.Size())
				}
			}

			b, err = os.ReadFile(filepath.Join(dir, fmt.Sprintf("%s.part000", name)))
			require.NoError(t, err)
			var data types.ZarfSplitPackageData
			err = json.Unmarshal(b, &data)
			require.NoError(t, err)
			require.Equal(t, tt.expectedFileCount, data.Count)
			require.Equal(t, int64(tt.fileSize), data.Bytes)
			require.Equal(t, tt.expectedSha256Sum, data.Sha256Sum)
		})
	}
}

func TestSplitDeleteExistingFiles(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	inputFilename := filepath.Join(tempDir, "testfile.txt")
	data := make([]byte, 50)
	err := os.WriteFile(inputFilename, data, 0644)
	require.NoError(t, err)
	// Create many fake split files
	for i := range 15 {
		f, err := os.Create(fmt.Sprintf("%s.part%03d", inputFilename, i))
		require.NoError(t, err)
		require.NoError(t, f.Close())
	}

	chunkSize := 20
	err = splitFile(context.Background(), inputFilename, chunkSize)
	require.NoError(t, err)

	entries, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	// Verify only header file + 3 data files remain, and not the 15 test split files
	require.Len(t, entries, 4)
}
