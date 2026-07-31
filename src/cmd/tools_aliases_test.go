// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToolsAliasesOutput(t *testing.T) {
	tests := map[string]struct {
		executablePath string
		prefix         string
		expected       string
	}{
		"native Zarf executable omits a prefix": {
			executablePath: "/path/zarf",
			expected: "" +
				"alias kubectl='/path/zarf tools kubectl'\n" +
				"alias helm='/path/zarf tools helm'\n" +
				"alias yq='/path/zarf tools yq'\n" +
				"alias k9s='/path/zarf tools monitor'\n" +
				"alias syft='/path/zarf tools sbom'\n",
		},
		"embedded executable infers the Zarf prefix": {
			executablePath: "/path/uds",
			expected: "" +
				"alias kubectl='/path/uds zarf tools kubectl'\n" +
				"alias helm='/path/uds zarf tools helm'\n" +
				"alias yq='/path/uds zarf tools yq'\n" +
				"alias k9s='/path/uds zarf tools monitor'\n" +
				"alias syft='/path/uds zarf tools sbom'\n",
		},
		"escapes whitespace and shell metacharacters": {
			executablePath: "/path with spaces/$zarf",
			expected: "" +
				`alias kubectl='/path\ with\ spaces/\$zarf zarf tools kubectl'` + "\n" +
				`alias helm='/path\ with\ spaces/\$zarf zarf tools helm'` + "\n" +
				`alias yq='/path\ with\ spaces/\$zarf zarf tools yq'` + "\n" +
				`alias k9s='/path\ with\ spaces/\$zarf zarf tools monitor'` + "\n" +
				`alias syft='/path\ with\ spaces/\$zarf zarf tools sbom'` + "\n",
		},
		"explicit prefix overrides embedded inference": {
			executablePath: "/path/uds",
			prefix:         "wrapper",
			expected: "" +
				"alias kubectl='/path/uds wrapper tools kubectl'\n" +
				"alias helm='/path/uds wrapper tools helm'\n" +
				"alias yq='/path/uds wrapper tools yq'\n" +
				"alias k9s='/path/uds wrapper tools monitor'\n" +
				"alias syft='/path/uds wrapper tools sbom'\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, test.expected, toolsAliases(test.executablePath, test.prefix))
		})
	}
}

func TestToolsAliasesExecuteLiteralUnsafeExecutablePath(t *testing.T) {
	executablePath := filepath.Join(t.TempDir(), "zarf\\'$(printf injected)$HOME")
	require.NoError(t, os.WriteFile(executablePath, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0o700))

	aliasesFile := filepath.Join(t.TempDir(), "aliases")
	require.NoError(t, os.WriteFile(aliasesFile, []byte(toolsAliases(executablePath, "")), 0o600))

	output, err := exec.Command("sh", "-c", ". \"$1\"\neval 'kubectl literal-argument'", "sh", aliasesFile).Output()
	require.NoError(t, err)
	require.Equal(t, "zarf\ntools\nkubectl\nliteral-argument\n", string(output))
}

func TestToolsAliasesTargetsExistInCommandTree(t *testing.T) {
	rootCmd := NewZarfCommand()

	for _, tool := range aliasedTools {
		t.Run(tool.alias, func(t *testing.T) {
			cmd, _, err := rootCmd.Find([]string{"tools", tool.target})

			require.NoError(t, err)
			require.Equal(t, tool.target, cmd.Name())
		})
	}
}
