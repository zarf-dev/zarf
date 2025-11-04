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

	// Deploy the package with both package values and CLI override values
	overrideValuesPath := filepath.Join(src, "override-values.yaml")
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm", "--features=\"values=true\"", "--values", overrideValuesPath)
	require.NoError(t, err, stdOut, stdErr)

	// Verify the configmap was templated with the override value (CLI takes precedence)
	kubectlOut, _, err := e2e.Kubectl(t, "get", "configmap", "test-values-configmap", "-o", "jsonpath='{.data.value}'")
	require.NoError(t, err, "unable to get configmap")
	require.Contains(t, kubectlOut, "override-value")

	// Verify additional field has the value from CLI override file
	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-values-configmap", "-o", "jsonpath='{.data.additional}'")
	require.NoError(t, err, "unable to get configmap")
	require.Contains(t, kubectlOut, "extra-data")

	// Verify the action configmap was templated with the action-set values
	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-action-configmap", "-o", "jsonpath='{.data.json}'")
	require.NoError(t, err, "unable to get action configmap")
	require.Contains(t, kubectlOut, "myValue")
	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-action-configmap", "-o", "jsonpath='{.data.yaml}'")
	require.NoError(t, err, "unable to get action configmap")
	require.Contains(t, kubectlOut, "myValue")

	// Verify the raw template configmap was NOT processed by Zarf (template: false)
	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-raw-template-configmap", "-o", "jsonpath='{.data.rawTemplate}'")
	require.NoError(t, err, "unable to get raw template configmap")
	require.Contains(t, kubectlOut, "template={{ .shouldNotBeProcessed }}")

	// Remove the package with values
	valuesFile := filepath.Join(src, "override-values.yaml")
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "test-values", "--confirm", "--features=\"values=true\"", "--values", valuesFile, "--set-values", "removeKey=custom-remove-value")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the remove actions used the values correctly
	// Check that the override-value from override-values.yaml was templated
	require.Contains(t, stdOut, "REMOVE_TEST_VALUE=override-value", "remove action should have templated the override value from override-values.yaml")
	// Check that the custom value from --set-values was templated
	require.Contains(t, stdOut, "REMOVE_CUSTOM_VALUE=custom-remove-value", "remove action should have templated value from --set-values")
}

func TestValuesSchema(t *testing.T) {
	t.Log("E2E: Values Schema Validation")

	t.Run("valid values pass schema validation at create time", func(t *testing.T) {
		src := filepath.Join("src", "test", "packages", "42-values", "schema-valid")
		tmpdir := t.TempDir()

		// Create should succeed with valid values
		stdOut, stdErr, err := e2e.Zarf(t, "package", "create", src, "-o", tmpdir, "--skip-sbom", "--confirm", "--features=\"values=true\"")
		require.NoError(t, err, stdOut, stdErr)

		// Deploy should also succeed
		packageName := fmt.Sprintf("zarf-package-test-values-schema-valid-%s.tar.zst", e2e.Arch)
		path := filepath.Join(tmpdir, packageName)
		stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm", "--features=\"values=true\"")
		require.NoError(t, err, stdOut, stdErr)

		// Verify the configmap was created with the correct values
		kubectlOut, _, err := e2e.Kubectl(t, "get", "configmap", "test-values-schema-configmap", "-o", "jsonpath='{.data.appName}'")
		require.NoError(t, err, "unable to get configmap")
		require.Contains(t, kubectlOut, "test-app")

		// Cleanup
		stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "test-values-schema-valid", "--confirm")
		require.NoError(t, err, stdOut, stdErr)
	})

	t.Run("invalid values fail schema validation at create time", func(t *testing.T) {
		src := filepath.Join("src", "test", "packages", "42-values", "schema-invalid")
		tmpdir := t.TempDir()

		// Create should fail with invalid values
		stdOut, stdErr, err := e2e.Zarf(t, "package", "create", src, "-o", tmpdir, "--skip-sbom", "--confirm", "--features=\"values=true\"")
		require.Error(t, err, "expected error for invalid values")
		// Check that the error message mentions validation failure
		output := stdOut + stdErr
		require.Contains(t, output, "values validation failed", "error should mention schema validation failure")
	})

	t.Run("invalid override values fail schema validation at deploy time", func(t *testing.T) {
		src := filepath.Join("src", "test", "packages", "42-values", "schema-deploy-invalid")
		tmpdir := t.TempDir()

		// Create should succeed with valid default values
		stdOut, stdErr, err := e2e.Zarf(t, "package", "create", src, "-o", tmpdir, "--skip-sbom", "--confirm", "--features=\"values=true\"")
		require.NoError(t, err, stdOut, stdErr)

		// Deploy with invalid override values should fail
		packageName := fmt.Sprintf("zarf-package-test-values-schema-deploy-invalid-%s.tar.zst", e2e.Arch)
		path := filepath.Join(tmpdir, packageName)
		overrideValuesPath := filepath.Join(src, "override-invalid.yaml")
		_, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm", "--features=\"values=true\"", "--values", overrideValuesPath)
		require.Error(t, err, "expected error for invalid override values at deploy time")
		// Check that the error message mentions validation failure
		require.Contains(t, stdErr, "values validation failed", "error should mention schema validation failure")
	})
}
