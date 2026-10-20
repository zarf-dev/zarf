// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbeddedTools(t *testing.T) {
	// Embedded tools must work independently of Zarf's configuration and feature flags.
	configPath := filepath.Join(t.TempDir(), "zarf-config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("invalid = ["), 0o600))
	t.Setenv("ZARF_CONFIG", configPath)
	t.Setenv("ZARF_LOG_LEVEL", "invalid")
	t.Setenv("ZARF_FEATURES", "invalid")
	t.Setenv("SYFT_CHECK_FOR_APP_UPDATE", "false")

	run := func(t *testing.T, args ...string) (string, error) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), e2e.ZarfBinPath, args...)
		out, err := cmd.CombinedOutput()
		require.NotContains(t, string(out), "panic:")
		return string(out), err
	}

	t.Run("tool versions and aliases ignore Zarf configuration", func(t *testing.T) {
		for _, tt := range []struct {
			args []string
			want string
		}{
			{[]string{"tools", "helm", "version", "--short"}, "v4."},
			{[]string{"t", "h", "version", "--short"}, "v4."},
			{[]string{"tools", "kubectl", "version", "--client"}, "Client Version:"},
			{[]string{"t", "k", "version", "--client"}, "Client Version:"},
			{[]string{"tools", "sbom", "version"}, "Application:   syft"},
			{[]string{"t", "syft", "version"}, "Application:   syft"},
			{[]string{"t", "s", "version"}, "Application:   syft"},
			{[]string{"tools", "monitor", "version"}, "Version:"},
			{[]string{"t", "k9s", "version"}, "Version:"},
			{[]string{"t", "m", "version"}, "Version:"},
			{[]string{"tools", "yq", "--version"}, "yq"},
		} {
			t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
				out, err := run(t, tt.args...)
				require.NoError(t, err, out)
				require.Contains(t, out, tt.want)
			})
		}
	})

	t.Run("yq evaluates expressions and preserves exit status", func(t *testing.T) {
		out, err := run(t, "tools", "yq", "-n", ".answer = 42")
		require.NoError(t, err, out)
		require.Equal(t, "answer: 42\n", out)

		out, err = run(t, "t", "yq", "-n", "-e", "false")
		require.Error(t, err)
		require.Contains(t, out, "no matches found")
	})

	t.Run("kubectl owns the label selector shorthand", func(t *testing.T) {
		manifest := filepath.Join(t.TempDir(), "configmap.yaml")
		require.NoError(t, os.WriteFile(manifest, []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: example\n  labels:\n    app: web\n"), 0o600))
		out, err := run(t, "tools", "kubectl", "label", "--local", "-f", manifest, "-l", "app=web", "stage=test", "-o", "json")
		require.NoError(t, err, out)
		require.Contains(t, out, `"stage": "test"`)
	})

	t.Run("Helm help and completion use the tool namespace", func(t *testing.T) {
		for _, args := range [][]string{
			{"tools", "helm", "list", "--help"},
			{"help", "tools", "helm", "list"},
			{"t", "h", "help", "list"},
		} {
			out, err := run(t, args...)
			require.NoError(t, err, out)
			require.Contains(t, out, "zarf tools helm list")
			require.Contains(t, out, "--selector")
			require.NotContains(t, out, "--log-format")
		}
		for _, request := range []string{"__complete", "__completeNoDesc"} {
			out, err := run(t, request, "tools", "helm", "list", "--sel")
			require.NoError(t, err, out)
			require.Contains(t, out, "--selector")
			require.Contains(t, out, ":4\n")
		}
	})

	t.Run("Zarf options are rejected without a panic", func(t *testing.T) {
		for _, args := range [][]string{
			{"--log-level", "info", "tools", "helm", "list"},
			{"--cache", "tools", "tools", "helm", "list", "--help"},
			{"-l", "info", "t", "k", "get", "pods"},
		} {
			out, err := run(t, args...)
			require.Error(t, err)
			require.Contains(t, out, "cannot be used with embedded tool")
		}
		out, err := run(t, "tools", "helm", "version", "--log-format=json")
		require.Error(t, err)
		require.Contains(t, out, "unknown flag: --log-format")
	})

	t.Run("kubectl plugins receive arguments and exit status", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("test plugin uses a POSIX shell")
		}
		dir := t.TempDir()
		plugin := filepath.Join(dir, "kubectl-zarfdispatchtest")
		require.NoError(t, os.WriteFile(plugin, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\nexit 7\n"), 0o700))
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		out, err := run(t, "t", "k", "zarfdispatchtest", "-l", "debug", "--", "tools", "helm")
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr)
		require.Equal(t, 7, exitErr.ExitCode())
		require.Equal(t, "-l\ndebug\n--\ntools\nhelm\n", out)
	})
}
