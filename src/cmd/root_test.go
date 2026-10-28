// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInvalidCommandsReturnError(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		wantErrContains string
	}{
		{
			name:            "unknown root command",
			args:            []string{"foo"},
			wantErrContains: `unknown command "foo" for "zarf"`,
		},
		{
			name:            "unknown package subcommand",
			args:            []string{"package", "foo"},
			wantErrContains: `unknown command "foo" for "zarf package"`,
		},
		{
			name:            "unknown component subcommand",
			args:            []string{"component", "foo"},
			wantErrContains: `unknown command "foo" for "zarf component"`,
		},
		{
			name:            "unknown dev subcommand",
			args:            []string{"dev", "foo"},
			wantErrContains: `unknown command "foo" for "zarf dev"`,
		},
		{
			name:            "unknown dev inspect subcommand",
			args:            []string{"dev", "inspect", "foo"},
			wantErrContains: `unknown command "foo" for "zarf dev inspect"`,
		},
		{
			name:            "unknown tools subcommand",
			args:            []string{"tools", "foo"},
			wantErrContains: `unknown command "foo" for "zarf tools"`,
		},
		{
			name:            "unknown tools archiver subcommand",
			args:            []string{"tools", "archiver", "foo"},
			wantErrContains: `unknown command "foo" for "zarf tools archiver"`,
		},
		{
			name:            "unknown tools registry subcommand",
			args:            []string{"tools", "registry", "foo"},
			wantErrContains: `unknown command "foo" for "zarf tools registry"`,
		},
		{
			name:            "unknown tools trusted-root subcommand",
			args:            []string{"tools", "trusted-root", "foo"},
			wantErrContains: `unknown command "foo" for "zarf tools trusted-root"`,
		},
		{
			name:            "unknown internal subcommand",
			args:            []string{"internal", "foo"},
			wantErrContains: `unknown command "foo" for "zarf internal"`,
		},
		{
			name:            "unknown version argument",
			args:            []string{"version", "foo"},
			wantErrContains: `unknown command "foo" for "zarf version"`,
		},
		{
			name:            "unknown say argument",
			args:            []string{"say", "foo"},
			wantErrContains: `unknown command "foo" for "zarf say"`,
		},
		{
			name:            "unknown clear-cache argument",
			args:            []string{"tools", "clear-cache", "foo"},
			wantErrContains: `unknown command "foo" for "zarf tools clear-cache"`,
		},
		{
			name:            "unknown download-init argument",
			args:            []string{"tools", "download-init", "foo"},
			wantErrContains: `unknown command "foo" for "zarf tools download-init"`,
		},
		{
			name:            "unknown gen-key argument",
			args:            []string{"tools", "gen-key", "foo"},
			wantErrContains: `unknown command "foo" for "zarf tools gen-key"`,
		},
		{
			name:            "unknown connect list argument",
			args:            []string{"connect", "list", "foo"},
			wantErrContains: `unknown command "foo" for "zarf connect list"`,
		},
		{
			name:            "unknown connect resource argument",
			args:            []string{"connect", "resource", "foo"},
			wantErrContains: `unknown command "foo" for "zarf connect resource"`,
		},
		{
			name:            "unknown internal agent argument",
			args:            []string{"internal", "agent", "foo"},
			wantErrContains: `unknown command "foo" for "zarf internal agent"`,
		},
		{
			name:            "unknown internal is-valid-hostname argument",
			args:            []string{"internal", "is-valid-hostname", "foo"},
			wantErrContains: `unknown command "foo" for "zarf internal is-valid-hostname"`,
		},
		{
			name:            "too many connect arguments",
			args:            []string{"connect", "foo", "bar"},
			wantErrContains: `accepts at most 1 arg(s), received 2`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewZarfCommand()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErrContains)
		})
	}
}

func TestParentCommandsWithoutArgsSucceed(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "root without args",
			args: []string{},
		},
		{
			name: "package without args",
			args: []string{"package"},
		},
		{
			name: "component without args",
			args: []string{"component"},
		},
		{
			name: "dev without args",
			args: []string{"dev"},
		},
		{
			name: "dev inspect without args",
			args: []string{"dev", "inspect"},
		},
		{
			name: "tools without args",
			args: []string{"tools"},
		},
		{
			name: "tools archiver without args",
			args: []string{"tools", "archiver"},
		},
		{
			name: "tools registry without args",
			args: []string{"tools", "registry"},
		},
		{
			name: "tools trusted-root without args",
			args: []string{"tools", "trusted-root"},
		},
		{
			name: "internal without args",
			args: []string{"internal"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewZarfCommand()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			require.NoError(t, err)
		})
	}
}

func TestExecute(t *testing.T) {
	t.Run("returns error on invalid command", func(t *testing.T) {
		rootCmd.SetArgs([]string{"tools", "foo"})
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		t.Cleanup(func() {
			rootCmd.SetArgs(nil)
			rootCmd.SetOut(os.Stdout)
			rootCmd.SetErr(os.Stderr)
		})

		err := Execute(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), `unknown command "foo" for "zarf tools"`)
	})

	t.Run("succeeds on help command", func(t *testing.T) {
		rootCmd.SetArgs([]string{"help"})
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		t.Cleanup(func() {
			rootCmd.SetArgs(nil)
			rootCmd.SetOut(os.Stdout)
			rootCmd.SetErr(os.Stderr)
		})

		err := Execute(context.Background())
		require.NoError(t, err)
	})
}
