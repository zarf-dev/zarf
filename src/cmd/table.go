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
		case lint.SevInfo:
			sevColor = color.FgBlue
		}

		location := finding.YqPath
		if finding.Source != "" {
			location = finding.Source
			if finding.Line > 0 {
				location += fmt.Sprintf(":%d:%d", finding.Line, finding.Column)
			}
			if finding.YqPath != "" {
				location += " (" + finding.YqPath + ")"
			}
		}
		lintData = append(lintData, []string{
			colorWrap(string(finding.Severity), sevColor),
			finding.RuleID,
			colorWrap(location, color.FgCyan),
			finding.ItemizedDescription(),
		})
	}
	// Print table to our OutputWriter
	logger.From(ctx).Info("linting composed package definition", "name", lintErr.PackageName)
	if len(lintData) > 0 {
		message.TableWithWriter(OutputWriter, []string{"Type", "Rule", "Location", "Message"}, lintData)
	}
	if len(lintErr.NotEvaluated) > 0 {
		fmt.Fprintln(OutputWriter, "Not evaluated:")
		for _, check := range lintErr.NotEvaluated {
			fmt.Fprintf(OutputWriter, "- %s\n", check)
		}
	}
}

func colorWrap(str string, attr color.Attribute) string {
	if !message.ColorEnabled() || str == "" {
		return str
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", attr, str)
}
