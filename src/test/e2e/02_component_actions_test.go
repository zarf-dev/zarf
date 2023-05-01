// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComponentActions(t *testing.T) {
	t.Log("E2E: Testing component actions")
	e2e.Setup(t)
	defer e2e.Teardown(t)

	// Note these files will be created in the package directory, not CWD.
	createArtifacts := []string{
		"examples/component-actions/test-create-before.txt",
		"examples/component-actions/test-create-after.txt",
	}
	deployArtifacts := []string{
		"test-deploy-before.txt",
		"test-deploy-after.txt",
	}
	deployWithEnvVarArtifact := "filename-from-env.txt"

	allArtifacts := append(deployArtifacts, createArtifacts...)
	allArtifacts = append(allArtifacts, deployWithEnvVarArtifact)
	allArtifacts = append(allArtifacts, "templated.txt")
	e2e.CleanFiles(allArtifacts...)
	defer e2e.CleanFiles(allArtifacts...)

	/* Create */
	// Try creating the package to test the onCreate actions.
	stdOut, stdErr, _, err := e2e.ExecZarfCommand("package", "create", "examples/component-actions", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Completed \"touch test-create-before.txt\"")
	require.Contains(t, stdErr, "multiline!")
	require.Contains(t, stdErr, "updates!")
	require.Contains(t, stdErr, "realtime!")
	require.Contains(t, stdErr, "Completed \"multiline & description demo\"")

	// Test for package create prepare artifacts.
	for _, artifact := range createArtifacts {
		require.FileExists(t, artifact)
	}

	// Test to ensure the deploy scripts are not executed.
	for _, artifact := range deployArtifacts {
		require.NoFileExists(t, artifact)
	}

	/* Deploy */
	path := fmt.Sprintf("build/zarf-package-component-actions-%s.tar.zst", e2e.Arch)
	// Deploy the simple script that should pass.
	stdOut, stdErr, _, err = e2e.ExecZarfCommand("package", "deploy", path, "--confirm", "--components=on-deploy-and-remove")
	require.NoError(t, err, stdOut, stdErr)

	// Check that the deploy artifacts were created.
	for _, artifact := range deployArtifacts {
		require.FileExists(t, artifact)
	}

	// Remove the simple script that should pass.
	stdOut, stdErr, _, err = e2e.ExecZarfCommand("package", "remove", path, "--confirm", "--components=on-deploy-and-remove")
	require.NoError(t, err, stdOut, stdErr)

	// Check that the deploy artifacts were created.
	for _, artifact := range deployArtifacts {
		require.NoFileExists(t, artifact)
	}

	// Deploy the simple action that should fail the timeout.
	stdOut, stdErr, _, err = e2e.ExecZarfCommand("package", "deploy", path, "--confirm", "--components=on-deploy-with-timeout")
	require.Error(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "😭😭😭 this action failed because it took too long to run 😭😭😭")

	// Test using a Zarf Variable within the action
	stdOut, stdErr, _, err = e2e.ExecZarfCommand("package", "deploy", path, "--confirm", "--components=on-deploy-with-variable", "-l=trace")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "the dog says ruff")

	// Test using dynamic and multiple-variables
	stdOut, stdErr, _, err = e2e.ExecZarfCommand("package", "deploy", path, "--confirm", "--components=on-deploy-with-dynamic-variable,on-deploy-with-multiple-variables", "-l=trace")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "the cat says meow")
	require.Contains(t, stdErr, "the dog says ruff")
	require.Contains(t, stdErr, "the snake says hiss")
	require.Contains(t, stdErr, "with a TF_VAR, the snake also says hiss")

	// Test using environment variables
	stdOut, stdErr, _, err = e2e.ExecZarfCommand("package", "deploy", path, "--confirm", "--components=on-deploy-with-env-var")
	require.NoError(t, err, stdOut, stdErr)
	require.FileExists(t, deployWithEnvVarArtifact)

	// Test using a templated file but without dynamic variables
	stdOut, stdErr, _, err = e2e.ExecZarfCommand("package", "deploy", path, "--confirm", "--components=on-deploy-with-template-use-of-variable")
	require.NoError(t, err, stdOut, stdErr)
	outTemplated, err := os.ReadFile("templated.txt")
	require.NoError(t, err)
	require.Contains(t, string(outTemplated), "The dog says ruff")
	require.Contains(t, string(outTemplated), "The cat says ###ZARF_VAR_CAT_SOUND###")
	require.Contains(t, string(outTemplated), "The snake says ###ZARF_VAR_SNAKE_SOUND###")

	// Remove the templated file so we can test with dynamic variables
	e2e.CleanFiles("templated.txt")

	// Test using a templated file with dynamic variables
	stdOut, stdErr, _, err = e2e.ExecZarfCommand("package", "deploy", path, "--confirm", "--components=on-deploy-with-template-use-of-variable,on-deploy-with-dynamic-variable,on-deploy-with-multiple-variables")
	require.NoError(t, err, stdOut, stdErr)
	outTemplated, err = os.ReadFile("templated.txt")
	require.NoError(t, err)
	require.Contains(t, string(outTemplated), "The dog says ruff")
	require.Contains(t, string(outTemplated), "The cat says meow")
	require.Contains(t, string(outTemplated), "The snake says hiss")
}
