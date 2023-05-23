// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComponentChoice(t *testing.T) {
	t.Log("E2E: Component choice")

	var (
		firstFile  = "first-choice-file.txt"
		secondFile = "second-choice-file.txt"
	)

	e2e.CleanFiles(firstFile, secondFile)

	path := fmt.Sprintf("build/zarf-package-component-choice-%s.tar.zst", e2e.Arch)

	// Try to deploy both and expect failure due to only one component allowed at a time
	// We currently don't have a pattern to actually test the interactive prompt, so just testing automation for now
	stdOut, stdErr, err := e2e.ZarfWithConfirm("package", "deploy", path, "--components=first-choice,second-choice")
	require.Error(t, err, stdOut, stdErr)

	// Deploy a single choice and expect success
	stdOut, stdErr, err = e2e.ZarfWithConfirm("package", "deploy", path, "--components=first-choice")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the file was created
	require.FileExists(t, firstFile)
	// Verify the second choice file was not created
	require.NoFileExists(t, secondFile)

	// Deploy using default choice
	stdOut, stdErr, err = e2e.ZarfWithConfirm("package", "deploy", path)
	require.NoError(t, err, stdOut, stdErr)

	// Verify the file was created
	require.FileExists(t, secondFile)

	t.Cleanup(func() {
		e2e.CleanFiles(firstFile, secondFile, path)
	})
}
