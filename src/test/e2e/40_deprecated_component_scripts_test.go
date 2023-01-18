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
	t.Log("E2E: Testing component scripts")
	e2e.setup(t)
	defer e2e.teardown(t)

	// Note these files will be created in the package directory, not CWD
	prepareArtifact := "src/test/test-packages/deprecated-component-scripts/test-prepare.txt"
	deployArtifacts := []string{
		"test-deploy-before.txt",
		"test-deploy-after.txt",
	}
	allArtifacts := append(deployArtifacts, prepareArtifact)
	e2e.cleanFiles(allArtifacts...)
	defer e2e.cleanFiles(allArtifacts...)

	// Try creating the package to test the create scripts
	testPackageDirPath := "src/test/test-packages/deprecated-component-scripts"
	testPackagePath := fmt.Sprintf("%s/zarf-package-deprecated-component-scripts-%s.tar.zst", testPackageDirPath, e2e.arch)
	outputFlag := fmt.Sprintf("-o=%s", testPackageDirPath)
	stdOut, stdErr, err := e2e.execZarfCommand("package", "create", testPackageDirPath, outputFlag, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.cleanFiles(testPackagePath)

	// Test for package create prepare artifact
	require.FileExists(t, prepareArtifact)

	// Test to ensure the deploy scripts are not executed
	for _, artifact := range deployArtifacts {
		require.NoFileExists(t, artifact)
	}

	// Deploy the simple script that should pass
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", testPackagePath, "--confirm", "--components=deploy")
	require.NoError(t, err, stdOut, stdErr)

	// Check that the deploy artifacts were created
	for _, artifact := range deployArtifacts {
		require.FileExists(t, artifact)
	}

	// Deploy the simple script that should fail the timeout
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", testPackagePath, "--confirm", "--components=timeout")
	require.Error(t, err, stdOut, stdErr)
}
