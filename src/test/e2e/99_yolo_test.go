// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/stretchr/testify/require"
)

func TestYOLOMode(t *testing.T) {
	t.Log("E2E: YOLO Mode")

	// Don't run this test in appliance mode
	if e2e.ApplianceMode {
		return
	}

	// Destroy the cluster to test Zarf cleaning up after itself
	stdOut, stdErr, err := e2e.Zarf("destroy", "--confirm", "--remove-components")
	require.NoError(t, err, stdOut, stdErr)

	path := fmt.Sprintf("build/zarf-package-yolo-%s.tar.zst", e2e.Arch)

	// Deploy the YOLO package
	stdOut, stdErr, err = e2e.Zarf("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	c, err := cluster.NewCluster()
	require.NoError(t, err)
	tunnel, err := c.Connect(context.Background(), "doom")
	require.NoError(t, err)
	defer tunnel.Close()

	// Check that 'curl' returns something.
	resp, err := http.Get(tunnel.HTTPEndpoint())
	require.NoError(t, err, resp)
	require.Equal(t, 200, resp.StatusCode)

	stdOut, stdErr, err = e2e.Zarf("package", "remove", "yolo", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func TestDevDeploy(t *testing.T) {
	// Don't run this test in appliance mode
	if e2e.ApplianceMode {
		return
	}

	stdOut, stdErr, err := e2e.Zarf("dev", "deploy", "examples/dos-games")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf("package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
