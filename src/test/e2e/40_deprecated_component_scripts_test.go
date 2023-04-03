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
	e2e.setup(t)
	defer e2e.teardown(t)

	// Note these files will be created in the package directory, not CWD
	testPackageDirPath := "src/test/test-packages/40-deprecated-component-scripts"
	prepareArtifact := fmt.Sprintf("%s/test-deprecated-prepare-hook.txt", testPackageDirPath)
	deployArtifacts := []string{
		"test-deprecated-deploy-before-hook.txt",
		"test-deprecated-deploy-after-hook.txt",
	}
	allArtifacts := append(deployArtifacts, prepareArtifact)
	e2e.cleanFiles(allArtifacts...)
	defer e2e.cleanFiles(allArtifacts...)

	// 1. Try creating the package to test the create scripts
	testPackagePath := fmt.Sprintf("%s/zarf-package-deprecated-component-scripts-%s.tar.zst", testPackageDirPath, e2e.arch)
	outputFlag := fmt.Sprintf("-o=%s", testPackageDirPath)
	stdOut, stdErr, err := e2e.execZarfCommand("package", "create", testPackageDirPath, outputFlag, "--confirm")
	defer e2e.cleanFiles(testPackagePath)
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
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", testPackagePath, "--confirm", "--components=2-test-deprecated-deploy-scripts")
	require.NoError(t, err, stdOut, stdErr)

	// Check that the deploy artifacts were created
	for _, artifact := range deployArtifacts {
		require.FileExists(t, artifact)
	}

	// 3. Deploy the simple script that should fail the timeout
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", testPackagePath, "--confirm", "--components=3-test-deprecated-timeout-scripts")
	require.Error(t, err, stdOut, stdErr)
}
