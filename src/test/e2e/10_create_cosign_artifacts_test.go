// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCosignArtifacts(t *testing.T) {
	t.Log("E2E: Cosign artifacts")

	var (
		createPath  = "src/test/packages/10-cosign-artifacts"
		packageName = fmt.Sprintf("zarf-package-cosign-artifacts-%s.tar.zst", e2e.Arch)
	)

	e2e.CleanFiles(packageName)

	// Create the package
	stdOut, stdErr, err := e2e.Zarf("package", "create", createPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Create the package a second time to validate caching does not cause errors
	stdOut, stdErr, err = e2e.Zarf("package", "create", createPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	e2e.CleanFiles(packageName)
}
