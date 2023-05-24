// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMismatchedArchitectures ensures that zarf produces an error
// when the package architecture doesn't match the target cluster architecture.
func TestMismatchedArchitectures(t *testing.T) {
	t.Log("E2E: Mismatched architectures")
	e2e.SetupWithCluster(t)

	var (
		mismatchedArch         = e2e.GetMismatchedArch()
		mismatchedGamesPackage = fmt.Sprintf("zarf-package-dos-games-%s.tar.zst", mismatchedArch)
		expectedErrorMessage   = fmt.Sprintf("this package architecture is %s", mismatchedArch)
	)

	// Build dos-games package with different arch than the cluster arch.
	stdOut, stdErr, err := e2e.Zarf("package", "create", "examples/dos-games/", "--architecture", mismatchedArch, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.CleanFiles(mismatchedGamesPackage)

	// Ensure zarf package deploy returns an error because of the mismatched architectures.
	_, stdErr, err = e2e.Zarf("package", "deploy", mismatchedGamesPackage, "--confirm")
	require.Error(t, err, stdErr)
	require.Contains(t, stdErr, expectedErrorMessage)
}
