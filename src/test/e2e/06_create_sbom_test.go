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
)

func TestCreateSBOM(t *testing.T) {
	cachePath := filepath.Join(os.TempDir(), ".cache-location")
	sbomPath := filepath.Join(os.TempDir(), ".sbom-location")

	e2e.CleanFiles(cachePath, sbomPath)

	pkgName := fmt.Sprintf("zarf-package-dos-games-%s.tar.zst", e2e.Arch)

	stdOut, stdErr, err := e2e.ZarfWithConfirm("package", "create", "examples/dos-games", "--zarf-cache", cachePath, "--sbom-out", sbomPath)
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Creating SBOMs for 1 images and 0 components with files.")
	// Test that the game package generates the SBOMs we expect (images only)
	_, err = os.ReadFile(filepath.Join(sbomPath, "dos-games", "sbom-viewer-defenseunicorns_zarf-game_multi-tile-dark.html"))
	require.NoError(t, err)
	_, err = os.ReadFile(filepath.Join(sbomPath, "dos-games", "compare.html"))
	require.NoError(t, err)
	_, err = os.ReadFile(filepath.Join(sbomPath, "dos-games", "defenseunicorns_zarf-game_multi-tile-dark.json"))
	require.NoError(t, err)

	// Clean the SBOM path so it is force to be recreated
	e2e.CleanFiles(sbomPath)

	stdOut, stdErr, err = e2e.Zarf("package", "inspect", pkgName, "--sbom-out", sbomPath)
	require.NoError(t, err, stdOut, stdErr)
	// Test that the game package generates the SBOMs we expect (images only)
	_, err = os.ReadFile(filepath.Join(sbomPath, "dos-games", "sbom-viewer-defenseunicorns_zarf-game_multi-tile-dark.html"))
	require.NoError(t, err)
	_, err = os.ReadFile(filepath.Join(sbomPath, "dos-games", "compare.html"))
	require.NoError(t, err)
	_, err = os.ReadFile(filepath.Join(sbomPath, "dos-games", "defenseunicorns_zarf-game_multi-tile-dark.json"))
	require.NoError(t, err)

	// Pull the current zarf binary version to find the corresponding init package
	version, stdErr, err := e2e.Zarf("version")
	require.NoError(t, err, version, stdErr)

	initName := fmt.Sprintf("build/zarf-init-%s-%s.tar.zst", e2e.Arch, strings.TrimSpace(version))

	stdOut, stdErr, err = e2e.Zarf("package", "inspect", initName, "--sbom-out", sbomPath)
	require.NoError(t, err, stdOut, stdErr)
	// Test that we preserve the filepath
	_, err = os.ReadFile(filepath.Join(sbomPath, "dos-games", "sbom-viewer-defenseunicorns_zarf-game_multi-tile-dark.html"))
	require.NoError(t, err)
	// Test that the init package generates the SBOMs we expect (images + component files)
	_, err = os.ReadFile(filepath.Join(sbomPath, "init", "sbom-viewer-gitea_gitea_1.18.5-rootless.html"))
	require.NoError(t, err)
	_, err = os.ReadFile(filepath.Join(sbomPath, "init", "gitea_gitea_1.18.5-rootless.json"))
	require.NoError(t, err)
	_, err = os.ReadFile(filepath.Join(sbomPath, "init", "sbom-viewer-zarf-component-k3s.html"))
	require.NoError(t, err)
	_, err = os.ReadFile(filepath.Join(sbomPath, "init", "zarf-component-k3s.json"))
	require.NoError(t, err)
	_, err = os.ReadFile(filepath.Join(sbomPath, "init", "compare.html"))
	require.NoError(t, err)

	e2e.CleanFiles(cachePath, sbomPath, pkgName)
}
