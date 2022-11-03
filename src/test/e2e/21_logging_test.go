// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for zarf
package test

import (
	"net/http"
	"testing"

	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogging(t *testing.T) {
	t.Log("E2E: Logging")
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	tunnel := cluster.NewZarfTunnel()
	tunnel.Connect(cluster.ZarfLogging, false)
	defer tunnel.Close()

	// Make sure Grafana comes up cleanly
	resp, err := http.Get(tunnel.HttpEndpoint())
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	stdOut, stdErr, err := e2e.execZarfCommand("package", "remove", "init", "--components=logging", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
