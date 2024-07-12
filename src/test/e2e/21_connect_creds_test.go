// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/stretchr/testify/require"
)

type RegistryResponse struct {
	Repositories []string `json:"repositories"`
}

func TestConnectAndCreds(t *testing.T) {
	t.Log("E2E: Connect")

	prevAgentSecretData, _, err := e2e.Kubectl("get", "secret", "agent-hook-tls", "-n", "zarf", "-o", "jsonpath={.data}")
	require.NoError(t, err)

	ctx := context.Background()

	connectToZarfServices(ctx, t)

	stdOut, stdErr, err := e2e.Zarf("tools", "update-creds", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	newAgentSecretData, _, err := e2e.Kubectl("get", "secret", "agent-hook-tls", "-n", "zarf", "-o", "jsonpath={.data}")
	require.NoError(t, err)
	require.NotEqual(t, prevAgentSecretData, newAgentSecretData, "agent secrets should not be the same")

	connectToZarfServices(ctx, t)
}

func TestMetrics(t *testing.T) {
	t.Log("E2E: Emits metrics")

	c, err := cluster.NewCluster()
	require.NoError(t, err)

	tunnel, err := c.NewTunnel("zarf", "svc", "agent-hook", "", 8888, 8443)
	require.NoError(t, err)
	_, err = tunnel.Connect(context.Background())
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	desiredString := "go_gc_duration_seconds_count"
	require.Contains(t, string(body), desiredString)
	require.NoError(t, err, resp)
	require.Equal(t, 200, resp.StatusCode)
}

func connectToZarfServices(ctx context.Context, t *testing.T) {
	// Make the Registry contains the images we expect
	stdOut, stdErr, err := e2e.Zarf("tools", "registry", "catalog")
	require.NoError(t, err, stdOut, stdErr)
	registryList := strings.Split(strings.Trim(stdOut, "\n "), "\n")

	// We assert greater than or equal to since the base init has 8 images
	// HOWEVER during an upgrade we could have mismatched versions/names resulting in more images
	require.GreaterOrEqual(t, len(registryList), 3)
	require.Contains(t, stdOut, "defenseunicorns/zarf/agent")
	require.Contains(t, stdOut, "gitea/gitea")
	require.Contains(t, stdOut, "library/registry")

	// Get the git credentials
	stdOut, stdErr, err = e2e.Zarf("tools", "get-creds", "git")
	require.NoError(t, err, stdOut, stdErr)
	gitPushPassword := strings.TrimSpace(stdOut)
	stdOut, stdErr, err = e2e.Zarf("tools", "get-creds", "git-readonly")
	require.NoError(t, err, stdOut, stdErr)
	gitPullPassword := strings.TrimSpace(stdOut)
	stdOut, stdErr, err = e2e.Zarf("tools", "get-creds", "artifact")
	require.NoError(t, err, stdOut, stdErr)
	gitArtifactToken := strings.TrimSpace(stdOut)

	// Connect to Gitea
	c, err := cluster.NewCluster()
	require.NoError(t, err)
	tunnelGit, err := c.Connect(ctx, cluster.ZarfGit)
	require.NoError(t, err)
	defer tunnelGit.Close()

	// Make sure Gitea comes up cleanly
	gitPushURL := fmt.Sprintf("http://zarf-git-user:%s@%s/api/v1/user", gitPushPassword, tunnelGit.Endpoint())
	respGit, err := http.Get(gitPushURL)
	require.NoError(t, err)
	require.Equal(t, 200, respGit.StatusCode)
	gitPullURL := fmt.Sprintf("http://zarf-git-read-user:%s@%s/api/v1/user", gitPullPassword, tunnelGit.Endpoint())
	respGit, err = http.Get(gitPullURL)
	require.NoError(t, err)
	require.Equal(t, 200, respGit.StatusCode)
	gitArtifactURL := fmt.Sprintf("http://zarf-git-user:%s@%s/api/v1/user", gitArtifactToken, tunnelGit.Endpoint())
	respGit, err = http.Get(gitArtifactURL)
	require.NoError(t, err)
	require.Equal(t, 200, respGit.StatusCode)
}
