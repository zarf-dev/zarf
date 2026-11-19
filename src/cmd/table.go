// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"fmt"

	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
)

const (
	ansiRed    = 31
	ansiYellow = 33
	ansiCyan   = 36
	ansiWhite  = 37
)

// PrintFindings prints the findings in the LintError as a table.
func PrintFindings(ctx context.Context, lintErr *lint.LintError) {
	lintData := [][]string{}
	for _, finding := range lintErr.Findings {
		sevColor := ansiWhite
		switch finding.Severity {
		case lint.SevErr:
			sevColor = ansiRed
		case lint.SevWarn:
			sevColor = ansiYellow
		}

		lintData = append(lintData, []string{
			colorWrap(string(finding.Severity), sevColor),
			colorWrap(finding.YqPath, ansiCyan),
			finding.ItemizedDescription(),
		})
	}
	// Print table to our OutputWriter
	logger.From(ctx).Info("linting composed package definition", "name", lintErr.PackageName)
	message.Table(OutputWriter, []string{"Type", "Path", "Message"}, lintData)
}

func colorWrap(str string, code int) string {
	if !message.ColorEnabled() || str == "" {
		return str
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", code, str)
}
