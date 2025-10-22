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
	// Check that the default value from embedded values.yaml was templated (not present in override-values.yaml)
	require.Contains(t, stdOut, "REMOVE_DEFAULT_VALUE=default-remove-value", "remove action should have templated the default value from embedded values.yaml")
	// Check that the override-value from override-values.yaml was templated
	require.Contains(t, stdOut, "REMOVE_TEST_VALUE=override-value", "remove action should have templated the override value from override-values.yaml")
	// Check that the custom value from --set-values was templated
	require.Contains(t, stdOut, "REMOVE_CUSTOM_VALUE=custom-remove-value", "remove action should have templated value from --set-values")
}
