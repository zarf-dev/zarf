// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestCreateSBOM(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	const (
		imageSBOM   = "ghcr.io_zarf-dev_doom-game_0.0.1.json"
		sbomViewer  = "sbom-viewer-ghcr.io_zarf-dev_doom-game_0.0.1.html"
		packageName = "dos-games"
	)

	defaultSBOMPath := t.TempDir()
	defaultBuildPath := t.TempDir()
	defaultTarPath := filepath.Join(defaultBuildPath, fmt.Sprintf("zarf-package-%s-%s-1.3.0.tar.zst", packageName, e2e.Arch))

	_, _, err := e2e.Zarf(t, "package", "create", "examples/dos-games", "-o", defaultBuildPath, "--sbom-out", defaultSBOMPath, "--confirm")
	require.NoError(t, err)

	defaultPkgLayout, err := layout.LoadFromTar(ctx, defaultTarPath, layout.PackageLayoutOptions{})
	require.NoError(t, err)
	defaultExtractPath := t.TempDir()
	err = defaultPkgLayout.GetSBOM(ctx, defaultExtractPath)
	require.NoError(t, err)
	for _, sbomPath := range []string{defaultExtractPath, filepath.Join(defaultSBOMPath, packageName)} {
		require.FileExists(t, filepath.Join(sbomPath, imageSBOM))
		require.NoFileExists(t, filepath.Join(sbomPath, sbomViewer))
	}

	legacySBOMPath := t.TempDir()
	legacyBuildPath := t.TempDir()
	legacyTarPath := filepath.Join(legacyBuildPath, fmt.Sprintf("zarf-package-%s-%s-1.3.0.tar.zst", packageName, e2e.Arch))
	_, _, err = e2e.Zarf(t, "package", "create", "examples/dos-games", "-o", legacyBuildPath, "--features=sbom-viewer=true", "--sbom-out", legacySBOMPath, "--confirm")
	require.NoError(t, err)

	legacyPkgLayout, err := layout.LoadFromTar(ctx, legacyTarPath, layout.PackageLayoutOptions{})
	require.NoError(t, err)
	legacyExtractPath := t.TempDir()
	err = legacyPkgLayout.GetSBOM(ctx, legacyExtractPath)
	require.NoError(t, err)
	for _, sbomPath := range []string{legacyExtractPath, filepath.Join(legacySBOMPath, packageName)} {
		require.FileExists(t, filepath.Join(sbomPath, imageSBOM))
		require.FileExists(t, filepath.Join(sbomPath, sbomViewer))
	}

	// Clean the SBOM path so it is forced to be recreated by inspect.
	err = os.RemoveAll(defaultSBOMPath)
	require.NoError(t, err)
	_, _, err = e2e.Zarf(t, "package", "inspect", "sbom", defaultTarPath, "--output", defaultSBOMPath)
	require.NoError(t, err)

	// Test that we preserve the package-name directory.
	require.FileExists(t, filepath.Join(defaultSBOMPath, packageName, imageSBOM))
	require.NoFileExists(t, filepath.Join(defaultSBOMPath, packageName, sbomViewer))

	stdOut, _, err := e2e.Zarf(t, "package", "inspect", "images", defaultTarPath)
	require.NoError(t, err)
	require.Contains(t, stdOut, "- ghcr.io/zarf-dev/doom-game:0.0.1\n")

	// Pull the current zarf binary version to find the corresponding init package.
	version, _, err := e2e.Zarf(t, "version")
	require.NoError(t, err)

	initName := fmt.Sprintf("build/zarf-init-%s-%s.tar.zst", e2e.Arch, strings.TrimSpace(version))
	_, _, err = e2e.Zarf(t, "package", "inspect", "sbom", initName, "--output", defaultSBOMPath)
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(defaultSBOMPath, packageName, imageSBOM))
	require.NoFileExists(t, filepath.Join(defaultSBOMPath, packageName, sbomViewer))
	require.FileExists(t, filepath.Join(defaultSBOMPath, "init", "ghcr.io_go-gitea_gitea_1.27.3-rootless.json"))
	require.NoFileExists(t, filepath.Join(defaultSBOMPath, "init", "sbom-viewer-ghcr.io_go-gitea_gitea_1.27.3-rootless.html"))
	require.FileExists(t, filepath.Join(defaultSBOMPath, "init", "zarf-component-k3s.json"))
}
