// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVendorCmdInArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{name: "plain vendor command", args: []string{"zarf", "tools", "kubectl", "get", "nodes"}, expected: true},
		{name: "root flag before subcommand", args: []string{"zarf", "--log-level", "info", "tools", "helm", "list"}, expected: true},
		{name: "tools alias", args: []string{"zarf", "-l", "debug", "t", "helm", "list"}, expected: true},
		{name: "non-vendored tools subcommand", args: []string{"zarf", "--log-level", "info", "tools", "update-creds"}, expected: false},
		{name: "tools with nothing after it", args: []string{"zarf", "tools"}, expected: false},
		{name: "not a tools invocation", args: []string{"zarf", "package", "deploy", "tools"}, expected: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, vendorCmdInArgs(tt.args))
		})
	}
}
