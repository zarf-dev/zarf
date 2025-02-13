// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	layout2 "github.com/zarf-dev/zarf/src/internal/packager2/layout"
)

func TestCreateSBOM(t *testing.T) {
	t.Parallel()

	outSbomPath := filepath.Join(t.TempDir(), ".sbom-location")
	buildPath := t.TempDir()
	tarPath := filepath.Join(buildPath, fmt.Sprintf("zarf-package-dos-games-%s-1.1.0.tar.zst", e2e.Arch))

	expectedFiles := []string{
		"sbom-viewer-ghcr.io_zarf-dev_doom-game_0.0.1.html",
		"compare.html",
		"ghcr.io_zarf-dev_doom-game_0.0.1.json",
	}

	_, _, err := e2e.Zarf(t, "package", "create", "examples/dos-games", "-o", buildPath, "--sbom-out", outSbomPath, "--confirm")
	require.NoError(t, err)

	pkgLayout, err := layout2.LoadFromTar(context.Background(), tarPath, layout2.PackageLayoutOptions{})
	require.NoError(t, err)
	getSbomPath, err := pkgLayout.GetSBOM(t.TempDir())
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
	require.FileExists(t, filepath.Join(outSbomPath, "init", "sbom-viewer-docker.io_gitea_gitea_1.21.5-rootless.html"))
	require.FileExists(t, filepath.Join(outSbomPath, "init", "docker.io_gitea_gitea_1.21.5-rootless.json"))
	require.FileExists(t, filepath.Join(outSbomPath, "init", "sbom-viewer-zarf-component-k3s.html"))
	require.FileExists(t, filepath.Join(outSbomPath, "init", "zarf-component-k3s.json"))
	require.FileExists(t, filepath.Join(outSbomPath, "init", "compare.html"))
}
