// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComponentChoice(t *testing.T) {
	t.Log("E2E: Component choice")
	tmpdir := t.TempDir()
	t.Parallel()

	var (
		firstFile  = "first-choice-file.txt"
		secondFile = "second-choice-file.txt"
	)
	t.Cleanup(func() {
		e2e.CleanFiles(t, firstFile, secondFile)
	})

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "src/test/packages/01-component-choice", "-o", tmpdir)
	require.NoError(t, err, stdOut, stdErr)
	packageName := fmt.Sprintf("zarf-package-component-choice-%s.tar.zst", e2e.Arch)
	path := filepath.Join(tmpdir, packageName)

	// Try to deploy both and expect failure due to only one component allowed at a time
	// We currently don't have a pattern to actually test the interactive prompt, so just testing automation for now
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--components=first-choice,second-choice", "--confirm")
	require.Error(t, err, stdOut, stdErr)

	// Deploy a single choice and expect success
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--components=first-choice", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Component first-choice is using group which has been deprecated", "output should show a warning for group being deprecated.")

	// Verify the file was created
	require.FileExists(t, firstFile)
	// Verify the second choice file was not created
	require.NoFileExists(t, secondFile)

	// Deploy using default choice
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the file was created
	require.FileExists(t, secondFile)
}
