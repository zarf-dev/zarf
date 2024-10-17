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

func TestCustomInit(t *testing.T) {
	t.Log("E2E: Custom Init Package")

	buildPath := filepath.Join("src", "test", "packages", "35-custom-init-package")
	pkgName := fmt.Sprintf("zarf-init-%s-%s.tar.zst", e2e.Arch, e2e.GetZarfVersion(t))
	privateKeyFlag := "--signing-key=src/test/packages/zarf-test.prv-key"
	publicKeyFlag := "--key=src/test/packages/zarf-test.pub"

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", buildPath, privateKeyFlag, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.CleanFiles(t, pkgName)

	/* Test operations during package inspect */
	// Test that we can inspect the yaml of the package without the private key
	stdOut, stdErr, err = e2e.Zarf(t, "package", "inspect", pkgName, "--skip-signature-validation")
	require.NoError(t, err, stdOut, stdErr)

	// Test that we don't get an error when we remember to provide the public key
	stdOut, stdErr, err = e2e.Zarf(t, "package", "inspect", pkgName, publicKeyFlag)
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Verified OK")

	/* Test operations during package deploy */
	// Test that we get an error when trying to deploy a package without providing the public key
	stdOut, stdErr, err = e2e.Zarf(t, "init", "--confirm")
	require.Error(t, err, stdOut, stdErr)
	require.Contains(t, e2e.StripMessageFormatting(stdErr), "unable to load the package: package is signed but no key was provided - add a key with the --key flag or use the --skip-signature-validation flag and run the command again")

	/* Test operations during package deploy */
	// Test that we can deploy the package with the public key
	stdOut, stdErr, err = e2e.Zarf(t, "init", "--confirm", publicKeyFlag)
	require.NoError(t, err, stdOut, stdErr)
}
