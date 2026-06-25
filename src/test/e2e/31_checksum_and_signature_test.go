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

func TestChecksumAndSignature(t *testing.T) {
	t.Log("E2E: Checksum and Signature")

	testPackageDirPath := "examples/dos-games"
	pkgName := fmt.Sprintf("zarf-package-dos-games-%s-1.3.0.tar.zst", e2e.Arch)
	privateKeyFlag := "--signing-key=src/test/packages/zarf-test.prv-key"
	publicKeyFlag := "--key=src/test/packages/zarf-test.pub"

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", testPackageDirPath, privateKeyFlag, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.CleanFiles(t, pkgName)

	// Capture the tarball digest before deploy so we can verify the cluster stores it correctly.
	stdOut, stdErr, err = e2e.Zarf(t, "package", "inspect", "digest", pkgName)
	require.NoError(t, err, stdOut, stdErr)
	tarballDigest := strings.TrimSpace(stdOut)
	require.True(t, strings.HasPrefix(tarballDigest, "sha256:"), "digest should start with sha256:")

	// Test that we don't get an error when we remember to provide the public key
	stdOut, stdErr, err = e2e.Zarf(t, "package", "inspect", "definition", pkgName, publicKeyFlag)
	require.NoError(t, err, stdOut, stdErr)

	/* Test operations during package inspect */
	// Test that we can inspect the yaml of the package without the private key
	stdOut, stdErr, err = e2e.Zarf(t, "package", "inspect", "definition", pkgName)
	require.NoError(t, err, stdOut, stdErr)

	/* Test operations during package deploy */
	// Test that we get an error when trying to deploy a package without providing the public key
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", pkgName, "--verify", "--confirm")
	require.Error(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "package was signed with a key; provide --key to verify")

	// Test that we don't get an error when we remember to provide the public key
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", pkgName, publicKeyFlag, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the cluster stored the same digest that was computed from the tarball.
	stdOut, stdErr, err = e2e.Zarf(t, "package", "inspect", "digest", "dos-games")
	require.NoError(t, err, stdOut, stdErr)
	clusterDigest := strings.TrimSpace(stdOut)
	require.Equal(t, tarballDigest, clusterDigest,
		"cluster-stored digest should match the digest computed locally from the tarball before deploy")

	// Remove the package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", pkgName, publicKeyFlag, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
