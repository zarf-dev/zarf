// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
)

// PrintFindings prints the findings in the LintError as a table.
func PrintFindings(ctx context.Context, lintErr *lint.LintError) {
	lintData := [][]string{}
	for _, finding := range lintErr.Findings {
		sevColor := color.FgWhite
		switch finding.Severity {
		case lint.SevErr:
			sevColor = color.FgRed
		case lint.SevWarn:
			sevColor = color.FgYellow
		}

		lintData = append(lintData, []string{
			colorWrap(string(finding.Severity), sevColor),
			colorWrap(finding.YqPath, color.FgCyan),
			finding.ItemizedDescription(),
		})
	}

	// Print table to our OutputWriter
	logger.From(ctx).Info("linting composed package", "name", lintErr.PackageName, "path", lintErr.BaseDir)
	message.TableWithWriter(OutputWriter, []string{"Type", "Path", "Message"}, lintData)
}

func colorWrap(str string, attr color.Attribute) string {
	if !message.ColorEnabled() || str == "" {
		return str
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", attr, str)
}
