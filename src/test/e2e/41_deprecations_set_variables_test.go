// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDeprecatedSetAndPackageVariables verifies that deprecated setVariables and PKG_VARs still able to be set.
func TestDeprecatedSetAndPackageVariables(t *testing.T) {
	t.Log("E2E: Testing deprecated set variables")
	e2e.setup(t)
	defer e2e.teardown(t)

	// Note prepare script files will be created in the package directory, not CWD
	testPackageDirPath := "src/test/test-packages/41-deprecated-set-variable"
	prepareArtifact := fmt.Sprintf("%s/test-deprecated-prepare-hook.txt", testPackageDirPath)
	deployArtifacts := []string{
		"test-deprecated-deploy-before-hook.txt",
		"test-deprecated-deploy-after-hook.txt",
	}
	allArtifacts := append(deployArtifacts, prepareArtifact)
	e2e.cleanFiles(allArtifacts...)
	defer e2e.cleanFiles(allArtifacts...)

	// 2. Try creating the package to test the create scripts
	testPackagePath := fmt.Sprintf("%s/zarf-package-deprecated-set-variable-%s.tar.zst", testPackageDirPath, e2e.arch)
	outputFlag := fmt.Sprintf("-o=%s", testPackageDirPath)

	// Check that the command still errors out
	stdOut, stdErr, err := e2e.execZarfCommand("package", "create", testPackageDirPath, outputFlag, "--confirm")
	require.Error(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "template 'ECHO' must be '--set'")

	// Check that the command displays a warning on create
	stdOut, stdErr, err = e2e.execZarfCommand("package", "create", testPackageDirPath, outputFlag, "--confirm", "--set", "ECHO=Zarf-The-Axolotl")
	defer e2e.cleanFiles(testPackagePath)
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Component '1-test-deprecated-set-variable' is using setVariable")
	require.Contains(t, stdErr, "deprecated syntax ###ZARF_PKG_VAR_ECHO###")

	// 1. Deploy the setVariable action that should pass and output the variable
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", testPackagePath, "--confirm", "--components=1-test-deprecated-set-variable")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Hello from: Hello Kitteh")

	// 2. Deploy the setVariable action that should pass and output the variable
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", testPackagePath, "--confirm", "--components=2-test-deprecated-pkg-var")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Zarf-The-Axolotl")
}
