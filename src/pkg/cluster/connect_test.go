// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
)

const twoContextKubeconfig = `apiVersion: v1
kind: Config
current-context: alpha
clusters:
  - name: alpha-cluster
    cluster:
      server: https://alpha.example.com:6443
  - name: beta-cluster
    cluster:
      server: https://beta.example.com:6443
contexts:
  - name: alpha
    context:
      cluster: alpha-cluster
      user: alpha-user
  - name: beta
    context:
      cluster: beta-cluster
      user: beta-user
users:
  - name: alpha-user
    user:
      token: alpha-token
  - name: beta-user
    user:
      token: beta-token
`

func writeKubeconfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte(twoContextKubeconfig), 0o600))
	return path
}

func TestKubeconfigEnv(t *testing.T) {
	t.Parallel()

	kubeconfig := writeKubeconfig(t)

	t.Run("zero options select nothing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		value, overlay, err := KubeconfigEnv(ConnectOptions{}, dir)
		require.NoError(t, err)
		require.Empty(t, value)
		require.Empty(t, overlay)
		requireEmptyDir(t, dir)
	})

	t.Run("path only is passed through without writing anything", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		value, overlay, err := KubeconfigEnv(ConnectOptions{Kubeconfig: kubeconfig}, dir)
		require.NoError(t, err)
		require.Equal(t, kubeconfig, value)
		require.Empty(t, overlay)
		requireEmptyDir(t, dir)
	})

	t.Run("context becomes an overlay ahead of the real kubeconfig", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		value, overlay, err := KubeconfigEnv(ConnectOptions{Kubeconfig: kubeconfig, Context: "beta"}, dir)
		require.NoError(t, err)
		require.NotEmpty(t, overlay)
		require.Equal(t, strings.Join([]string{overlay, kubeconfig}, string(os.PathListSeparator)), value)

		// The overlay must carry the context and nothing else: it outlives the command when a
		// vendored tool exits on its own, so it must never hold credentials.
		contents, err := os.ReadFile(overlay)
		require.NoError(t, err)
		require.Contains(t, string(contents), `current-context: "beta"`)
		require.NotContains(t, string(contents), "token")
		require.NotContains(t, string(contents), "alpha-cluster")

		written, err := clientcmd.LoadFromFile(overlay)
		require.NoError(t, err)
		require.Equal(t, "beta", written.CurrentContext)
		require.Empty(t, written.AuthInfos)
		require.Empty(t, written.Clusters)
	})

	t.Run("unknown context is an error, not a silent fallback", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		_, _, err := KubeconfigEnv(ConnectOptions{Kubeconfig: kubeconfig, Context: "gamma"}, dir)
		require.ErrorContains(t, err, `context "gamma" does not exist`)
		requireEmptyDir(t, dir)
	})
}

// TestKubeconfigEnvWithoutExplicitPath is separate because it sets KUBECONFIG.
func TestKubeconfigEnvWithoutExplicitPath(t *testing.T) {
	kubeconfig := writeKubeconfig(t)
	dir := t.TempDir()
	t.Setenv("KUBECONFIG", kubeconfig)

	value, overlay, err := KubeconfigEnv(ConnectOptions{Context: "beta"}, dir)
	require.NoError(t, err)
	require.Equal(t, strings.Join([]string{overlay, kubeconfig}, string(os.PathListSeparator)), value)
}

func requireEmptyDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestClientAndConfigSelectsContext(t *testing.T) {
	// Not parallel: NewDefaultClientConfigLoadingRules reads KUBECONFIG from the environment.
	kubeconfig := writeKubeconfig(t)
	t.Setenv("KUBECONFIG", kubeconfig)

	_, cfg, err := ClientAndConfig()
	require.NoError(t, err)
	require.Equal(t, "https://alpha.example.com:6443", cfg.Host, "defaults to current-context")

	_, cfg, err = ClientAndConfig(ConnectOptions{Context: "beta"})
	require.NoError(t, err)
	require.Equal(t, "https://beta.example.com:6443", cfg.Host)

	t.Setenv("KUBECONFIG", "")
	_, cfg, err = ClientAndConfig(ConnectOptions{Kubeconfig: kubeconfig, Context: "beta"})
	require.NoError(t, err)
	require.Equal(t, "https://beta.example.com:6443", cfg.Host)
}
