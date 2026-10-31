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

// TestDifferentialPrune ensures images a differential package omits are still treated as in use after deploy.
func TestDifferentialPrune(t *testing.T) {
	t.Log("E2E: Differential package images survive registry prune")
	tmpDir := t.TempDir()
	packagePath := "src/test/packages/53-differential-prune"
	basePackage := filepath.Join(tmpDir, fmt.Sprintf("zarf-package-differential-prune-%s-1.0.0.tar.zst", e2e.Arch))
	differentialPackage := filepath.Join(tmpDir, fmt.Sprintf("zarf-package-differential-prune-%s-1.0.0-differential-2.0.0.tar.zst", e2e.Arch))
	unchangedImage := "ghcr.io/stefanprodan/podinfo:6.0.0"

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", packagePath, "-o", tmpDir, "--set=PACKAGE_VERSION=1.0.0", "--set=UPDATED_TAG=6.0.1", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", packagePath, "-o", tmpDir, "--set=PACKAGE_VERSION=2.0.0", "--set=UPDATED_TAG=6.0.2", "--differential", basePackage, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// The differential package must not ship the unchanged image but must record it
	stdOut, stdErr, err = e2e.Zarf(t, "package", "inspect", "definition", differentialPackage)
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "differentialImages")

	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", basePackage, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", differentialPackage, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	t.Cleanup(func() {
		stdOut, stdErr, err := e2e.Zarf(t, "package", "remove", "differential-prune", "--confirm")
		require.NoError(t, err, stdOut, stdErr)
	})

	// The deployed definition must still list the image the differential package omitted
	stdOut, stdErr, err = e2e.Zarf(t, "package", "inspect", "definition", "differential-prune")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, unchangedImage)

	// The superseded image is pruned
	stdOut, stdErr, err = e2e.Zarf(t, "tools", "registry", "prune", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Prune errors on any deployed image missing from the registry, so a clean run with nothing left to prune
	// shows the unchanged image survived and is still considered in use
	stdOut, stdErr, err = e2e.Zarf(t, "tools", "registry", "prune", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "there are no images to prune")
}
