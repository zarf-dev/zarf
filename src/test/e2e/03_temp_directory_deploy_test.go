// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTempDirectoryDeploy(t *testing.T) {
	t.Log("E2E: Temporary directory deploy")

	// run `zarf package deploy` with a specified tmp location
	var (
		otherTmpPath = filepath.Join(os.TempDir(), "othertmp")
		firstFile    = "first-choice-file.txt"
		secondFile   = "second-choice-file.txt"
	)

	e2e.Setup(t)
	defer e2e.Teardown(t)

	e2e.CleanFiles(otherTmpPath, firstFile, secondFile)

	path := fmt.Sprintf("build/zarf-package-component-choice-%s.tar.zst", e2e.Arch)

	_ = os.Mkdir(otherTmpPath, 0750)

	stdOut, stdErr, err := e2e.Zarf("package", "deploy", path, "--confirm", "--tmpdir", otherTmpPath, "--log-level=debug")
	require.Contains(t, stdErr, otherTmpPath, "The other tmp path should show as being created")
	require.NoError(t, err, stdOut, stdErr)

	e2e.CleanFiles(otherTmpPath, firstFile, secondFile)
}
