// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDeprecatedComponentScripts verifies that deprecated component scripts are still able to be executed (after being internally
// migrated into zarf actions).
func TestDeprecatedComponentScripts(t *testing.T) {
	t.Log("E2E: Testing deprecated component scripts")

	// Note these files will be created in the package directory, not CWD
	testPackageDirPath := "src/test/packages/03-deprecated-component-scripts"
	prepareArtifact := fmt.Sprintf("%s/test-deprecated-prepare-hook.txt", testPackageDirPath)
	deployArtifacts := []string{
		"test-deprecated-deploy-before-hook.txt",
		"test-deprecated-deploy-after-hook.txt",
	}
	allArtifacts := append(deployArtifacts, prepareArtifact)
	e2e.CleanFiles(t, allArtifacts...)
	defer e2e.CleanFiles(t, allArtifacts...)

	// 1. Try creating the package to test the create scripts
	testPackagePath := fmt.Sprintf("%s/zarf-package-deprecated-component-scripts-%s.tar.zst", testPackageDirPath, e2e.Arch)
	outputFlag := fmt.Sprintf("-o=%s", testPackageDirPath)
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", testPackageDirPath, outputFlag, "--confirm")
	defer e2e.CleanFiles(t, testPackagePath)
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Component '1-test-deprecated-prepare-scripts' is using scripts")
	require.Contains(t, stdErr, "Component '2-test-deprecated-deploy-scripts' is using scripts")
	require.Contains(t, stdErr, "Component '3-test-deprecated-timeout-scripts' is using scripts")

	// Test for package create prepare artifact
	require.FileExists(t, prepareArtifact)

	// Test to ensure the deploy scripts are not executed
	for _, artifact := range deployArtifacts {
		require.NoFileExists(t, artifact)
	}

	// 2. Deploy the simple script that should pass
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", testPackagePath, "--confirm", "--components=2-test-deprecated-deploy-scripts")
	require.NoError(t, err, stdOut, stdErr)

	// Check that the deploy artifacts were created
	for _, artifact := range deployArtifacts {
		require.FileExists(t, artifact)
	}

	// 3. Deploy the simple script that should fail the timeout
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", testPackagePath, "--confirm", "--components=3-test-deprecated-timeout-scripts")
	require.Error(t, err, stdOut, stdErr)
}

// TestDeprecatedSetAndPackageVariables verifies that deprecated setVariables and PKG_VARs still able to be set.
func TestDeprecatedSetAndPackageVariables(t *testing.T) {
	t.Log("E2E: Testing deprecated set variables")

	// Note prepare script files will be created in the package directory, not CWD
	testPackageDirPath := "src/test/packages/03-deprecated-set-variable"
	prepareArtifact := fmt.Sprintf("%s/test-deprecated-prepare-hook.txt", testPackageDirPath)
	deployArtifacts := []string{
		"test-deprecated-deploy-before-hook.txt",
		"test-deprecated-deploy-after-hook.txt",
	}
	allArtifacts := append(deployArtifacts, prepareArtifact)
	e2e.CleanFiles(t, allArtifacts...)
	defer e2e.CleanFiles(t, allArtifacts...)

	// 2. Try creating the package to test the create scripts
	testPackagePath := fmt.Sprintf("%s/zarf-package-deprecated-set-variable-%s.tar.zst", testPackageDirPath, e2e.Arch)
	outputFlag := fmt.Sprintf("-o=%s", testPackageDirPath)

	// Check that the command still errors out
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", testPackageDirPath, outputFlag, "--confirm")
	require.Error(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "template \"ECHO\" must be '--set'")

	// Check that the command displays a warning on create
	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", testPackageDirPath, outputFlag, "--confirm", "--set", "ECHO=Zarf-The-Axolotl")
	defer e2e.CleanFiles(t, testPackagePath)
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Component '1-test-deprecated-set-variable' is using setVariable")
	require.Contains(t, stdErr, "deprecated syntax ###ZARF_PKG_VAR_ECHO###")

	// 1. Deploy the setVariable action that should pass and output the variable
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", testPackagePath, "--confirm", "--components=1-test-deprecated-set-variable")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Hello from Hello Kitteh")

	// 2. Deploy the setVariable action that should pass and output the variable
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", testPackagePath, "--confirm", "--components=2-test-deprecated-pkg-var")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Zarf-The-Axolotl")
}
