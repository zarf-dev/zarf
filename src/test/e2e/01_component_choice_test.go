package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComponentChoice(t *testing.T) {
	t.Log("E2E: Component choice")
	e2e.setup(t)
	defer e2e.teardown(t)

	var (
		firstFile  = "first-choice-file.txt"
		secondFile = "second-choice-file.txt"
	)

	e2e.cleanFiles(firstFile, secondFile)

	path := fmt.Sprintf("build/zarf-package-component-choice-%s.tar.zst", e2e.arch)

	// Try to deploy both and expect failure due to only one component allowed at a time
	// We currently don't have a pattern to actually test the interactive prompt, so just testing automation for now
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm", "--components=first-choice,second-choice")
	require.Error(t, err, stdOut, stdErr)

	// Deploy a single choice and expect success
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", path, "--confirm", "--components=first-choice")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the file was created
	require.FileExists(t, firstFile)
	// Verify the second choice file was not created
	require.NoFileExists(t, secondFile)

	// Deploy using default choice
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the file was created
	require.FileExists(t, secondFile)

	e2e.cleanFiles(firstFile, secondFile)
}
