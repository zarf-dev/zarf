// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2"

	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestCreateSBOM(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	outSbomPath := filepath.Join(t.TempDir(), ".sbom-location")
	buildPath := t.TempDir()
	tarPath := filepath.Join(buildPath, fmt.Sprintf("zarf-package-dos-games-%s-1.3.0.tar.zst", e2e.Arch))

	expectedFiles := []string{
		"sbom-viewer-ghcr.io_zarf-dev_doom-game_0.0.1.html",
		"ghcr.io_zarf-dev_doom-game_0.0.1.json",
	}

	_, _, err := e2e.Zarf(t, "package", "create", "examples/dos-games", "-o", buildPath, "--sbom-out", outSbomPath, "--confirm")
	require.NoError(t, err)

	pkgLayout, err := layout.LoadFromTar(ctx, tarPath, layout.PackageLayoutOptions{})
	require.NoError(t, err)
	getSbomPath := t.TempDir()
	err = pkgLayout.GetSBOM(ctx, getSbomPath)
	require.NoError(t, err)
	for _, expectedFile := range expectedFiles {
		require.FileExists(t, filepath.Join(getSbomPath, expectedFile))
		require.FileExists(t, filepath.Join(outSbomPath, "dos-games", expectedFile))
	}

	// Clean the SBOM path so it is force to be recreated
	err = os.RemoveAll(outSbomPath)
	require.NoError(t, err)

	_, _, err = e2e.Zarf(t, "package", "inspect", "sbom", tarPath, "--output", outSbomPath)
	require.NoError(t, err)

	for _, expectedFile := range expectedFiles {
		require.FileExists(t, filepath.Join(outSbomPath, "dos-games", expectedFile))
	}

	stdOut, _, err := e2e.Zarf(t, "package", "inspect", "images", tarPath)
	require.NoError(t, err)
	require.Contains(t, stdOut, "- ghcr.io/zarf-dev/doom-game:0.0.1\n")

	// Pull the current zarf binary version to find the corresponding init package
	version, _, err := e2e.Zarf(t, "version")
	require.NoError(t, err)

	initName := fmt.Sprintf("build/zarf-init-%s-%s.tar.zst", e2e.Arch, strings.TrimSpace(version))
	_, _, err = e2e.Zarf(t, "package", "inspect", "sbom", initName, "--output", outSbomPath)
	require.NoError(t, err)

	// Test that we preserve the filepath
	require.FileExists(t, filepath.Join(outSbomPath, "dos-games", "sbom-viewer-ghcr.io_zarf-dev_doom-game_0.0.1.html"))
	require.FileExists(t, filepath.Join(outSbomPath, "init", "sbom-viewer-ghcr.io_go-gitea_gitea_1.24.6-rootless.html"))
	require.FileExists(t, filepath.Join(outSbomPath, "init", "ghcr.io_go-gitea_gitea_1.24.6-rootless.json"))
	require.FileExists(t, filepath.Join(outSbomPath, "init", "sbom-viewer-zarf-component-k3s.html"))
	require.FileExists(t, filepath.Join(outSbomPath, "init", "zarf-component-k3s.json"))
}

func TestCreateSBOMSkipsOCIArtifact(t *testing.T) {
	ctx := testutil.TestContext(t)
	registryURL := testutil.SetupInMemoryRegistryDynamic(ctx, t)

	// Create a runtime image with a single layer that is a tar archive
	runtimeRef := fmt.Sprintf("%s/runtime:latest", registryURL)
	runtimeRepo := testutil.NewRepo(t, runtimeRef)
	tarLayer, gzipLayer := createGzipTarLayer(t)
	runtimeLayer := testutil.PushBlob(ctx, t, runtimeRepo, ocispec.MediaTypeImageLayerGzip, gzipLayer)
	runtimeConfig := testutil.PushBlob(ctx, t, runtimeRepo, ocispec.MediaTypeImageConfig, fmt.Appendf(nil, `{"architecture":"amd64","os":"linux","rootfs":{"type":"layers","diff_ids":[%q]}}`, digest.FromBytes(tarLayer)))
	runtimeManifest := testutil.PushManifest(ctx, t, runtimeRepo, runtimeConfig, []ocispec.Descriptor{runtimeLayer})
	require.NoError(t, runtimeRepo.Tag(ctx, runtimeManifest, "latest"))

	// Create an OCI artifact with a single layer that is not a tar archive
	artifactRef := fmt.Sprintf("%s/data-artifact:latest", registryURL)
	artifactRepo := testutil.NewRepo(t, artifactRef)
	artifactLayer, err := oras.PushBytes(ctx, artifactRepo, ocispec.MediaTypeImageLayer, []byte("this payload is not a tar archive"))
	require.NoError(t, err)
	artifactManifest, err := oras.PackManifest(ctx, artifactRepo, oras.PackManifestVersion1_0, "application/vnd.example.data.config.v1+json", oras.PackManifestOptions{
		Layers: []ocispec.Descriptor{artifactLayer},
	})
	require.NoError(t, err)
	require.NoError(t, artifactRepo.Tag(ctx, artifactManifest, "latest"))

	// Create a Zarf package that includes both the runtime image and the OCI artifact
	pkgDir := t.TempDir()
	pkgDefinition := fmt.Sprintf(`kind: ZarfPackageConfig
metadata:
  name: sbom-oci-artifact
  version: 0.0.1
components:
  - name: images
    required: true
    images:
      - %s
      - %s
`, runtimeRef, artifactRef)
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "zarf.yaml"), []byte(pkgDefinition), 0o600))

	// Create the Zarf package and verify that the SBOM is only generated for the runtime image and not for the OCI artifact
	buildDir := t.TempDir()
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", pkgDir, "-o", buildDir, "--plain-http", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "creating image SBOM reference="+runtimeRef)
	require.NotContains(t, stdErr, "creating image SBOM reference="+artifactRef)

	// Verify that the SBOM for the runtime image exists and the SBOM for the OCI artifact does not exist
	pkgPath := filepath.Join(buildDir, fmt.Sprintf("zarf-package-sbom-oci-artifact-%s-0.0.1.tar.zst", e2e.Arch))
	pkgLayout, err := layout.LoadFromTar(ctx, pkgPath, layout.PackageLayoutOptions{})
	require.NoError(t, err)
	sbomDir := t.TempDir()
	require.NoError(t, pkgLayout.GetSBOM(ctx, sbomDir))
	require.FileExists(t, filepath.Join(sbomDir, normalizeSBOMName(runtimeRef)+".json"))
	require.NoFileExists(t, filepath.Join(sbomDir, normalizeSBOMName(artifactRef)+".json"))
}

func createGzipTarLayer(t *testing.T) ([]byte, []byte) {
	t.Helper()

	var tarBuffer bytes.Buffer
	tarWriter := tar.NewWriter(&tarBuffer)
	contents := []byte("hello from a container image\n")
	require.NoError(t, tarWriter.WriteHeader(&tar.Header{Name: "hello.txt", Mode: 0o644, Size: int64(len(contents))}))
	_, err := tarWriter.Write(contents)
	require.NoError(t, err)
	require.NoError(t, tarWriter.Close())

	var gzipBuffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&gzipBuffer)
	_, err = gzipWriter.Write(tarBuffer.Bytes())
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())

	return tarBuffer.Bytes(), gzipBuffer.Bytes()
}

func normalizeSBOMName(identifier string) string {
	return strings.NewReplacer("/", "_", ":", "_").Replace(identifier)
}
