package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComponentScripts(t *testing.T) {
	t.Log("E2E: Testing component scripts")
	e2e.setup(t)
	defer e2e.teardown(t)

	// Note these files will be created in the package directory, not CWD
	prepareArtifact := "examples/component-scripts/test-prepare.txt"
	deployArtifacts := []string{
		"test-deploy-before.txt",
		"test-deploy-after.txt",
	}
	allArtifacts := append(deployArtifacts, prepareArtifact)
	e2e.cleanFiles(allArtifacts...)
	defer e2e.cleanFiles(allArtifacts...)

	// Try creating the package to test the create scripts
	stdOut, stdErr, err := e2e.execZarfCommand("package", "create", "examples/component-scripts/", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Test for package create prepare artirfact
	require.FileExists(t, prepareArtifact)

	// Test to ensure the deploy scripts are not executed
	for _, artifact := range deployArtifacts {
		require.NoFileExists(t, artifact)
	}

	// Remove the prepare artifact before running package deploy
	e2e.cleanFiles(prepareArtifact)

	path := fmt.Sprintf("build/zarf-package-component-scripts-%s.tar.zst", e2e.arch)

	// Deploy the simple script that should pass
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", path, "--confirm", "--components=prepare,deploy")
	require.NoError(t, err, stdOut, stdErr)

	// Check that the deploy artifacts were created
	for _, artifact := range deployArtifacts {
		require.FileExists(t, artifact)
	}

	// Check that the prepare artifact were not created
	require.NoFileExists(t, prepareArtifact)

	// Deploy the simple script that should fail the timeout
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", path, "--confirm", "--components=timeout")
	require.Error(t, err, stdOut, stdErr)
}
