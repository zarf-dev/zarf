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

func TestHelmPreInstallHook(t *testing.T) {
	t.Log("E2E: Helm pre-install hook")

	tmpdir := t.TempDir()
	packagePath := filepath.Join("src", "test", "packages", "46-helm-hooks")

	// Create the package
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", packagePath, "-o", tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the package
	pkgPath := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-helm-hooks-%s-0.1.0.tar.zst", e2e.Arch))
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", pkgPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the pre-install hook ConfigMap was created
	kubectlOut, _, err := e2e.Kubectl(t, "-n", "helm-hooks", "get", "configmap", "pre-install-hook-config", "-o", "jsonpath={.data.message}")
	require.NoError(t, err, "pre-install hook ConfigMap should exist")
	require.Equal(t, "This was created by a pre-install hook", kubectlOut, "pre-install hook ConfigMap should have correct data")

	// Verify the main ConfigMap was also created
	kubectlOut, _, err = e2e.Kubectl(t, "-n", "helm-hooks", "get", "configmap", "main-config", "-o", "jsonpath={.data.message}")
	require.NoError(t, err, "main ConfigMap should exist")
	require.Equal(t, "This is the main config deployed after hooks", kubectlOut, "main ConfigMap should have correct data")

	// Remove the package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "helm-hooks", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
