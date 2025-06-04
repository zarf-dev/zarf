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

func TestSingleNamespaceOverride(t *testing.T) {
	t.Log("E2E: Namespace override")
	tmpDir := t.TempDir()

	// Create a package without a namespace override (baseline)
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "src/test/packages/40-namespace-override", "-o", tmpDir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	singlePackage := fmt.Sprintf("%s/zarf-package-test-package-%s-0.0.1.tar.zst", tmpDir, e2e.Arch)

	// Deploy the baseline package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", singlePackage, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Query the state of the package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "list")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "test-package")

	// Deploy the package with a namespace override while retaining the baseline package to check for conflicts
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", singlePackage, "--namespace", "test2", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Query the state of the package - now includes the namespace-override package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "list")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "test-package-test2")

	// Remove the baseline and original override packages via deployed package name
	for _, pkg := range []string{"test-package", "test-package-test2"} {
		stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", pkg, "--confirm")
		require.NoError(t, err, stdOut, stdErr)
	}

	// Redeploy the test2 package override to test tarball removal with namespace flag
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", singlePackage, "--namespace", "test2", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Query the state of the package - now includes the namespace-override package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "list")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "test-package-test2")

	// Remove the remaining package via tarball using the config file namespace override
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", singlePackage, "--namespace", "test2", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the package using a namespace override in a config file - targeting the test3 namespace
	t.Setenv("ZARF_CONFIG", filepath.Join("src", "test", "packages", "40-namespace-override", "zarf-config.yaml"))
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", singlePackage, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Query the state of the package - now includes the namespace-override package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "list")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "test-package-test3")

	// Remove the remaining package via tarball using the config file namespace override
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", singlePackage, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func TestMultiNamespaceOverride(t *testing.T) {
	t.Log("E2E: Multi-namespace override")
	tmpDir := t.TempDir()

	// Create the bad package
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "src/test/packages/40-namespace-override/multi-ns", "-o", tmpDir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	multiPackage := fmt.Sprintf("%s/zarf-package-test-package-fail-%s-0.0.1.tar.zst", tmpDir, e2e.Arch)

	// Attempt to deploy the bad package - should fail
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", multiPackage, "--namespace", "test4", "--confirm")
	require.Error(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "cannot override namespace to test4")
}
