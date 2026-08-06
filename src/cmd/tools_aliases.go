// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

var aliasedTools = []struct {
	alias  string
	target string
}{
	{"kubectl", "kubectl"},
	{"helm", "helm"},
	{"yq", "yq"},
	{"k9s", "monitor"},
	{"syft", "sbom"},
}

var escapeReplacer = strings.NewReplacer(
	`\`, `\\`,
	`$`, `\$`,
	"`", "\\`",
	`"`, `\"`,
)

func newToolsAliasesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "aliases",
		Args:  cobra.NoArgs,
		Short: lang.CmdToolsAliasesShort,
		RunE: func(cmd *cobra.Command, _ []string) error {
			executablePath, err := utils.GetFinalExecutablePath()
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), toolsAliases(executablePath, config.ActionsCommandZarfPrefix))
			return err
		},
	}
}

func toolsAliases(executablePath, prefix string) string {
	if prefix == "" && filepath.Base(executablePath) != "zarf" {
		prefix = "zarf"
	}

	command := escape(executablePath)
	if prefix != "" {
		command += " " + prefix
	}

	var output strings.Builder
	for _, tool := range aliasedTools {
		fmt.Fprintf(&output, "alias %s=%s\n", tool.alias, quote(command+" tools "+tool.target))
	}

	return output.String()
}

func escape(value string) string {
	var escaped strings.Builder

	for _, r := range value {
		if r == '\n' {
			escaped.WriteString("'\n'")
			continue
		}

		if !isShellSafe(r) {
			escaped.WriteByte('\\')
		}

		escaped.WriteRune(r)
	}

	return escaped.String()
}

func isShellSafe(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_@%+=:,./-", r)
}

func quote(value string) string {
	if !strings.Contains(value, "'") {
		return "'" + value + "'"
	}

	return `"` + escapeReplacer.Replace(value) + `"`
}
