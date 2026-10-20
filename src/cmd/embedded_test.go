// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewZarfCommand_EmbeddedToolFlagIsolation(t *testing.T) {
	// Command construction and the upstream tools use process-wide configuration.
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "reject root logging flag before Helm selector command",
			args: []string{"--log-level", "info", "tools", "helm", "list"},
			want: "zarf flag --log-level cannot be used with embedded tool",
		},
		{
			name: "cache directory named tools does not hide the command",
			args: []string{"--cache", "tools", "tools", "helm", "list", "--help"},
			want: "zarf flag --cache cannot be used with embedded tool",
		},
		{
			name: "root shorthand before aliases",
			args: []string{"-l", "info", "t", "h", "list"},
			want: "zarf flag --log-level cannot be used with embedded tool",
		},
		{
			name: "root flags between tools and tool",
			args: []string{"tools", "--architecture", "amd64", "helm", "list"},
			want: "zarf flag --architecture cannot be used with embedded tool",
		},
		{
			name: "Zarf flags are not inherited by Helm",
			args: []string{"tools", "helm", "version", "--log-format=json"},
			want: "unknown flag: --log-format",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewZarfCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(tt.args)
			err := cmd.ExecuteContext(t.Context())
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestNewZarfCommand_EmbeddedHelmHelpAndCompletion(t *testing.T) {
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"help", "tools", "helm", "list"}, "zarf tools helm list"},
		{[]string{"__complete", "tools", "helm", "list", "--sel"}, "--selector"},
		{[]string{"__complete", "help", "tools", "helm", "li"}, "list"},
	} {
		t.Run(tt.want, func(t *testing.T) {
			cmd := NewZarfCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(tt.args)
			require.NoError(t, cmd.ExecuteContext(t.Context()))
			require.Contains(t, out.String(), tt.want)
			require.NotContains(t, out.String(), "--log-format")
		})
	}
}
