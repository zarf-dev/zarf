// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPackageRemoveAfterFailedDeploy verifies that Zarf can successfully remove a package after a failed deployment.
func TestPackageRemoveAfterFailedDeploy(t *testing.T) {
	t.Log("E2E: Package Remove After Failed Deploy")
	e2e.SetupWithCluster(t)

	goodPath := fmt.Sprintf("build/zarf-package-dos-games-%s-1.0.0.tar.zst", e2e.Arch)
	evilPath := fmt.Sprintf("zarf-package-dos-games-%s.tar.zst", e2e.Arch)

	// Create the evil package (with the bad service).
	stdOut, stdErr, err := e2e.Zarf("package", "create", "src/test/packages/25-evil-dos-games/", "--skip-sbom", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the good package.
	stdOut, stdErr, err = e2e.Zarf("package", "deploy", goodPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the evil package.
	stdOut, stdErr, err = e2e.Zarf("package", "deploy", evilPath, "--confirm")
	require.Error(t, err, stdOut, stdErr)

	// Remove the package.
	stdOut, stdErr, err = e2e.Zarf("package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Ensure the dos-games chart was uninstalled.
	helmOut, err := exec.Command("helm", "list", "-n", "dos-games").Output()
	require.NoError(t, err)
	require.NotContains(t, string(helmOut), "zarf-f53a99d4a4dd9a3575bedf59cd42d48d751ae866")
}
