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

func TestNamespaceOverride(t *testing.T) {
	t.Log("E2E: Namespace override")
	tmpDir := t.TempDir()

	goodPackage := fmt.Sprintf("%s/zarf-package-test-package-%s-0.0.1.tar.zst", tmpDir, e2e.Arch)
	badPackage := fmt.Sprintf("%s/zarf-package-test-package-fail-%s-0.0.1.tar.zst", tmpDir, e2e.Arch)

	// Create a package without a namespace override (baseline)
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "src/test/packages/40-namespace-override", "-o", tmpDir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the baseline package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", goodPackage, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Query the state of the package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "list")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "test-package")

	// Deploy the package with a namespace override
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", goodPackage, "--namespace", "test2", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Query the state of the package - now includes the namespace-override package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "list")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "test-package-test2")

	// Deploy the package using a namespace override in a config file - targeting the test3 namespace
	t.Setenv("ZARF_CONFIG", filepath.Join("src", "test", "packages", "40-namespace-override", "zarf-config.yaml"))
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", goodPackage, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Query the state of the package - now includes the namespace-override package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "list")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "test-package-test3")

	// Create the bad package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", "src/test/packages/40-namespace-override/multi-ns", "-o", tmpDir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Attempt to deploy the bad package - should fail
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", badPackage, "--namespace", "test4", "--confirm")
	require.Error(t, err, stdOut, stdErr)

	// Remove each zarf package
	for _, pkg := range []string{"test-package", "test-package-test2", "test-package-test3"} {
		stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", pkg, "--confirm")
		require.NoError(t, err, stdOut, stdErr)
	}
}
