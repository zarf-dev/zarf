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

func TestDosGames(t *testing.T) {
	t.Log("E2E: Dos games")
	e2e.SetupWithCluster(t)
	defer e2e.Teardown(t)

	path := fmt.Sprintf("build/zarf-package-dos-games-%s.tar.zst", e2e.Arch)

	// Deploy the game
	stdOut, stdErr, _, err := e2e.ExecZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	tunnel, err := cluster.NewZarfTunnel()
	require.NoError(t, err)
	tunnel.Connect("doom", false)
	defer tunnel.Close()

	// Check that 'curl' returns something.
	resp, err := http.Get(tunnel.HTTPEndpoint())
	assert.NoError(t, err, resp)
	assert.Equal(t, 200, resp.StatusCode)

	stdOut, stdErr, _, err = e2e.ExecZarfCommand("package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func TestRemoteManifests(t *testing.T) {
	t.Log("E2E: Remote Manifests")
	e2e.SetupWithCluster(t)
	defer e2e.Teardown(t)

	path := fmt.Sprintf("build/zarf-package-remote-manifests-%s-0.0.1.tar.zst", e2e.Arch)

	// Deploy the package
	stdOut, stdErr, _, err := e2e.ExecZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Remove the package
	stdOut, stdErr, _, err = e2e.ExecZarfCommand("package", "remove", "remote-manifests", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
