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

// This digest is the multi-arch index manifest of ghcr.io/stefanprodan/podinfo:6.4.0.
const multiArchPodinfoIndexDigest = "sha256:57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8"

func TestMultiArchPackage(t *testing.T) {
	t.Log("E2E: multi-arch package create + publish + pull + deploy")

	pkgDefinitionPath := filepath.Join("src", "test", "packages", "48-multi-arch")
	createDir := t.TempDir()

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", pkgDefinitionPath, "-o", createDir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	createdPkgPath := filepath.Join(createDir, "zarf-package-multi-arch-multi-0.0.1.tar.zst")
	require.FileExists(t, createdPkgPath, "package filename must include the multi architecture suffix")

	registryURL := testutil.SetupInMemoryRegistryDynamic(testutil.TestContext(t), t)
	ref := registry.Reference{
		Registry:   registryURL,
		Repository: "multi-arch",
		Reference:  "0.0.1",
	}

	stdOut, stdErr, err = e2e.Zarf(t, "package", "publish", createdPkgPath, "oci://"+registryURL, "--plain-http")
	require.NoError(t, err, stdOut, stdErr)

	pullDir := t.TempDir()
	stdOut, stdErr, err = e2e.Zarf(t, "package", "pull", "oci://"+ref.String(), "--plain-http", "-o", pullDir, "-a", "multi")
	require.NoError(t, err, stdOut, stdErr)

	pulledPkgPath := filepath.Join(pullDir, "zarf-package-multi-arch-multi-0.0.1.tar.zst")
	pkgLayout, err := layout.LoadFromTar(t.Context(), pulledPkgPath, layout.PackageLayoutOptions{})
	require.NoError(t, err)

	idxBytes, err := os.ReadFile(filepath.Join(pkgLayout.GetImageDirPath(), "index.json"))
	require.NoError(t, err)
	var idx ocispec.Index
	require.NoError(t, json.Unmarshal(idxBytes, &idx))

	var rootDigest string
	for _, m := range idx.Manifests {
		if strings.Contains(m.Annotations[ocispec.AnnotationRefName], multiArchPodinfoIndexDigest) {
			require.Equal(t, ocispec.MediaTypeImageIndex, m.MediaType, "multi-arch image must be stored as an OCI index")
			rootDigest = m.Digest.String()
			break
		}
	}
	require.Equal(t, multiArchPodinfoIndexDigest, rootDigest, "expected to find the podinfo index manifest in the package layout")

	blobPath := filepath.Join(pkgLayout.GetImageDirPath(), "blobs", "sha256", strings.TrimPrefix(rootDigest, "sha256:"))
	b, err := os.ReadFile(blobPath)
	require.NoError(t, err)
	var pulledIdx ocispec.Index
	require.NoError(t, json.Unmarshal(b, &pulledIdx))
	require.Greater(t, len(pulledIdx.Manifests), 1, "expected multiple platform manifests under the index")

	sbomDir := t.TempDir()
	require.NoError(t, pkgLayout.GetSBOM(t.Context(), sbomDir))
	sbomEntries, err := os.ReadDir(sbomDir)
	require.NoError(t, err)
	var platformSBOMs []string
	for _, entry := range sbomEntries {
		name := entry.Name()
		if strings.HasSuffix(name, ".json") && strings.Contains(name, multiArchPodinfoIndexDigest[len("sha256:"):]) {
			platformSBOMs = append(platformSBOMs, name)
		}
	}
	require.GreaterOrEqual(t, len(platformSBOMs), 2, "expected per-platform SBOMs for the multi-arch image, got %v", platformSBOMs)

	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", pulledPkgPath, "--confirm", "--skip-version-check")
	require.NoError(t, err, stdOut, stdErr)
	t.Cleanup(func() {
		_, _, err = e2e.Zarf(t, "package", "remove", "multi-arch", "--confirm", "--skip-version-check")
		require.NoError(t, err)
	})
}
