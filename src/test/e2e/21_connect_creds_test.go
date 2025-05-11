// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/test"
)

type RegistryResponse struct {
	Repositories []string `json:"repositories"`
}

func TestConnectAndCreds(t *testing.T) {
	t.Log("E2E: Connect")
	ctx := logger.WithContext(t.Context(), test.GetLogger(t))

	prevAgentSecretData, _, err := e2e.Kubectl(t, "get", "secret", "agent-hook-tls", "-n", "zarf", "-o", "jsonpath={.data}")
	require.NoError(t, err)

	var prevData map[string]string
	require.NoError(t, json.Unmarshal([]byte(prevAgentSecretData), &prevData))

	c, err := cluster.New(ctx)
	require.NoError(t, err)
	// Init the state variable
	oldState, err := c.LoadState(ctx)
	require.NoError(t, err)

	connectToZarfServices(ctx, t)

	stdOut, stdErr, err := e2e.Zarf(t, "tools", "update-creds", "--confirm", "--log-format=console", "--no-color")
	require.NoError(t, err, stdOut, stdErr)

	newAgentSecretData, _, err := e2e.Kubectl(t, "get", "secret", "agent-hook-tls", "-n", "zarf", "-o", "jsonpath={.data}")
	require.NoError(t, err)

	var newData map[string]string
	require.NoError(t, json.Unmarshal([]byte(newAgentSecretData), &newData))

	newState, err := c.LoadState(ctx)
	require.NoError(t, err)

	require.NotEqual(t, prevData["tls.crt"], newData["tls.crt"])
	require.NotEqual(t, prevData["tls.key"], newData["tls.key"])
	require.NotEqual(t, oldState.ArtifactServer.PushToken, newState.ArtifactServer.PushToken)
	require.NotEqual(t, oldState.GitServer.PushPassword, newState.GitServer.PushPassword)

	connectToZarfServices(ctx, t)
}

func TestMetrics(t *testing.T) {
	t.Log("E2E: Emits metrics")

	c, err := cluster.New(t.Context())
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
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

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
	stdOut, stdErr, err := e2e.Zarf(t, "tools", "registry", "catalog")
	require.NoError(t, err, stdOut, stdErr)
	registryList := strings.Split(strings.Trim(stdOut, "\n "), "\n")

	// We assert greater than or equal to since the base init has 8 images
	// HOWEVER during an upgrade we could have mismatched versions/names resulting in more images
	require.GreaterOrEqual(t, len(registryList), 3)
	require.Contains(t, stdOut, "zarf-dev/zarf/agent")
	require.Contains(t, stdOut, "gitea/gitea")
	require.Contains(t, stdOut, "library/registry")

	// Get the git credentials
	stdOut, stdErr, err = e2e.Zarf(t, "tools", "get-creds", "git", "--log-format=console", "--no-color")
	require.NoError(t, err, stdOut, stdErr)
	gitPushPassword := strings.TrimSpace(stdOut)
	stdOut, stdErr, err = e2e.Zarf(t, "tools", "get-creds", "git-readonly", "--log-format=console", "--no-color")
	require.NoError(t, err, stdOut, stdErr)
	gitPullPassword := strings.TrimSpace(stdOut)
	stdOut, stdErr, err = e2e.Zarf(t, "tools", "get-creds", "artifact", "--log-format=console", "--no-color")
	require.NoError(t, err, stdOut, stdErr)
	gitArtifactToken := strings.TrimSpace(stdOut)

	// Connect to Gitea
	c, err := cluster.New(ctx)
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
