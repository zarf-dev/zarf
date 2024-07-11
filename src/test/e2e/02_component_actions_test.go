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

	// Note these files will be created in the package directory, not CWD.
	createArtifacts := []string{
		"examples/component-actions/test-create-before.txt",
		"examples/component-actions/test-create-after.txt",
	}
	deployArtifacts := []string{
		"test-deploy-before.txt",
		"test-deploy-after.txt",
	}

	allArtifacts := append(deployArtifacts, createArtifacts...)
	t.Cleanup(func() { e2e.CleanFiles(allArtifacts...) })

	/* Create */
	// Try creating the package to test the onCreate actions.
	stdOut, stdErr, err := e2e.Zarf("package", "create", "examples/component-actions", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "multiline!")
	require.Contains(t, stdErr, "updates!")
	require.Contains(t, stdErr, "realtime!")

	// Test for package create prepare artifacts.
	for _, artifact := range createArtifacts {
		require.FileExists(t, artifact)
	}

	// Test to ensure the deploy scripts are not executed.
	for _, artifact := range deployArtifacts {
		require.NoFileExists(t, artifact)
	}

	path := fmt.Sprintf("build/zarf-package-component-actions-%s.tar.zst", e2e.Arch)
	t.Run("action on-deploy-and-remove", func(t *testing.T) {
		t.Parallel()

		// Deploy the simple script that should pass.
		stdOut, stdErr, err = e2e.Zarf("package", "deploy", path, "--components=on-deploy-and-remove", "--confirm")
		require.NoError(t, err, stdOut, stdErr)

		// Check that the deploy artifacts were created.
		for _, artifact := range deployArtifacts {
			require.FileExists(t, artifact)
		}

		// Remove the simple script that should pass.
		stdOut, stdErr, err = e2e.Zarf("package", "remove", path, "--components=on-deploy-and-remove", "--confirm")
		require.NoError(t, err, stdOut, stdErr)

		// Check that the deploy artifacts were removed.
		for _, artifact := range deployArtifacts {
			require.NoFileExists(t, artifact)
		}
	})

	t.Run("action on-deploy-with-variable", func(t *testing.T) {
		t.Parallel()

		// Test using a Zarf Variable within the action
		stdOut, stdErr, err = e2e.Zarf("package", "deploy", path, "--components=on-deploy-with-variable", "--confirm")
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdOut, "the dog says ruff")

	})

	t.Run("action on-deploy-with-dynamic-variable", func(t *testing.T) {
		t.Parallel()
		// Test using dynamic and multiple-variables
		stdOut, stdErr, err = e2e.Zarf("package", "deploy", path, "--components=on-deploy-with-dynamic-variable,on-deploy-with-multiple-variables", "--confirm")
		require.NoError(t, err, stdOut, stdErr)
		require.Equal(t, "meow\nthe cat says meow\nhiss\n", stdOut)
	})

	t.Run("action on-deploy-with-env-var", func(t *testing.T) {
		t.Parallel()
		deployWithEnvVarArtifact := "test-filename-from-env.txt"

		// Test using environment variables
		stdOut, stdErr, err = e2e.Zarf("package", "deploy", path, "--components=on-deploy-with-env-var", "--confirm")
		require.NoError(t, err, stdOut, stdErr)
		require.FileExists(t, deployWithEnvVarArtifact)

		// Remove the env var file at the end of the test
		e2e.CleanFiles(deployWithEnvVarArtifact)
	})

	t.Run("action on-deploy-with-template", func(t *testing.T) {
		t.Parallel()
		deployTemplatedArtifact := "test-templated.txt"

		// Test using a templated file but without dynamic variables
		stdOut, stdErr, err = e2e.Zarf("package", "deploy", path, "--components=on-deploy-with-template-use-of-variable", "--confirm")
		require.NoError(t, err, stdOut, stdErr)
		outTemplated, err := os.ReadFile(deployTemplatedArtifact)
		require.NoError(t, err)
		require.Contains(t, string(outTemplated), "The dog says ruff")
		require.Contains(t, string(outTemplated), "The cat says ###ZARF_VAR_CAT_SOUND###")
		require.Contains(t, string(outTemplated), "The snake says ###ZARF_VAR_SNAKE_SOUND###")

		// Remove the templated file so we can test with dynamic variables
		e2e.CleanFiles(deployTemplatedArtifact)

		// Test using a templated file with dynamic variables
		stdOut, stdErr, err = e2e.Zarf("package", "deploy", path, "--components=on-deploy-with-template-use-of-variable,on-deploy-with-dynamic-variable,on-deploy-with-multiple-variables", "--confirm")
		require.NoError(t, err, stdOut, stdErr)
		outTemplated, err = os.ReadFile(deployTemplatedArtifact)
		require.NoError(t, err)
		require.Contains(t, string(outTemplated), "The dog says ruff")
		require.Contains(t, string(outTemplated), "The cat says meow")
		require.Contains(t, string(outTemplated), "The snake says hiss")

		// Remove the templated file at the end of the test
		e2e.CleanFiles(deployTemplatedArtifact)
	})

	t.Run("action on-deploy-immediate-failure", func(t *testing.T) {
		t.Parallel()
		stdOut, stdErr, err = e2e.Zarf("package", "deploy", path, "--components=on-deploy-immediate-failure", "--confirm")
		require.Error(t, err, stdOut, stdErr)
		require.Contains(t, stdErr, "failed to deploy package")
		// regression test to ensure that failed commands are not erroneously flagged as a timeout
		require.NotContains(t, stdErr, "timed out")
	})
}
