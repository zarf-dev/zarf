// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package common

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/fatih/color"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/message"
)

// OutputWriter provides a writer to stdout for user-focused output
var OutputWriter = os.Stdout

// PrintFindings prints the findings in the LintError as a table.
func PrintFindings(lintErr *lint.LintError) {
	mapOfFindingsByPath := lint.GroupFindingsByPath(lintErr.Findings, lintErr.PackageName)
	for _, findings := range mapOfFindingsByPath {
		lintData := [][]string{}
		for _, finding := range findings {
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
		var packagePathFromUser string
		if helpers.IsOCIURL(findings[0].PackagePathOverride) {
			packagePathFromUser = findings[0].PackagePathOverride
		} else {
			packagePathFromUser = filepath.Join(lintErr.BaseDir, findings[0].PackagePathOverride)
		}

		// Print table to our OutputWriter
		message.Notef("Linting package %q at %s", findings[0].PackageNameOverride, packagePathFromUser)
		message.TableWithWriter(OutputWriter, []string{"Type", "Path", "Message"}, lintData)
	}
}

func colorWrap(str string, attr color.Attribute) string {
	if !message.ColorEnabled() || str == "" {
		return str
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", attr, str)
}
