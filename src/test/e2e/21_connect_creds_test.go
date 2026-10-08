// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/test"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage"
	"helm.sh/helm/v4/pkg/storage/driver"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
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

	connectToZarfServices(ctx, t)
}

// FIXME: not super happy with this test
func TestGitCredsLegacyHTTP(t *testing.T) {
	ctx := t.Context()
	c, err := cluster.New(ctx)
	require.NoError(t, err)
	s, err := c.LoadState(ctx)
	require.NoError(t, err)
	require.True(t, s.GitServer.IsInternal())
	require.False(t, s.GitServer.TLSMode.Enabled())

	// Model an older Helm release without Git TLS values.
	releases := storage.Init(driver.NewSecrets(c.Clientset.CoreV1().Secrets(state.ZarfNamespaceName)))
	storedRelease, err := releases.Last("zarf-gitea")
	require.NoError(t, err)
	giteaRelease, ok := storedRelease.(*releasev1.Release)
	require.True(t, ok)
	originalValues := giteaRelease.Config
	giteaRelease.Config = maps.Clone(originalValues)
	giteaRelease.Config["extraVolumes"] = []any{}
	t.Cleanup(func() {
		giteaRelease.Config = originalValues
		require.NoError(t, releases.Update(giteaRelease))
	})
	require.NoError(t, releases.Update(giteaRelease))

	for _, tc := range []struct {
		name       string
		mode       state.GitTLSMode
		management state.GitTLSCertManagement
	}{
		{name: "legacy HTTP mode"},
		{name: "HTTP ignores certificate ownership", mode: state.GitTLSDisabled, management: state.GitTLSCertZarfManaged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := c.LoadState(ctx)
			require.NoError(t, err)
			s.GitServer.TLSMode = tc.mode
			s.GitServer.TLSCertManagement = tc.management
			require.NoError(t, c.SaveState(ctx, s))

			stdOut, stdErr, err := e2e.Zarf(t, "tools", "update-creds", "git", "--git-tls-mode=disabled", "--confirm")
			require.NoError(t, err, stdOut, stdErr)
			updated, err := c.LoadState(ctx)
			require.NoError(t, err)
			require.Equal(t, state.GitTLSDisabled, updated.GitServer.TLSMode)
			require.Empty(t, updated.GitServer.TLSCertManagement)
			require.NotEqual(t, s.GitServer.PushPassword, updated.GitServer.PushPassword)

			tunnel, err := c.Connect(ctx, cluster.ZarfGit)
			require.NoError(t, err)
			t.Cleanup(tunnel.Close)
			endpoints := tunnel.URLEndpoints()
			require.Len(t, endpoints, 1)
			require.Regexp(t, "^http://", endpoints[0])
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoints[0]+"/api/v1/user", nil)
			require.NoError(t, err)
			request.SetBasicAuth(updated.GitServer.PushUsername, updated.GitServer.PushPassword)
			client := &http.Client{Timeout: 10 * time.Second}
			response, err := client.Do(request)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, response.Body.Close()) })
			require.Equal(t, http.StatusOK, response.StatusCode)
		})
	}
}

func TestGitTLSModeSwitch(t *testing.T) {
	ctx := t.Context()
	c, err := cluster.New(ctx)
	require.NoError(t, err)
	for _, mode := range []state.GitTLSMode{state.GitTLSDisabled, state.GitTLSEnabled, state.GitTLSDisabled} {
		stdOut, stdErr, err := e2e.Zarf(t, "tools", "update-creds", "git", "--git-tls-mode="+string(mode), "--features=git-server-tls=true", "--confirm")
		require.NoError(t, err, stdOut, stdErr)

		deployments, err := c.Clientset.AppsV1().Deployments(state.ZarfNamespaceName).List(ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/instance=zarf-gitea",
		})
		require.NoError(t, err)
		require.Len(t, deployments.Items, 1)
		foundTLSVolume := false
		for _, volume := range deployments.Items[0].Spec.Template.Spec.Volumes {
			if volume.Secret == nil || volume.Secret.SecretName != state.GitServerTLSSecret {
				continue
			}
			foundTLSVolume = true
			require.NotNil(t, volume.Secret.Optional)
			require.Equal(t, !mode.Enabled(), *volume.Secret.Optional)
		}
		require.True(t, foundTLSVolume, "Gitea must mount the Git TLS secret")

		roots := x509.NewCertPool()
		if mode.Enabled() {
			certs, err := c.GetGitServerTLS(ctx)
			require.NoError(t, err)
			require.True(t, roots.AppendCertsFromPEM(certs.CA))
		} else {
			// HTTP must keep working when the unused TLS secret is absent.
			if err := c.Clientset.CoreV1().Secrets(state.ZarfNamespaceName).Delete(ctx, state.GitServerTLSSecret, metav1.DeleteOptions{}); !kerrors.IsNotFound(err) {
				require.NoError(t, err)
			}
			stdOut, stdErr, err = e2e.Kubectl(t, "rollout", "restart", "deployment/"+deployments.Items[0].Name, "-n", state.ZarfNamespaceName)
			require.NoError(t, err, stdOut, stdErr)
			stdOut, stdErr, err = e2e.Kubectl(t, "rollout", "status", "deployment/"+deployments.Items[0].Name, "-n", state.ZarfNamespaceName, "--timeout=2m")
			require.NoError(t, err, stdOut, stdErr)
		}

		tunnel, err := c.Connect(ctx, cluster.ZarfGit)
		require.NoError(t, err)
		t.Cleanup(tunnel.Close)
		endpoints := tunnel.URLEndpoints()
		require.Len(t, endpoints, 1)
		if mode.Enabled() {
			require.Regexp(t, "^https://", endpoints[0])
		} else {
			require.Regexp(t, "^http://", endpoints[0])
		}
		transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
		client := &http.Client{Timeout: 10 * time.Second, Transport: transport}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoints[0]+"/explore/repos", nil)
		require.NoError(t, err)
		response, err := client.Do(request)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.NoError(t, response.Body.Close())
		transport.CloseIdleConnections()
		tunnel.Close()
	}
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
