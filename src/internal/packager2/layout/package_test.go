// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestPackageLayout(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	pkgLayout, err := LoadFromTar(ctx, "../testdata/zarf-package-test-amd64-0.0.1.tar.zst", PackageLayoutOptions{})
	require.NoError(t, err)

	require.Equal(t, "test", pkgLayout.Pkg.Metadata.Name)
	require.Equal(t, "0.0.1", pkgLayout.Pkg.Metadata.Version)

	tmpDir := t.TempDir()
	manifestDir, err := pkgLayout.GetComponentDir(tmpDir, "test", ManifestsComponentDir)
	require.NoError(t, err)
	expected, err := os.ReadFile("../testdata/deployment.yaml")
	require.NoError(t, err)
	b, err := os.ReadFile(filepath.Join(manifestDir, "deployment-0.yaml"))
	require.NoError(t, err)
	require.Equal(t, expected, b)

	_, err = pkgLayout.GetComponentDir(t.TempDir(), "does-not-exist", ManifestsComponentDir)
	require.ErrorContains(t, err, "component does-not-exist does not exist in package")

	_, err = pkgLayout.GetComponentDir(t.TempDir(), "test", FilesComponentDir)
	require.ErrorContains(t, err, "component test could not access a files directory")

	tmpDir = t.TempDir()
	sbomPath, err := pkgLayout.GetSBOM(tmpDir)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(sbomPath, "compare.html"))

	ref, err := transform.ParseImageRef("docker.io/library/alpine:3.20")
	require.NoError(t, err)
	img, err := pkgLayout.GetImage(ref)
	require.NoError(t, err)
	dgst, err := img.Digest()
	require.NoError(t, err)
	require.Equal(t, "sha256:33735bd63cf84d7e388d9f6d297d348c523c044410f553bd878c6d7829612735", dgst.String())

	files, err := pkgLayout.Files()
	require.NoError(t, err)
	expectedNames := []string{
		"checksums.txt",
		"components/test.tar",
		"images/blobs/sha256/33735bd63cf84d7e388d9f6d297d348c523c044410f553bd878c6d7829612735",
		"images/blobs/sha256/43c4264eed91be63b206e17d93e75256a6097070ce643c5e8f0379998b44f170",
		"images/blobs/sha256/91ef0af61f39ece4d6710e465df5ed6ca12112358344fd51ae6a3b886634148b",
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
