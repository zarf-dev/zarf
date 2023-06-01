// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMismatchedArchitectures ensures that zarf produces an error
// when the package architecture doesn't match the target cluster architecture.
func TestMismatchedArchitectures(t *testing.T) {
	t.Log("E2E: Mismatched architectures")
	e2e.SetupWithCluster(t)

	t.Run("package deploy mismatched arch", func(t *testing.T) {
		t.Parallel()
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
	})

	t.Run("init mismatched arch", func(t *testing.T) {
		t.Parallel()
		// Get the version of the CLI
		stdOut, stdErr, err := e2e.Zarf("version")
		require.NoError(t, err, stdOut, stdErr)
		initPackageVersion := strings.Trim(stdOut, "\n")

		var (
			mismatchedArch        = e2e.GetMismatchedArch()
			mismatchedInitPackage = fmt.Sprintf("zarf-init-%s-%s.tar.zst", mismatchedArch, initPackageVersion)
			expectedErrorMessage  = fmt.Sprintf("this package architecture is %s", mismatchedArch)
		)
		t.Cleanup(func() {
			e2e.CleanFiles(mismatchedInitPackage)
		})

		// Build init package with different arch than the cluster arch.
		stdOut, stdErr, err = e2e.Zarf("package", "create", "src/test/packages/29-mismatched-arch-init", "--architecture", mismatchedArch, "--confirm")
		require.NoError(t, err, stdOut, stdErr)
		// Check that `zarf init` fails in appliance mode when we try to initialize a k3s cluster
		// on a machine with a different architecture than the package architecture.
		// We need to use the --architecture flag here to force zarf to find the package.
		componentsFlag := ""
		if e2e.ApplianceMode {
			componentsFlag = "--components=k3s"
		}
		_, stdErr, err = e2e.Zarf("init", "--architecture", mismatchedArch, componentsFlag, "--confirm")
		require.Error(t, err, stdErr)
		require.Contains(t, stdErr, expectedErrorMessage)
	})
}
