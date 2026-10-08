// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManifestWithSymlink(t *testing.T) {
	t.Log("E2E: Manifest With Symlink")

	tmpdir := t.TempDir()
	buildPath := filepath.Join("src", "test", "packages", "34-manifest-with-symlink")
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", buildPath, "-o", tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	packageName := fmt.Sprintf("zarf-package-manifest-with-symlink-%s-0.0.1.tar.zst", e2e.Arch)
	path := filepath.Join(tmpdir, packageName)
	require.FileExists(t, path)

	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
	defer e2e.CleanFiles(t, "temp/manifests")
	require.NoError(t, err, stdOut, stdErr)

	materializedPath := filepath.Join("temp", "manifests", "resources", "img")
	info, err := os.Lstat(materializedPath)
	require.NoError(t, err)
	require.True(t, info.IsDir(), "expected a materialized directory, got mode %s", info.Mode())
	require.Zero(t, info.Mode()&os.ModeSymlink)
	require.FileExists(t, filepath.Join(materializedPath, "test.txt"))
}
