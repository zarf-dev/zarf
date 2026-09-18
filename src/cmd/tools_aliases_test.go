// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
)

func TestToolsAliasesIntegration(t *testing.T) {
	cmd := newToolsAliasesCommand()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()

	if runtime.GOOS == "windows" {
		require.ErrorContains(t, err, lang.CmdToolsAliasesErrWindows)
		return
	}

	require.NoError(t, err)

	for i, alias := range strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n") {
		parts := strings.Split(alias, " ")
		cmd := parts[len(parts)-1]
		require.Contains(t, cmd, aliasedTools[i].target)
	}
}

func TestToolsAliasesOutput(t *testing.T) {
	tests := map[string]struct {
		before         func() func()
		executablePath string
		args           []string
		expected       string
	}{
		"standalone executable": {
			executablePath: "/path/zarf",
			args:           []string{"/path/zarf", "tools", "aliases"},
			expected: "" +
				"alias kubectl='/path/zarf tools kubectl'\n" +
				"alias helm='/path/zarf tools helm'\n" +
				"alias yq='/path/zarf tools yq'\n" +
				"alias k9s='/path/zarf tools monitor'\n" +
				"alias syft='/path/zarf tools sbom'\n",
		},
		"embedded executable": {
			executablePath: "/path/uds",
			args:           []string{"/path/uds", "zarf", "tools", "aliases"},
			expected: "" +
				"alias kubectl='/path/uds zarf tools kubectl'\n" +
				"alias helm='/path/uds zarf tools helm'\n" +
				"alias yq='/path/uds zarf tools yq'\n" +
				"alias k9s='/path/uds zarf tools monitor'\n" +
				"alias syft='/path/uds zarf tools sbom'\n",
		},
		"escapes whitespace and shell metacharacters": {
			executablePath: "/path with spaces/$zarf",
			args:           []string{"/path with spaces/$zarf", "tools", "aliases"},
			expected: "" +
				`alias kubectl='/path\ with\ spaces/\$zarf tools kubectl'` + "\n" +
				`alias helm='/path\ with\ spaces/\$zarf tools helm'` + "\n" +
				`alias yq='/path\ with\ spaces/\$zarf tools yq'` + "\n" +
				`alias k9s='/path\ with\ spaces/\$zarf tools monitor'` + "\n" +
				`alias syft='/path\ with\ spaces/\$zarf tools sbom'` + "\n",
		},
		"escapes double-quote-unfriendly characters": {
			executablePath: "/linus' $special$ k8s tools/zarf",
			args:           []string{"/linus' $special$ k8s tools/zarf", "tools", "aliases"},
			expected: "" +
				`alias kubectl="/linus\\'\\ \\\$special\\\$\\ k8s\\ tools/zarf tools kubectl"` + "\n" +
				`alias helm="/linus\\'\\ \\\$special\\\$\\ k8s\\ tools/zarf tools helm"` + "\n" +
				`alias yq="/linus\\'\\ \\\$special\\\$\\ k8s\\ tools/zarf tools yq"` + "\n" +
				`alias k9s="/linus\\'\\ \\\$special\\\$\\ k8s\\ tools/zarf tools monitor"` + "\n" +
				`alias syft="/linus\\'\\ \\\$special\\\$\\ k8s\\ tools/zarf tools sbom"` + "\n",
		},
		"multi-token embedded prefix": {
			executablePath: "/path/uds",
			args:           []string{"/path/uds", "wrapper", "zarf", "tools", "aliases"},
			expected: "" +
				"alias kubectl='/path/uds wrapper zarf tools kubectl'\n" +
				"alias helm='/path/uds wrapper zarf tools helm'\n" +
				"alias yq='/path/uds wrapper zarf tools yq'\n" +
				"alias k9s='/path/uds wrapper zarf tools monitor'\n" +
				"alias syft='/path/uds wrapper zarf tools sbom'\n",
		},
		"with custom zarf prefix": {
			before: func() func() {
				oldPrefix := config.ActionsCommandZarfPrefix
				config.ActionsCommandZarfPrefix = "thisisatest"
				return func() {
					config.ActionsCommandZarfPrefix = oldPrefix
				}
			},
			executablePath: "/path/uds",
			args:           []string{"/path/uds", "zarf", "tools", "aliases"},
			expected: "" +
				"alias kubectl='/path/uds thisisatest zarf tools kubectl'\n" +
				"alias helm='/path/uds thisisatest zarf tools helm'\n" +
				"alias yq='/path/uds thisisatest zarf tools yq'\n" +
				"alias k9s='/path/uds thisisatest zarf tools monitor'\n" +
				"alias syft='/path/uds thisisatest zarf tools sbom'\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.before != nil {
				after := test.before()
				defer after()
			}
			require.Equal(t, test.expected, toolsAliases(test.executablePath, test.args))
		})
	}
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
