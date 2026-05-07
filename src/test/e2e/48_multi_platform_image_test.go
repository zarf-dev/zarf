// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory" // used for docker test registry
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry"

	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

// podinfoIndexDigest is the index digest of ghcr.io/stefanprodan/podinfo:6.4.0.
const podinfoIndexDigest = "sha256:57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8"

// TestMultiPlatformIndexImage exercises digest-pinned multi-platform images end-to-end:
// create + publish + pull + deploy of a single-arch package whose only image is pinned by an
// index digest. The package layout must preserve the full upstream index, the per-platform
// SBOMs must exist, and the image must end up at the original digest in the destination registry.
func TestMultiPlatformIndexImage(t *testing.T) {
	t.Log("E2E: index-sha image preserved as the full upstream index")

	pkgDefinitionPath := filepath.Join("src", "test", "packages", "48-multi-platform-image")
	createDir := t.TempDir()

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", pkgDefinitionPath, "-o", createDir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	createdPkgPath := filepath.Join(createDir, "zarf-package-multi-platform-image-amd64-0.0.1.tar.zst")
	require.FileExists(t, createdPkgPath)

	registryURL := testutil.SetupInMemoryRegistryDynamic(testutil.TestContext(t), t)
	ref := registry.Reference{
		Registry:   registryURL,
		Repository: "multi-platform-image",
		Reference:  "0.0.1",
	}

	stdOut, stdErr, err = e2e.Zarf(t, "package", "publish", createdPkgPath, "oci://"+registryURL, "--plain-http")
	require.NoError(t, err, stdOut, stdErr)

	pullDir := t.TempDir()
	stdOut, stdErr, err = e2e.Zarf(t, "package", "pull", "oci://"+ref.String(), "--plain-http", "-o", pullDir)
	require.NoError(t, err, stdOut, stdErr)

	pulledPkgPath := filepath.Join(pullDir, "zarf-package-multi-platform-image-amd64-0.0.1.tar.zst")
	pkgLayout, err := layout.LoadFromTar(t.Context(), pulledPkgPath, layout.PackageLayoutOptions{})
	require.NoError(t, err)

	idxBytes, err := os.ReadFile(filepath.Join(pkgLayout.GetImageDirPath(), "index.json"))
	require.NoError(t, err)
	var idx ocispec.Index
	require.NoError(t, json.Unmarshal(idxBytes, &idx))

	digestedRoot := verifyPreservedIndex(t, pkgLayout, idx, podinfoIndexDigest)
	require.Equal(t, podinfoIndexDigest, digestedRoot, "index-pinned image must keep its original index digest in the package layout")

	sbomDir := t.TempDir()
	require.NoError(t, pkgLayout.GetSBOM(t.Context(), sbomDir))
	sbomEntries, err := os.ReadDir(sbomDir)
	require.NoError(t, err)
	count := 0
	for _, entry := range sbomEntries {
		name := entry.Name()
		if strings.HasSuffix(name, ".json") && strings.Contains(name, "podinfo_6.4.0") {
			count++
		}
	}
	require.GreaterOrEqual(t, count, 2, "expected per-platform SBOMs for the digested multi-platform image")

	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", pulledPkgPath, "--confirm", "--skip-version-check")
	require.NoError(t, err, stdOut, stdErr)
	t.Cleanup(func() {
		_, _, err = e2e.Zarf(t, "package", "remove", "multi-platform-image", "--confirm", "--skip-version-check")
		require.NoError(t, err)
	})
}

// verifyPreservedIndex finds the top-level index.json entry whose ref-name annotation matches
// imageSubstring, asserts it points at an OCI image index with multiple platform manifests
// stored on disk, and returns the underlying index digest.
func verifyPreservedIndex(t *testing.T, pkgLayout *layout.PackageLayout, topIdx ocispec.Index, imageSubstring string) string {
	t.Helper()
	for _, m := range topIdx.Manifests {
		if !strings.Contains(m.Annotations[ocispec.AnnotationRefName], imageSubstring) {
			continue
		}
		require.Equal(t, ocispec.MediaTypeImageIndex, m.MediaType, "image %s must be stored as an OCI index", imageSubstring)
		blobPath := filepath.Join(pkgLayout.GetImageDirPath(), "blobs", "sha256", strings.TrimPrefix(m.Digest.String(), "sha256:"))
		b, err := os.ReadFile(blobPath)
		require.NoError(t, err)
		var pulledIdx ocispec.Index
		require.NoError(t, json.Unmarshal(b, &pulledIdx))
		require.Greater(t, len(pulledIdx.Manifests), 1, "expected multiple platform manifests under the %s index", imageSubstring)
		return m.Digest.String()
	}
	t.Fatalf("expected to find %s in the package layout", imageSubstring)
	return ""
}
