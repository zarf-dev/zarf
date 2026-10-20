// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
)

const testKubeconfig = `apiVersion: v1
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
    context: {cluster: alpha-cluster, user: alpha-user}
  - name: beta
    context: {cluster: beta-cluster, user: beta-user}
users:
  - name: alpha-user
    user: {token: alpha-token}
  - name: beta-user
    user: {token: beta-token}
`

// setupKubeconfig reads and writes package-level state, so each case restores it.
func withKubeconfigFlags(t *testing.T, kubeconfig, context string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte(testKubeconfig), 0o600))

	origPath, origContext, origTemp := kubeconfigPath, kubeContext, tempKubeconfig
	t.Cleanup(func() {
		if tempKubeconfig != "" {
			//nolint: errcheck // best effort test cleanup
			os.Remove(tempKubeconfig)
		}
		kubeconfigPath, kubeContext, tempKubeconfig = origPath, origContext, origTemp
	})

	if kubeconfig == "default" {
		kubeconfig = path
	}
	kubeconfigPath, kubeContext, tempKubeconfig = kubeconfig, context, ""
	return path
}

func TestSetupKubeconfig(t *testing.T) {
	t.Run("no flags leaves the environment alone", func(t *testing.T) {
		withKubeconfigFlags(t, "", "")
		t.Setenv("KUBECONFIG", "/ambient/config")

		require.NoError(t, setupKubeconfig())
		require.Equal(t, "/ambient/config", os.Getenv("KUBECONFIG"))
		require.Empty(t, tempKubeconfig)
	})

	t.Run("kubeconfig only is exported without a temp file", func(t *testing.T) {
		path := withKubeconfigFlags(t, "default", "")
		t.Setenv("KUBECONFIG", "")

		require.NoError(t, setupKubeconfig())
		require.Equal(t, path, os.Getenv("KUBECONFIG"))
		require.Empty(t, tempKubeconfig, "nothing to clean up when no file was written")
	})

	t.Run("context is exported as a temp kubeconfig marked for cleanup", func(t *testing.T) {
		path := withKubeconfigFlags(t, "default", "beta")
		t.Setenv("KUBECONFIG", "")

		require.NoError(t, setupKubeconfig())
		require.NotEmpty(t, tempKubeconfig)
		require.Equal(t, tempKubeconfig, os.Getenv("KUBECONFIG"))
		require.NotEqual(t, path, tempKubeconfig)

		written, err := clientcmd.LoadFromFile(tempKubeconfig)
		require.NoError(t, err)
		require.Equal(t, "beta", written.CurrentContext)
	})

	t.Run("unknown context fails without exporting anything", func(t *testing.T) {
		withKubeconfigFlags(t, "default", "gamma")
		t.Setenv("KUBECONFIG", "/ambient/config")

		require.ErrorContains(t, setupKubeconfig(), `context "gamma" does not exist`)
		require.Equal(t, "/ambient/config", os.Getenv("KUBECONFIG"))
		require.Empty(t, tempKubeconfig)
	})
}

func TestChainKubeconfigPreRun(t *testing.T) {
	t.Run("runs before the vendored hook", func(t *testing.T) {
		withKubeconfigFlags(t, "default", "beta")
		t.Setenv("KUBECONFIG", "")

		var seen string
		cmd := &cobra.Command{Use: "vendored"}
		cmd.PersistentPreRun = func(_ *cobra.Command, _ []string) {
			seen = os.Getenv("KUBECONFIG")
		}

		chainKubeconfigPreRun(cmd)
		require.Nil(t, cmd.PersistentPreRun, "cobra would skip PersistentPreRunE if both were set")
		require.NoError(t, cmd.PersistentPreRunE(cmd, nil))
		require.Equal(t, tempKubeconfig, seen, "the vendored hook must see the exported kubeconfig")
	})

	t.Run("propagates the vendored hook error", func(t *testing.T) {
		withKubeconfigFlags(t, "", "")

		cmd := &cobra.Command{Use: "vendored"}
		cmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
			return os.ErrPermission
		}

		chainKubeconfigPreRun(cmd)
		require.ErrorIs(t, cmd.PersistentPreRunE(cmd, nil), os.ErrPermission)
	})

	t.Run("fails before the vendored hook on a bad context", func(t *testing.T) {
		withKubeconfigFlags(t, "default", "gamma")

		ran := false
		cmd := &cobra.Command{Use: "vendored"}
		cmd.PersistentPreRun = func(_ *cobra.Command, _ []string) { ran = true }

		chainKubeconfigPreRun(cmd)
		require.Error(t, cmd.PersistentPreRunE(cmd, nil))
		require.False(t, ran)
	})
}

func TestAddHiddenDummyFlagDoesNotShadowRealFlags(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "monitor"}
	// k9s registers these locally rather than persistently.
	var local string
	cmd.Flags().StringVar(&local, "context", "", "the real flag")

	addHiddenDummyFlag(cmd, "context")
	require.Nil(t, cmd.PersistentFlags().Lookup("context"), "must not shadow the command's own flag")

	addHiddenDummyFlag(cmd, "kubeconfig")
	require.NotNil(t, cmd.PersistentFlags().Lookup("kubeconfig"), "a free name still gets a dummy")
	require.True(t, cmd.PersistentFlags().Lookup("kubeconfig").Hidden)
}

func TestSetupRootFlags(t *testing.T) {
	withArgs := func(t *testing.T, args ...string) *cobra.Command {
		t.Helper()
		orig := os.Args
		t.Cleanup(func() { os.Args = orig })
		os.Args = args

		cmd := &cobra.Command{Use: "zarf"}
		setupRootFlags(cmd)
		return cmd
	}

	t.Run("registers the cluster flags", func(t *testing.T) {
		cmd := withArgs(t, "zarf", "package", "list")
		require.NotNil(t, cmd.PersistentFlags().Lookup("kubeconfig"))
		require.NotNil(t, cmd.PersistentFlags().Lookup("context"))
		require.Empty(t, cmd.PersistentFlags().Lookup("context").Shorthand, "-c belongs to subcommands")
		require.Empty(t, cmd.PersistentFlags().Lookup("kubeconfig").Shorthand)
	})

	t.Run("keeps shorthands for Zarf's own commands", func(t *testing.T) {
		cmd := withArgs(t, "zarf", "package", "list")
		require.Equal(t, "l", cmd.PersistentFlags().Lookup("log-level").Shorthand)
		require.Equal(t, "a", cmd.PersistentFlags().Lookup("architecture").Shorthand)
	})

	t.Run("drops shorthands when a vendored tool is the target", func(t *testing.T) {
		// kubectl and helm use -l for --selector, which makes pflag panic on merge.
		cmd := withArgs(t, "zarf", "--context", "dev", "tools", "kubectl", "get", "nodes")
		require.Empty(t, cmd.PersistentFlags().Lookup("log-level").Shorthand)
		require.Empty(t, cmd.PersistentFlags().Lookup("architecture").Shorthand)
		require.NotNil(t, cmd.PersistentFlags().Lookup("context"), "the cluster flags are still needed")
	})
}
