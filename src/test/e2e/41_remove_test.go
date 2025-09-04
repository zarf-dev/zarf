// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
)

func TestRemovePackageComponents(t *testing.T) {
	t.Log("E2E: Remove test package")

	tmpdir := t.TempDir()
	testCreate := filepath.Join("src", "test", "packages", "41-remove-test")
	packageName := fmt.Sprintf("zarf-package-remove-test-%s.tar.zst", e2e.Arch)
	packagePath := filepath.Join(tmpdir, packageName)

	// Create the test package
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", testCreate, "-o", tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", packagePath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Remove the package and check that the other component is still in state
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "remove-test", "--components=first", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	c, err := cluster.New(t.Context())
	require.NoError(t, err)
	deployedPackage, err := c.GetDeployedPackage(t.Context(), "remove-test")
	require.NoError(t, err)
	require.Len(t, deployedPackage.DeployedComponents, 1)
	require.Equal(t, "second", deployedPackage.DeployedComponents[0].Name)

	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", packagePath, "--components=second", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// verify the package is no longer in state
	_, err = c.GetDeployedPackage(t.Context(), "remove-test")
	require.Error(t, err)
}
