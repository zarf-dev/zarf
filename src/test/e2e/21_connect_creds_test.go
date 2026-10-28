// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RegistryResponse struct {
	Repositories []string `json:"repositories"`
}

func TestConnectAndCreds(t *testing.T) {
	t.Log("E2E: Connect")
	ctx := logger.WithContext(t.Context(), test.GetLogger(t))

	prevAgentSecretData, _, err := e2e.Kubectl(t, "get", "secret", "agent-hook-tls", "-n", "zarf", "-o", "jsonpath={.data}")
	require.NoError(t, err)

	c, err := cluster.New(ctx)
	require.NoError(t, err)
	// Init the state variable
	oldState, err := c.LoadState(ctx)
	require.NoError(t, err)

	connectToZarfServices(ctx, t)

	stdOut, stdErr, err := e2e.Zarf(t, "tools", "update-creds", "registry", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	stdOut, stdErr, err = e2e.Zarf(t, "tools", "update-creds", "git", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	// Because we use kubectl scale the agent in an earlier command we force conflict
	stdOut, stdErr, err = e2e.Zarf(t, "tools", "update-creds", "agent", "--confirm", "--force-conflicts")
	require.NoError(t, err, stdOut, stdErr)
	// The artifact server is deprecated and only updatable through the legacy command.
	stdOut, stdErr, err = e2e.Zarf(t, "tools", "update-creds", "artifact", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	newAgentSecretData, _, err := e2e.Kubectl(t, "get", "secret", "agent-hook-tls", "-n", "zarf", "-o", "jsonpath={.data}")
	require.NoError(t, err)
	newState, err := c.LoadState(ctx)
	require.NoError(t, err)
	require.NotEqual(t, prevAgentSecretData, newAgentSecretData)
	require.NotEqual(t, oldState.ArtifactServer.PushToken, newState.ArtifactServer.PushToken)
	require.NotEqual(t, oldState.GitServer.PushPassword, newState.GitServer.PushPassword)
	verifyAgentTLS(t, c)

	connectToZarfServices(ctx, t)
}

func testAgentCredentialRotationCompatibility(t *testing.T) {
	c, err := cluster.New(t.Context())
	require.NoError(t, err)
	previousState, err := c.LoadState(t.Context())
	require.NoError(t, err)

	legacyCLI := e2e
	legacyCLI.ZarfBinPath = e2e.GetZarfAtVersion(t, "v0.88.0")
	for _, cli := range []test.ZarfE2ETest{e2e, legacyCLI} {
		stdOut, stdErr, err := cli.Zarf(t, "tools", "update-creds", "agent", "--confirm", "--force-conflicts")
		require.NoError(t, err, stdOut, stdErr)
		currentState, err := c.LoadState(t.Context())
		require.NoError(t, err)
		require.NotEqual(t, previousState.AgentInfo.TLS.Cert, currentState.AgentInfo.TLS.Cert)
		verifyAgentTLS(t, c)
		previousState = currentState
	}
}

func verifyAgentTLS(t *testing.T, c *cluster.Cluster) {
	t.Helper()
	ctx := t.Context()
	s, err := c.LoadState(ctx)
	require.NoError(t, err)
	agentTLS := s.AgentInfo.TLS
	secret, err := c.Clientset.CoreV1().Secrets(state.ZarfNamespaceName).Get(ctx, "agent-hook-tls", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, agentTLS.Cert, secret.Data["tls.crt"])
	require.Equal(t, agentTLS.Key, secret.Data["tls.key"])
	webhook, err := c.Clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, "zarf", metav1.GetOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, webhook.Webhooks)
	for _, hook := range webhook.Webhooks {
		require.Equal(t, agentTLS.CA, hook.ClientConfig.CABundle)
	}

	tunnel, err := c.NewTunnel(state.ZarfNamespaceName, "svc", "agent-hook", "", 0, 8443)
	require.NoError(t, err)
	_, err = tunnel.Connect(ctx)
	require.NoError(t, err)
	defer tunnel.Close()
	endpoints := tunnel.Endpoints()
	require.Len(t, endpoints, 1)
	caPool := x509.NewCertPool()
	require.True(t, caPool.AppendCertsFromPEM(agentTLS.CA))
	dialer := tls.Dialer{Config: &tls.Config{RootCAs: caPool, ServerName: state.ZarfAgentHost}}
	conn, err := dialer.DialContext(ctx, "tcp", endpoints[0])
	require.NoError(t, err, "agent must serve a certificate trusted by the CA in cluster state")
	defer func() { require.NoError(t, conn.Close()) }()
	tlsConn, ok := conn.(*tls.Conn)
	require.True(t, ok)
	cert, _ := pem.Decode(agentTLS.Cert)
	require.NotNil(t, cert)
	require.Equal(t, cert.Bytes, tlsConn.ConnectionState().PeerCertificates[0].Raw)
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
	// tunnel is create with the default listenAddress - there will only be one endpoint until otherwise supported
	endpoints := tunnel.HTTPEndpoints()
	require.Len(t, endpoints, 1)
	httpsEndpoint := strings.ReplaceAll(endpoints[0], "http", "https")
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

	// tunnel is create with the default listenAddress - there will only be one endpoint until otherwise supported
	endpoints := tunnelGit.Endpoints()
	require.Len(t, endpoints, 1)

	// Make sure Gitea comes up cleanly
	gitPushURL := fmt.Sprintf("http://zarf-git-user:%s@%s/api/v1/user", gitPushPassword, endpoints[0])
	respGit, err := http.Get(gitPushURL)
	require.NoError(t, err)
	require.Equal(t, 200, respGit.StatusCode)
	gitPullURL := fmt.Sprintf("http://zarf-git-read-user:%s@%s/api/v1/user", gitPullPassword, endpoints[0])
	respGit, err = http.Get(gitPullURL)
	require.NoError(t, err)
	require.Equal(t, 200, respGit.StatusCode)
	gitArtifactURL := fmt.Sprintf("http://zarf-git-user:%s@%s/api/v1/user", gitArtifactToken, endpoints[0])
	respGit, err = http.Get(gitArtifactURL)
	require.NoError(t, err)
	require.Equal(t, 200, respGit.StatusCode)
}
