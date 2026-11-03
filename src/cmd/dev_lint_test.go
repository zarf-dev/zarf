// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestDevLintValidatesControlsBeforeInputs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags []string
		want  string
	}{
		{"failure level", []string{"--fail-on=none"}, "invalid failure level"},
		{"case sensitive level", []string{"--fail-on=Warning"}, "invalid failure level"},
		{"unknown rule", []string{"--skip-rule=not-a-rule"}, "unknown lint rule"},
		{"repeatable skips", []string{"--skip-rule=image-unpinned", "--skip-rule=not-a-rule"}, "unknown lint rule"},
		{"no wildcards", []string{"--skip-rule=image-*"}, "unknown lint rule"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newDevLintCommand(viper.New())
			cmd.SetArgs(append([]string{"missing.tar.zst", "--values=missing-values.yaml"}, tc.flags...))
			err := cmd.ExecuteContext(t.Context())
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestDevLintCreationFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want map[string]string
	}{
		{"config defaults", nil, map[string]string{"tag": "config", "config_only": "retained"}},
		{"canonical", []string{"--create-set=tag=cli"}, map[string]string{"tag": "cli"}},
		{"deprecated alias", []string{"--set=tag=cli"}, map[string]string{"tag": "cli"}},
		{"canonical then alias", []string{"--create-set=tag=first,canonical=kept", "--set=tag=last,alias=kept"}, map[string]string{"tag": "last", "canonical": "kept", "alias": "kept"}},
		{"alias then canonical", []string{"--set=tag=first,alias=kept", "--create-set=tag=last,canonical=kept"}, map[string]string{"tag": "last", "canonical": "kept", "alias": "kept"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := viper.New()
			config.Set(VPkgCreateSet, map[string]string{"tag": "config", "config_only": "retained"})
			cmd := newDevLintCommand(config)
			canonical, alias := cmd.Flags().Lookup("create-set"), cmd.Flags().Lookup("set")
			require.Same(t, canonical.Value, alias.Value)
			require.False(t, canonical.Hidden)
			require.Empty(t, canonical.Deprecated)
			require.True(t, alias.Hidden)
			require.Contains(t, alias.Deprecated, "--create-set")
			require.NoError(t, cmd.ParseFlags(tc.args))
			for _, name := range []string{"create-set", "set"} {
				got, err := cmd.Flags().GetStringToString(name)
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestDevLintInputFlags(t *testing.T) {
	cmd := newDevLintCommand(viper.New())
	require.NoError(t, cmd.ParseFlags([]string{
		"--flavor=edge", "--deploy-set-variables=token=cli",
		"--values=first.yaml", "--values=second.yaml",
		"--set-values=app.replicas=5,app.enabled=true", "--fail-on=warning",
		"--skip-rule=image-unpinned", "--skip-rule=values-unused",
	}))
	for name, want := range map[string]string{"flavor": "edge", "fail-on": "warning"} {
		got, err := cmd.Flags().GetString(name)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	values, err := cmd.Flags().GetStringSlice("values")
	require.NoError(t, err)
	require.Equal(t, []string{"first.yaml", "second.yaml"}, values)
	skips, err := cmd.Flags().GetStringArray("skip-rule")
	require.NoError(t, err)
	require.Equal(t, []string{"image-unpinned", "values-unused"}, skips)
	deploy, err := cmd.Flags().GetStringToString("deploy-set-variables")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"token": "cli"}, deploy)
	setValues, err := cmd.Flags().GetStringToString("set-values")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"app.replicas": "5", "app.enabled": "true"}, setValues)
}
