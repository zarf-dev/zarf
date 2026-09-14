// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

var (
	aliasedTools = []struct {
		alias  string
		target string
	}{
		{"kubectl", "kubectl"},
		{"helm", "helm"},
		{"yq", "yq"},
		{"k9s", "monitor"},
		{"syft", "sbom"},
	}
	doubleQuoteEscaper = strings.NewReplacer(
		`\`, `\\`,
		`$`, `\$`,
		"`", "\\`",
		`"`, `\"`,
	)
	toolsAliasesUsage = "aliases"
)

func newToolsAliasesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   toolsAliasesUsage,
		Args:  cobra.NoArgs,
		Short: lang.CmdToolsAliasesShort,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runtime.GOOS == "windows" {
				return errors.New(lang.CmdToolsAliasesErrWindows)
			}

			executablePath, err := utils.GetFinalExecutablePath()
			if err != nil {
				return fmt.Errorf("resolving final executable path: %w", err)
			}

			_, err = fmt.Fprint(cmd.OutOrStdout(), toolsAliases(executablePath, os.Args))

			return err
		},
	}
}

func toolsAliases(executablePath string, args []string) string {
	aliasComponents := []string{escape(executablePath)}
	if config.ActionsCommandZarfPrefix != "" {
		aliasComponents = append(aliasComponents, config.ActionsCommandZarfPrefix)
	}

	for i, arg := range args {
		if i == 0 {
			continue
		}

		if arg == toolsAliasesUsage {
			break
		}

		aliasComponents = append(aliasComponents, escape(arg))
	}

	aliasPrefix := strings.Join(aliasComponents, " ")

	var output strings.Builder
	for _, tool := range aliasedTools {
		fmt.Fprintf(&output, "alias %s=%s\n", tool.alias, quote(aliasPrefix+" "+tool.target))
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

	return `"` + doubleQuoteEscaper.Replace(value) + `"`
}
