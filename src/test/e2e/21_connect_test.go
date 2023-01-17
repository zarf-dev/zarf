// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
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
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	// Get the state from the cluster
	zarfState, err := cluster.NewClusterOrDie().LoadZarfState()
	require.NoError(t, err)

	// Connect to the Registry
	tunnelReg, err := cluster.NewZarfTunnel()
	require.NoError(t, err)
	tunnelReg.Connect(cluster.ZarfRegistry, false)
	defer tunnelReg.Close()

	// Make the Registry contains the images we expect
	reqReg, err := http.NewRequest("GET", tunnelReg.HTTPEndpoint()+"/v2/_catalog", nil)
	assert.NoError(t, err)

	authReg := zarfState.RegistryInfo.PullUsername + ":" + zarfState.RegistryInfo.PullPassword
	authRegB64 := base64.StdEncoding.EncodeToString([]byte(authReg))
	reqReg.Header.Add("Authorization", "Basic "+authRegB64)

	respReg, err := http.DefaultClient.Do(reqReg)
	assert.NoError(t, err)
	assert.Equal(t, 200, respReg.StatusCode)

	bodyReg, err := io.ReadAll(respReg.Body)
	defer respReg.Body.Close()
	assert.NoError(t, err)

	registries := RegistryResponse{}
	err = json.Unmarshal(bodyReg, &registries)
	assert.NoError(t, err)
	assert.Equal(t, 12, len(registries.Repositories))
	assert.Contains(t, registries.Repositories, "gitea/gitea")
	assert.Contains(t, registries.Repositories, "gitea/gitea-3431384023")

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

	stdOut, stdErr, err := e2e.execZarfCommand("package", "remove", "init", "--components=logging", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
