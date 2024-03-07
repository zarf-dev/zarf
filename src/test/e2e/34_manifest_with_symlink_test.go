// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManifestWithSymlink(t *testing.T) {
	t.Log("E2E: Manifest With Symlink")

	// Build the package, should succeed, even though there is a symlink in the package.
	buildPath := filepath.Join("src", "test", "packages", "34-manifest-with-symlink")
	stdOut, stdErr, err := e2e.Zarf("package", "create", buildPath, "-o=build", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	path := fmt.Sprintf("build/zarf-package-manifest-with-symlink-%s-0.0.1.tar.zst", e2e.Arch)
	require.FileExists(t, path)
	defer e2e.CleanFiles(path)

	stdOut, stdErr, err = e2e.Zarf("package", "deploy", path, "--confirm")
	defer e2e.CleanFiles("temp/manifests")
	require.NoError(t, err, stdOut, stdErr)
	require.FileExists(t, "temp/manifests/resources/img", "Symlink does not exist in the package as expected.")
}
