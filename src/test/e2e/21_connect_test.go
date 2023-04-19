// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type RegistryResponse struct {
	Repositories []string `json:"repositories"`
}

func TestConnect(t *testing.T) {
	t.Log("E2E: Connect")
	e2e.SetupWithCluster(t)
	defer e2e.Teardown(t)

	// Make the Registry contains the images we expect
	stdOut, stdErr, err := e2e.ExecZarfCommand("tools", "registry", "catalog")
	assert.NoError(t, err, stdOut, stdErr)
	registryList := strings.Split(strings.Trim(stdOut, "\n "), "\n")

	// We assert greater than or equal to since the base init has 12 images
	// HOWEVER during an upgrade we could have mismatched versions/names resulting in more images
	assert.GreaterOrEqual(t, len(registryList), 7)
	assert.Contains(t, stdOut, "gitea/gitea")

	// Connect to Gitea
	tunnelGit, err := cluster.NewZarfTunnel()
	require.NoError(t, err)
	tunnelGit.Connect(cluster.ZarfGit, false)
	defer tunnelGit.Close()

	// Make sure Gitea comes up cleanly
	respGit, err := http.Get(tunnelGit.HTTPEndpoint())
	assert.NoError(t, err)
	assert.Equal(t, 200, respGit.StatusCode)

	// Connect to the Logging Stack
	tunnelLog, err := cluster.NewZarfTunnel()
	require.NoError(t, err)
	tunnelLog.Connect(cluster.ZarfLogging, false)
	defer tunnelLog.Close()

	// Make sure Grafana comes up cleanly
	respLog, err := http.Get(tunnelLog.HTTPEndpoint())
	assert.NoError(t, err)
	assert.Equal(t, 200, respLog.StatusCode)

	stdOut, stdErr, err = e2e.ExecZarfCommand("package", "remove", "init", "--components=logging", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
