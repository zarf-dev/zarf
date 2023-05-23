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

func TestMultiPartPackage(t *testing.T) {
	t.Log("E2E: Multi-part package")

	var (
		createPath = "src/test/packages/05-multi-part"
		deployPath = fmt.Sprintf("zarf-package-multi-part-%s.tar.zst.part000", e2e.Arch)
		outputFile = "multi-part-demo.dat"
	)

	e2e.CleanFiles(deployPath, outputFile)

	// Create the package with a max size of 1MB
	stdOut, stdErr, err := e2e.ZarfWithConfirm("package", "create", createPath, "--max-package-size=1")
	require.NoError(t, err, stdOut, stdErr)

	list, err := filepath.Glob("zarf-package-multi-part-*")
	require.NoError(t, err)
	// Length is 7 because there are 6 parts and 1 manifest
	require.Len(t, list, 7)

	stdOut, stdErr, err = e2e.ZarfWithConfirm("package", "deploy", deployPath)
	require.NoError(t, err, stdOut, stdErr)

	// Verify the package was deployed
	require.FileExists(t, outputFile)

	e2e.CleanFiles(deployPath, outputFile)
}
