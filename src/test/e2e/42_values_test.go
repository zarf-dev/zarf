// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
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
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm", "--features=\"values=true\"", "--values", overrideValuesPath, "--set-values", "cliOverride=cli-wins")
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
}

// This test does not require cluster access - can be run independently
func TestValuesImportNamespacing(t *testing.T) {
	t.Log("E2E: Values Import Namespacing")

	// Create a package that contains a component import
	// The child component uses .Values.app.name which gets namespaced to .Values.imported-app.app.name
	src := filepath.Join("src", "test", "packages", "42-values", "import-namespacing")
	tmpdir := t.TempDir()
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", src, "-o", tmpdir, "--skip-sbom", "--features=\"values=true\"")
	require.NoError(t, err, stdOut, stdErr)

	// Load the created package to inspect its contents
	packageName := fmt.Sprintf("zarf-package-test-values-import-namespacing-%s.tar.zst", e2e.Arch)
	tarPath := filepath.Join(tmpdir, packageName)
	pkgLayout, err := layout.LoadFromTar(t.Context(), tarPath, layout.PackageLayoutOptions{})
	require.NoError(t, err)

	// Verify the package has the imported component
	require.Len(t, pkgLayout.Pkg.Components, 1)
	comp := pkgLayout.Pkg.Components[0]
	require.Equal(t, "imported-app", comp.Name)

	// Verify action templates were namespaced
	// Original: {{ .Values.app.name }} -> Namespaced: {{ .Values.imported-app.app.name }}
	require.Len(t, comp.Actions.OnDeploy.Before, 2)
	require.Contains(t, comp.Actions.OnDeploy.Before[0].Cmd, ".Values.imported-app.app.name",
		"action cmd should have namespaced .Values.app.name to .Values.imported-app.app.name")
	require.Contains(t, comp.Actions.OnDeploy.Before[1].Cmd, ".Values.imported-app.app.environment",
		"action cmd should have namespaced .Values.app.environment to .Values.imported-app.app.environment")

	// Verify chart values sourcePath was namespaced
	// Original: .app.replicas -> Namespaced: .imported-app.app.replicas
	require.Len(t, comp.Charts, 1)
	require.Len(t, comp.Charts[0].Values, 3)
	require.Equal(t, ".imported-app.app.replicas", comp.Charts[0].Values[0].SourcePath,
		"chart values sourcePath should be namespaced")
	require.Equal(t, ".imported-app.config.setting", comp.Charts[0].Values[1].SourcePath,
		"chart values sourcePath should be namespaced")
	require.Equal(t, ".parent.app.replicas", comp.Charts[0].Values[2].SourcePath,
		"parent values sourcePath should not be namespaced and should be last in the list")

	// Read and unmarshal the merged values.yaml from the package
	valuesPath := filepath.Join(pkgLayout.DirPath(), "values.yaml")
	valuesContent, err := os.ReadFile(valuesPath)
	require.NoError(t, err, "should be able to read merged values.yaml")

	var values map[string]any
	err = goyaml.Unmarshal(valuesContent, &values)
	require.NoError(t, err, "should be able to unmarshal values.yaml")

	// Verify parent-level values are present
	parent, ok := values["parent"].(map[string]any)
	require.True(t, ok, "merged values should contain parent-level values")
	require.Equal(t, "parent-package-name", parent["name"], "parent.name should be set")
	require.Equal(t, "1.0.0", parent["version"], "parent.version should be set")

	// Verify child values were namespaced under component name and merged
	importedApp, ok := values["imported-app"].(map[string]any)
	require.True(t, ok, "merged values should contain namespaced child values under component name")

	app, ok := importedApp["app"].(map[string]any)
	require.True(t, ok, "imported-app should contain app values")
	// Parent's override value should take precedence over child defaults
	require.Equal(t, "production", app["environment"],
		"parent override should take precedence over child default")

	config, ok := importedApp["config"].(map[string]any)
	require.True(t, ok, "imported-app should contain config values")
	require.Equal(t, "parent-override-setting", config["setting"],
		"parent override should take precedence over child default")

	// Verify manifest file contents were namespaced
	// Extract the manifests directory from the component
	manifestsDir, err := pkgLayout.GetComponentDir(context.Background(), tmpdir, "imported-app", layout.ManifestsComponentDir)
	require.NoError(t, err, "should be able to extract manifests directory")

	// Read the configmap manifest file
	// The manifest name is "app-configmap" so the file is "app-configmap-0.yaml"
	manifestContent, err := os.ReadFile(filepath.Join(manifestsDir, "app-configmap-0.yaml"))
	require.NoError(t, err, "should be able to read manifest file")

	// Verify templates in the manifest were namespaced
	// Original: {{ .Values.app.name }} -> Namespaced: {{ .Values.imported-app.app.name }}
	require.Contains(t, string(manifestContent), ".Values.imported-app.app.name",
		"manifest template should have namespaced .Values.app.name")
	require.Contains(t, string(manifestContent), ".Values.imported-app.app.environment",
		"manifest template should have namespaced .Values.app.environment")
	require.Contains(t, string(manifestContent), ".Values.imported-app.config.setting",
		"manifest template should have namespaced .Values.config.setting")
}
