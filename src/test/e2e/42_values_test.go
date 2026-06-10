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

	// Deploy the package with both package values, CLI override values file, and --set-values
	overrideValuesPath := filepath.Join(src, "override-values.yaml")
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm", "--features=\"values=true\"", "--values", overrideValuesPath, "--set-values", "cliOverride=cli-wins", "--skip-version-check")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the configmap was templated with the override value (--values file overrides package defaults)
	kubectlOut, _, err := e2e.Kubectl(t, "get", "configmap", "test-values-configmap", "-o", "jsonpath='{.data.value}'")
	require.NoError(t, err, "unable to get configmap")
	require.Contains(t, kubectlOut, "override-value")

	// Verify additional field has the value from CLI override file
	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-values-configmap", "-o", "jsonpath='{.data.additional}'")
	require.NoError(t, err, "unable to get configmap")
	require.Contains(t, kubectlOut, "extra-data")

	// Verify --set-values takes precedence over both package defaults and --values file
	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-values-configmap", "-o", "jsonpath='{.data.cliOverride}'")
	require.NoError(t, err, "unable to get configmap")
	require.Contains(t, kubectlOut, "cli-wins", "--set-values should override both package defaults and --values file")

	// Verify the action configmap was templated with the action-set values
	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-action-configmap", "-o", "jsonpath='{.data.json}'")
	require.NoError(t, err, "unable to get action configmap")
	require.Contains(t, kubectlOut, "myValue")
	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-action-configmap", "-o", "jsonpath='{.data.yaml}'")
	require.NoError(t, err, "unable to get action configmap")
	require.Contains(t, kubectlOut, "myValue")

	// Verify the raw template configmap was NOT processed by Zarf (default behavior without template: true)
	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-raw-template-configmap", "-o", "jsonpath='{.data.rawTemplate}'")
	require.NoError(t, err, "unable to get raw template configmap")
	require.Contains(t, kubectlOut, "template={{ .shouldNotBeProcessed }}")

	// Verify the processed template configmap WAS processed by Zarf (template: true)
	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-processed-template-configmap", "-o", "jsonpath='{.data.processedTemplate}'")
	require.NoError(t, err, "unable to get processed template configmap")
	require.Contains(t, kubectlOut, "processed=myValue")

	// Verify public state fields were templated into the configmap (no stateAccess declaration needed)
	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-state-configmap", "-o", "jsonpath='{.data.registryAddress}'")
	require.NoError(t, err, "unable to get state configmap")
	require.NotContains(t, kubectlOut, "{{", "registryAddress should have been templated")
	require.NotEmpty(t, kubectlOut, "registryAddress should be non-empty")

	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-state-configmap", "-o", "jsonpath='{.data.storageClass}'")
	require.NoError(t, err, "unable to get state configmap")
	require.NotContains(t, kubectlOut, "{{", "storageClass should have been templated")

	// Verify templatedValuesFiles rendered .Values.* into chart values at deploy time.
	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-values-chart-configmap", "-o", "jsonpath='{.data.staticKey}'")
	require.NoError(t, err, "unable to get chart configmap")
	require.Contains(t, kubectlOut, "static-from-file", "staticKey should come from valuesFiles unchanged")

	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-values-chart-configmap", "-o", "jsonpath='{.data.chartImageTag}'")
	require.NoError(t, err, "unable to get chart configmap")
	require.Contains(t, kubectlOut, "override-tag", "chartImageTag should be rendered from .Values.imageTag in templatedValuesFiles")
	require.NotContains(t, kubectlOut, "{{", "Go template syntax should have been resolved in templatedValuesFiles")

	// Verify the chart value mapping carried the non-excluded sibling through.
	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-values-chart-configmap", "-o", "jsonpath='{.data.mappedIncluded}'")
	require.NoError(t, err, "unable to get chart configmap")
	require.Contains(t, kubectlOut, "mapped-source-value", "mappedIncluded should be mapped from .chartMapping.included")

	// Verify excludePaths dropped a value supplied at deploy time: override-values.yaml
	// sets chartMapping.excluded, but the mapping excludes it, so the chart keeps its default.
	kubectlOut, _, err = e2e.Kubectl(t, "get", "configmap", "test-values-chart-configmap", "-o", "jsonpath='{.data.mappedExcluded}'")
	require.NoError(t, err, "unable to get chart configmap")
	require.NotContains(t, kubectlOut, "should-not-appear", "excludePaths should drop the deploy-time value before it reaches the chart")
	require.Contains(t, kubectlOut, "chart-default", "mappedExcluded should retain the chart default since the source was excluded")

	// Remove the package with values
	valuesFile := filepath.Join(src, "override-values.yaml")
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "test-values", "--confirm", "--features=\"values=true\"", "--values", valuesFile, "--set-values", "removeKey=custom-remove-value", "--skip-version-check")
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
		_, stdErr, err := e2e.Zarf(t, "package", "create", src, "-o", tmpdir, "--skip-sbom", "--confirm", "--features=\"values=true\"")
		require.Error(t, err, "expected error for invalid values")
		// Check that the error message mentions validation failure
		require.Contains(t, stdErr, "values validation failed", "error should mention schema validation failure")
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

	t.Run("merged schema enforces parent constraint on imported child property", func(t *testing.T) {
		src := filepath.Join("src", "test", "packages", "42-values", "schema-merge")
		tmpdir := t.TempDir()

		// Create should succeed: parent values (namespace, replicas:3) satisfy both schemas.
		// The child's values.yaml (appName, version, replicas, enabled) are merged in from the import.
		stdOut, stdErr, err := e2e.Zarf(t, "package", "create", src, "-o", tmpdir, "--skip-sbom", "--confirm", "--features=\"values=true\"")
		require.NoError(t, err, stdOut, stdErr)

		packageName := fmt.Sprintf("zarf-package-test-values-schema-merge-%s.tar.zst", e2e.Arch)
		path := filepath.Join(tmpdir, packageName)

		// Deploy with replicas:7 should fail. The parent schema caps replicas at 5, overriding
		// the child schema's maximum of 10. A failure here confirms parent-wins on property
		// constraints in the assembled merged schema.
		overridePath := filepath.Join(src, "override-invalid.yaml")
		_, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm", "--features=\"values=true\"", "--values", overridePath)
		require.Error(t, err, "expected error: replicas:7 violates parent schema's maximum of 5")
		require.Contains(t, stdErr, "values validation failed")
	})
}

func TestValuesPreflight(t *testing.T) {
	t.Log("E2E: Values preflight validation")

	src := filepath.Join("src", "test", "packages", "42-values", "preflight-invalid")
	tmpdir := t.TempDir()

	// Create succeeds; the undefined reference is only detectable at deploy time.
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", src, "-o", tmpdir, "--skip-sbom", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	packageName := fmt.Sprintf("zarf-package-test-values-preflight-%s.tar.zst", e2e.Arch)
	path := filepath.Join(tmpdir, packageName)

	// Deploy must fail before any component is applied because a templated action references an
	// undefined value that no setValues action can provide.
	_, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
	require.Error(t, err, "deploy should fail early on an undefined templated value")
	require.Contains(t, stdErr, "doesNotExist", "error should identify the undefined value")

	// The manifest in the same component must not have been applied.
	_, _, err = e2e.Kubectl(t, "get", "configmap", "test-preflight-configmap")
	require.Error(t, err, "configmap should not exist because deploy failed before applying manifests")
}

func TestStateTemplates(t *testing.T) {
	t.Log("E2E: State template access controls")

	t.Run("sensitive fields are blocked without stateAccess", func(t *testing.T) {
		src := filepath.Join("src", "test", "packages", "42-values", "state-blocked")
		tmpdir := t.TempDir()

		stdOut, stdErr, err := e2e.Zarf(t, "package", "create", src, "-o", tmpdir, "--skip-sbom", "--confirm")
		require.NoError(t, err, stdOut, stdErr)

		packageName := fmt.Sprintf("zarf-package-test-state-blocked-%s.tar.zst", e2e.Arch)
		path := filepath.Join(tmpdir, packageName)

		_, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
		require.Error(t, err, "deploy should fail when accessing sensitive state without stateAccess")
		require.Contains(t, stdErr, "PushPassword", "error should identify the inaccessible field")
	})
}
