// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"testing"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/stretchr/testify/require"
)

// TestMismatchedArchitectures ensures that zarf produces an error
// when the package architecture doesn't match the target cluster architecture.
func TestMismatchedArchitectures(t *testing.T) {
	t.Log("E2E: Zarf init with mismatched architectures")
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	// Determine what test runner architecture we're running on,
	// and set mismatchedArch to the opposite architecture.
	var mismatchedArch string
	if e2e.arch == "amd64" {
		mismatchedArch = "arm64"
	}
	if e2e.arch == "arm64" {
		mismatchedArch = "amd64"
	}

	var (
		deployPackagePath           = "src/test/test-packages/29-mistmatched-architectures-deploy/"
		initPackagePath             = "src/test/test-packages/29-mistmatched-architectures-init/"
		deployPackageName           = "mismatched-arch"
		initPackageVersion          = "UnknownVersion"
		mismatchedDeployPackageName = fmt.Sprintf("build/zarf-package-%s-%s.tar.zst", deployPackageName, mismatchedArch)
		mismatchedInitPackageName   = fmt.Sprintf("build/zarf-init-%s-%s.tar.zst", mismatchedArch, initPackageVersion)
		expectedErrorMessage        = fmt.Sprintf(lang.CmdPackageDeployValidateArchitectureErr, mismatchedArch, e2e.arch)
	)

	// Build init package with different arch than the cluster arch.
	stdOut, stdErr, err := e2e.execZarfCommand("package", "create", initPackagePath, "--architecture", mismatchedArch, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.cleanFiles(mismatchedInitPackageName)

	// Build deploy package with different arch than the cluster arch.
	stdOut, stdErr, err = e2e.execZarfCommand("package", "create", deployPackagePath, "--architecture", mismatchedArch, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.cleanFiles(mismatchedDeployPackageName)

	// Make sure zarf init returns an error because of the mismatched architectures.
	// We need to use the --architecture flag here to force zarf to find the package.
	_, stdErr, err = e2e.execZarfCommand("init", "--architecture", mismatchedArch, "--confirm")
	require.Error(t, err, stdErr)
	require.Contains(t, stdErr, expectedErrorMessage)

	// Make sure zarf package deploy returns an error because of the mismatched architectures.
	_, stdErr, err = e2e.execZarfCommand("package", "deploy", mismatchedDeployPackageName, "--confirm")
	require.Error(t, err, stdErr)
	require.Contains(t, stdErr, expectedErrorMessage)
}
