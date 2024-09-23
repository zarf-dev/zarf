// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChecksumAndSignature(t *testing.T) {
	t.Log("E2E: Checksum and Signature")

	testPackageDirPath := "examples/dos-games"
	pkgName := fmt.Sprintf("zarf-package-dos-games-%s-1.1.0.tar.zst", e2e.Arch)
	privateKeyFlag := "--signing-key=src/test/packages/zarf-test.prv-key"
	publicKeyFlag := "--key=src/test/packages/zarf-test.pub"

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", testPackageDirPath, privateKeyFlag, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.CleanFiles(pkgName)

	// Test that we don't get an error when we remember to provide the public key
	stdOut, stdErr, err = e2e.Zarf(t, "package", "inspect", pkgName, publicKeyFlag)
	require.NoError(t, err, stdOut, stdErr)

	/* Test operations during package inspect */
	// Test that we can inspect the yaml of the package without the private key
	stdOut, stdErr, err = e2e.Zarf(t, "package", "inspect", pkgName, "--skip-signature-validation")
	require.NoError(t, err, stdOut, stdErr)

	/* Test operations during package deploy */
	// Test that we get an error when trying to deploy a package without providing the public key
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", pkgName, "--confirm")
	require.Error(t, err, stdOut, stdErr)
	require.Contains(t, e2e.StripMessageFormatting(stdErr), "failed to deploy package: unable to load the package: package is signed but no key was provided - add a key with the --key flag or use the --skip-signature-validation flag and run the command again")

	// Test that we don't get an error when we remember to provide the public key
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", pkgName, publicKeyFlag, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Zarf deployment complete")

	// Remove the package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", pkgName, publicKeyFlag, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
