// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"fmt"
)

// LintError represents an error containing lint findings.
//
//nolint:revive // ignore name
type LintError struct {
	BaseDir     string
	PackageName string
	Findings    []PackageFinding
}

func (e *LintError) Error() string {
	return fmt.Sprintf("linting error found %d instance(s)", len(e.Findings))
}

// OnlyWarnings returns true if all findings have severity warning.
func (e *LintError) OnlyWarnings() bool {
	for _, f := range e.Findings {
		if f.Severity == SevErr {
			return false
		}
	}
	return true
}

// Severity is the type of finding.
type Severity string

// Severity definitions.
const (
	SevErr  = "Error"
	SevWarn = "Warning"
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
	// Severity of finding.
	Severity Severity
}

// ItemizedDescription returns a string with the description and item if finding contains one.
func (f PackageFinding) ItemizedDescription() string {
	if f.Item == "" {
		return f.Description
	}
	return fmt.Sprintf("%s - %s", f.Description, f.Item)
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
