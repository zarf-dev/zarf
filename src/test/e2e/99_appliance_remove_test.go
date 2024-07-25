// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplianceRemove(t *testing.T) {
	t.Log("E2E: Appliance Remove")

	// Don't run this test in appliance mode
	if !e2e.ApplianceMode || e2e.ApplianceModeKeep {
		return
	}

	initPackageVersion := e2e.GetZarfVersion(t)

	path := fmt.Sprintf("build/zarf-init-%s-%s.tar.zst", e2e.Arch, initPackageVersion)

	// Package remove the cluster to test Zarf cleaning up after itself
	stdOut, stdErr, err := e2e.Zarf(t, "package", "remove", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Check that the cluster is now gone
	_, _, err = e2e.Kubectl(t, "get", "nodes")
	require.Error(t, err)

	// TODO (@WSTARR) - This needs to be refactored to use the remove logic instead of reaching into a magic directory
	// Re-init the cluster so that we can test if the destroy scripts run
	stdOut, stdErr, err = e2e.Zarf(t, "init", "--components=k3s", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Destroy the cluster to test Zarf cleaning up after itself
	stdOut, stdErr, err = e2e.Zarf(t, "destroy", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Check that the cluster gone again
	_, _, err = e2e.Kubectl(t, "get", "nodes")
	require.Error(t, err)
}
