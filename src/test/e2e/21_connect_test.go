// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/stretchr/testify/require"
)

type RegistryResponse struct {
	Repositories []string `json:"repositories"`
}

func TestConnect(t *testing.T) {
	t.Log("E2E: Connect")
	e2e.SetupWithCluster(t)

	// Make the Registry contains the images we expect
	stdOut, stdErr, err := e2e.Zarf("tools", "registry", "catalog")
	require.NoError(t, err, stdOut, stdErr)
	registryList := strings.Split(strings.Trim(stdOut, "\n "), "\n")

	// We assert greater than or equal to since the base init has 12 images
	// HOWEVER during an upgrade we could have mismatched versions/names resulting in more images
	require.GreaterOrEqual(t, len(registryList), 7)
	require.Contains(t, stdOut, "gitea/gitea")

	// Connect to Gitea
	tunnelGit, err := cluster.NewZarfTunnel()
	require.NoError(t, err)
	err = tunnelGit.Connect(cluster.ZarfGit, false)
	require.NoError(t, err)
	defer tunnelGit.Close()

	// Make sure Gitea comes up cleanly
	respGit, err := http.Get(tunnelGit.HTTPEndpoint())
	require.NoError(t, err)
	require.Equal(t, 200, respGit.StatusCode)

	// Connect to the Logging Stack
	tunnelLog, err := cluster.NewZarfTunnel()
	require.NoError(t, err)
	err = tunnelLog.Connect(cluster.ZarfLogging, false)
	require.NoError(t, err)
	defer tunnelLog.Close()

	// Make sure Grafana comes up cleanly
	respLog, err := http.Get(tunnelLog.HTTPEndpoint())
	require.NoError(t, err)
	require.Equal(t, 200, respLog.StatusCode)

	stdOut, stdErr, err = e2e.Zarf("package", "remove", "init", "--components=logging", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf("tools", "registry", "prune", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf("tools", "registry", "catalog")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "gitea/gitea")
	require.NotContains(t, stdOut, "grafana/promtail")
	require.NotContains(t, stdOut, "grafana/grafana")
	require.NotContains(t, stdOut, "grafana/loki")
}

func TestMetrics(t *testing.T) {
	t.Log("E2E: Emits metrics")
	e2e.SetupWithCluster(t)

	tunnel, err := cluster.NewTunnel("zarf", "svc", "agent-hook", 8888, 8443)

	require.NoError(t, err)
	err = tunnel.Connect("", false)
	require.NoError(t, err)
	defer tunnel.Close()

	// Skip certificate verification
	// this is an https endpoint being accessed through port-forwarding
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Transport: tr}
	httpsEndpoint := strings.ReplaceAll(tunnel.HTTPEndpoint(), "http", "https")
	resp, err := client.Get(httpsEndpoint + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	desiredString := "go_gc_duration_seconds_count"
	require.Equal(t, true, strings.Contains(string(body), desiredString))
	require.NoError(t, err, resp)
	require.Equal(t, 200, resp.StatusCode)

}
