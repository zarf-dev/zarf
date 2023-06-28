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
	if !e2e.ApplianceMode {
		return
	}

	e2e.SetupWithCluster(t)

	initPackageVersion := e2e.GetZarfVersion(t)

	path := fmt.Sprintf("build/zarf-init-%s-%s.tar.zst", e2e.Arch, initPackageVersion)

	// Destroy the cluster to test Zarf cleaning up after itself
	stdOut, stdErr, err := e2e.Zarf("package", "remove", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Check that the cluster is now gone
	_, _, err = e2e.Kubectl("get", "nodes")
	require.Error(t, err)
}
