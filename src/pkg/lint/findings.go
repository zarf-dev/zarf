// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"fmt"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/fatih/color"
	"github.com/zarf-dev/zarf/src/pkg/message"
)

// PackageFinding is a struct that contains a finding about something wrong with a package
type PackageFinding struct {
	// YqPath is the path to the key where the error originated from, this is sometimes empty in the case of a general error
	YqPath      string
	Description string
	// Item is the value of a key that is causing an error, for example a bad image name
	Item string
	// PackageNameOverride shows the name of the package that the error originated from
	// If it is not set the base package will be used when displaying the error
	PackageNameOverride string
	// PackagePathOverride shows the path to the package that the error originated from
	// If it is not set the base package will be used when displaying the error
	PackagePathOverride string
	Severity            Severity
}

// Severity is the type of finding
type Severity int

// different severities of package errors
const (
	SevErr Severity = iota + 1
	SevWarn
)

func (f PackageFinding) itemizedDescription() string {
	if f.Item == "" {
		return f.Description
	}
	return fmt.Sprintf("%s - %s", f.Description, f.Item)
}

func colorWrapSev(s Severity) string {
	if s == SevErr {
		return message.ColorWrap("Error", color.FgRed)
	} else if s == SevWarn {
		return message.ColorWrap("Warning", color.FgYellow)
	}
	return "unknown"
}

func filterLowerSeverity(findings []PackageFinding, severity Severity) []PackageFinding {
	findings = helpers.RemoveMatches(findings, func(finding PackageFinding) bool {
		return finding.Severity > severity
	})
	return findings
}

// PrintFindings prints the findings of the given severity in a table
func PrintFindings(findings []PackageFinding, severity Severity, baseDir string, packageName string) {
	findings = filterLowerSeverity(findings, severity)
	if len(findings) == 0 {
		return
	}
	mapOfFindingsByPath := GroupFindingsByPath(findings, packageName)

	header := []string{"Type", "Path", "Message"}

	for _, findings := range mapOfFindingsByPath {
		lintData := [][]string{}
		for _, finding := range findings {
			lintData = append(lintData, []string{
				colorWrapSev(finding.Severity),
				message.ColorWrap(finding.YqPath, color.FgCyan),
				finding.itemizedDescription(),
			})
		}
		var packagePathFromUser string
		if helpers.IsOCIURL(findings[0].PackagePathOverride) {
			packagePathFromUser = findings[0].PackagePathOverride
		} else {
			packagePathFromUser = filepath.Join(baseDir, findings[0].PackagePathOverride)
		}
		message.Notef("Linting package %q at %s", findings[0].PackageNameOverride, packagePathFromUser)
		message.Table(header, lintData)
	}
}

// GroupFindingsByPath groups findings by their package path
func GroupFindingsByPath(findings []PackageFinding, packageName string) map[string][]PackageFinding {
	for i := range findings {
		if findings[i].PackageNameOverride == "" {
			findings[i].PackageNameOverride = packageName
		}
		if findings[i].PackagePathOverride == "" {
			findings[i].PackagePathOverride = "."
		}
	}

	mapOfFindingsByPath := make(map[string][]PackageFinding)
	for _, finding := range findings {
		mapOfFindingsByPath[finding.PackagePathOverride] = append(mapOfFindingsByPath[finding.PackagePathOverride], finding)
	}
	return mapOfFindingsByPath
}

// HasSevOrHigher returns true if the findings contain a severity equal to or greater than the given severity
func HasSevOrHigher(findings []PackageFinding, severity Severity) bool {
	return len(filterLowerSeverity(findings, severity)) > 0
}
