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
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	testPackageDirPath := "examples/dos-games"
	pkgName := fmt.Sprintf("zarf-package-dos-games-%s.tar.zst", e2e.arch)
	privateKeyFlag := "--key=src/test/test-packages/zarf-test.prv-key"
	publicKeyFlag := "--key=src/test/test-packages/zarf-test.pub"

	stdOut, stdErr, err := e2e.execZarfCommand("package", "create", testPackageDirPath, privateKeyFlag, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.cleanFiles(pkgName)

	/* Test operations during package inspect */
	// Test that we can inspect the yaml of the package without the private key
	stdOut, stdErr, err = e2e.execZarfCommand("package", "inspect", pkgName)
	require.NoError(t, err, stdOut, stdErr)

	// Test that we don't get an error when we remember to provide the public key
	stdOut, stdErr, err = e2e.execZarfCommand("package", "inspect", pkgName, publicKeyFlag)
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Verified OK")

	/* Test operations during package deploy */
	// Test that we get an error when trying to deploy a package without providing the public key
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", pkgName, "--confirm")
	require.Error(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Failed to deploy package: package is signed but no key was provided")

	// Test that we don't get an error when we remember to provide the public key
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", pkgName, publicKeyFlag, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Zarf deployment complete")
}
