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

	// Asking for removal of a component that doesn't exist should error
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "remove-test", "--components=unknown_component", "--confirm")
	require.Error(t, err, stdOut, stdErr)

	// Remove the component "first" and check that the other component is still in state
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

func TestRemoveFailedPackagedComponents(t *testing.T) {
	t.Log("E2E: Remove a failed package component")

	tmpdir := t.TempDir()
	testCreate := filepath.Join("src", "test", "packages", "41-remove-test", "fail")
	packageName := fmt.Sprintf("zarf-package-failing-deploy-remove-test-%s.tar.zst", e2e.Arch)
	packagePath := filepath.Join(tmpdir, packageName)

	// create a package where the component will fail to deploy
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", testCreate, "-o", tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// expect an error during deploy
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", packagePath, "--confirm", "--timeout", "20s", "--retries", "1")
	require.Error(t, err, stdOut, stdErr)

	// check state that the installedChart is deployed and recorded in state
	c, err := cluster.New(t.Context())
	require.NoError(t, err)
	deployedPackage, err := c.GetDeployedPackage(t.Context(), "failing-deploy-remove-test")
	require.NoError(t, err)
	require.Len(t, deployedPackage.DeployedComponents, 2)
	require.Equal(t, "second", deployedPackage.DeployedComponents[1].Name)
	require.Len(t, deployedPackage.DeployedComponents[1].InstalledCharts, 1)

	// remove the package by component
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "failing-deploy-remove-test", "--components=second", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// confirm component is removed
	deployedPackage, err = c.GetDeployedPackage(t.Context(), "failing-deploy-remove-test")
	require.NoError(t, err)
	require.Len(t, deployedPackage.DeployedComponents, 1)

	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", packagePath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// verify the package is no longer in state
	_, err = c.GetDeployedPackage(t.Context(), "failing-deploy-remove-test")
	require.Error(t, err)
}
