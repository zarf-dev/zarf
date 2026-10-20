// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"os"
	"path/filepath"
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

func TestWriteEffectiveKubeconfig(t *testing.T) {
	t.Parallel()

	kubeconfig := writeKubeconfig(t)

	t.Run("zero options select nothing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path, err := WriteEffectiveKubeconfig(ConnectOptions{}, dir)
		require.NoError(t, err)
		require.Empty(t, path)
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, entries, "no file should be written when nothing is selected")
	})

	t.Run("path only is passed through without copying credentials", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path, err := WriteEffectiveKubeconfig(ConnectOptions{Kubeconfig: kubeconfig}, dir)
		require.NoError(t, err)
		require.Equal(t, kubeconfig, path)
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, entries)
	})

	t.Run("context is materialized as current-context", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path, err := WriteEffectiveKubeconfig(ConnectOptions{Kubeconfig: kubeconfig, Context: "beta"}, dir)
		require.NoError(t, err)
		require.NotEqual(t, kubeconfig, path)

		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "the written kubeconfig holds credentials")

		written, err := clientcmd.LoadFromFile(path)
		require.NoError(t, err)
		require.Equal(t, "beta", written.CurrentContext)
	})

	t.Run("unknown context is an error, not a silent fallback", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		_, err := WriteEffectiveKubeconfig(ConnectOptions{Kubeconfig: kubeconfig, Context: "gamma"}, dir)
		require.ErrorContains(t, err, `context "gamma" does not exist`)
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, entries, "a failed resolution must not leave a kubeconfig behind")
	})
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
