package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestE2eComponentChoice(t *testing.T) {
	defer e2e.cleanupAfterTest(t)

	path := fmt.Sprintf("../../../build/zarf-package-component-choice-%s.tar.zst", e2e.arch)

	// Try to deploy both and expect failure due to only one component allowed at a time
	// We currently don't have a pattern to actually test the interactive prompt, so just testing automation for now
	output, err := e2e.execZarfCommand("package", "deploy", path, "--confirm", "--components=first-choice,second-choice")
	require.Error(t, err, output)

	// Deploy a single choice and expect success
	output, err = e2e.execZarfCommand("package", "deploy", path, "--confirm", "--components=first-choice")
	require.NoError(t, err, output)

	// Verify the file was created
	expectedFile := "first-choice.txt"
	require.FileExists(t, expectedFile)
	e2e.filesToRemove = append(e2e.filesToRemove, expectedFile)
}
