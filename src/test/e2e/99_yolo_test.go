// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYOLOMode(t *testing.T) {
	t.Log("E2E: YOLO Mode")

	// Don't run this test in appliance mode
	if e2e.applianceMode {
		return
	}

	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	// Destroy the cluster to test Zarf cleaning up after itself
	stdOut, stdErr, err := e2e.execZarfCommand("destroy", "--confirm", "--remove-components")
	require.NoError(t, err, stdOut, stdErr)

	path := fmt.Sprintf("build/zarf-package-yolo-%s.tar.zst", e2e.arch)

	// Deploy the YOLO package
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	tunnel, err := cluster.NewZarfTunnel()
	require.NoError(t, err)
	tunnel.Connect("doom", false)
	defer tunnel.Close()

	// Check that 'curl' returns something.
	resp, err := http.Get(tunnel.HTTPEndpoint())
	assert.NoError(t, err, resp)
	assert.Equal(t, 200, resp.StatusCode)

	stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", "yolo", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
