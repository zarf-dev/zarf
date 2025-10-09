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

	src := filepath.Join("src", "test", "packages", "42_values", "basic")
	tmpdir := t.TempDir()

	// Create the package
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", src, "-o", tmpdir, "--skip-sbom", "--confirm", "--features=\"values=true\"")
	require.NoError(t, err, stdOut, stdErr)

	packageName := fmt.Sprintf("zarf-package-test-values-%s.tar.zst", e2e.Arch)
	path := filepath.Join(tmpdir, packageName)

	// Deploy the package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the configmap was templated with the default value
	kubectlOut, _, err := e2e.Kubectl(t, "get", "configmap", "test-values-configmap", "-o", "jsonpath='{.data.value}'")
	require.NoError(t, err, "unable to get configmap")
	require.Contains(t, kubectlOut, "default-value")

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

func TestValuesSchema(t *testing.T) {
	t.Log("E2E: Values Schema Validation")

	src := filepath.Join("src", "test", "packages", "42_values", "schema-valid")
	tmpdir := t.TempDir()

	// Valid values should pass during create
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", src, "-o", tmpdir, "--skip-sbom", "--confirm", "--features=\"values=true\"")
	require.NoError(t, err, stdOut, stdErr)

	packageName := fmt.Sprintf("zarf-package-test-values-schema-%s.tar.zst", e2e.Arch)
	path := filepath.Join(tmpdir, packageName)

	// Valid values should pass during deploy
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the configmap was created with schema-validated values
	kubectlOut, _, err := e2e.Kubectl(t, "get", "configmap", "test-schema-configmap", "-o", "jsonpath='{.data.replicas}'")
	require.NoError(t, err, "unable to get configmap")
	require.Contains(t, kubectlOut, "3")

	// Remove the package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "test-values-schema", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Invalid values should fail during create
	invalidSrc := filepath.Join("src", "test", "packages", "42_values", "schema-invalid")
	_, stdErr, err = e2e.Zarf(t, "package", "create", invalidSrc, "-o", tmpdir, "--skip-sbom", "--confirm", "--features=\"values=true\"")
	require.Error(t, err, "package create should fail with invalid values")
	require.Contains(t, stdErr, "schema validation failed", "error should mention schema validation")
}
