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

func TestValues(t *testing.T) {
	t.Log("E2E: Values")

	src := filepath.Join("src", "test", "packages", "42-values")
	tmpdir := t.TempDir()

	// Create the package
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", src, "-o", tmpdir, "--skip-sbom", "--confirm", "--features=\"values=true\"")
	require.NoError(t, err, stdOut, stdErr)

	packageName := fmt.Sprintf("zarf-package-test-values-%s.tar.zst", e2e.Arch)
	path := filepath.Join(tmpdir, packageName)

	// Deploy the package with --set-values to override one value from the file
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm", "--set-values=.cliTest=cli-override-value", "--features=\"values=true\"")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the configmap field from values file is preserved
	kubectlOut, _, err := e2e.Kubectl(t, "get", "configmap", "test-values-configmap", "-o", "jsonpath='{.data.value}'")
	require.NoError(t, err, "unable to get configmap")
	require.Contains(t, kubectlOut, "default-value")

	// Verify the cliValue configmap field was overridden by CLI
	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-values-configmap", "-o", "jsonpath='{.data.cliValue}'")
	require.NoError(t, err, "unable to get configmap")
	require.Contains(t, kubectlOut, "cli-override-value")

	// Verify the action configmap was templated with the action-set values
	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-action-configmap", "-o", "jsonpath='{.data.json}'")
	require.NoError(t, err, "unable to get action configmap")
	require.Contains(t, kubectlOut, "myValue")
	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-action-configmap", "-o", "jsonpath='{.data.yaml}'")
	require.NoError(t, err, "unable to get action configmap")
	require.Contains(t, kubectlOut, "myValue")

	// Remove the package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "test-values", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
