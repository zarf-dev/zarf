// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComponentActions(t *testing.T) {
	t.Log("E2E: Testing component actions")
	e2e.setup(t)
	defer e2e.teardown(t)

	// Note these files will be created in the package directory, not CWD.
	createArtifact := "examples/component-actions/test-create.txt"
	deployArtifacts := []string{
		"test-deploy-before.txt",
		"test-deploy-after.txt",
	}
	allArtifacts := append(deployArtifacts, createArtifact)
	e2e.cleanFiles(allArtifacts...)
	defer e2e.cleanFiles(allArtifacts...)

	// Try creating the package to test the onCreate actions.
	stdOut, stdErr, err := e2e.execZarfCommand("package", "create", "examples/component-actions", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Test for package create prepare artifact.
	require.FileExists(t, createArtifact)

	// Test to ensure the deploy scripts are not executed.
	for _, artifact := range deployArtifacts {
		require.NoFileExists(t, artifact)
	}

	path := fmt.Sprintf("build/zarf-package-component-actions-%s.tar.zst", e2e.arch)

	// Deploy the simple script that should pass.
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", path, "--confirm", "--components=on-deploy")
	require.NoError(t, err, stdOut, stdErr)

	// Check that the deploy artifacts were created.
	for _, artifact := range deployArtifacts {
		require.FileExists(t, artifact)
	}

	// Deploy the simple action that should fail the timeout.
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", path, "--confirm", "--components=on-deploy-with-timeout")
	require.Error(t, err, stdOut, stdErr)
}
