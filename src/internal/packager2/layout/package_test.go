// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/test/testutil"
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
	require.Equal(t, len(expectedNames), len(files))
	for _, expectedName := range expectedNames {
		path := filepath.Join(pkgLayout.dirPath, filepath.FromSlash(expectedName))
		name := files[path]
		require.Equal(t, expectedName, name)
	}
}
