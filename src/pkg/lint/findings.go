// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"fmt"
)

// LintError represents an error containing lint findings.
type LintError struct {
	PackageName  string
	Findings     []PackageFinding
	SkippedRules []string
	NotEvaluated []string
	// Err is an operational or prerequisite failure that always fails linting.
	Err error
}

func (e *LintError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("linting error found %d instance(s): %v", len(e.Findings), e.Err)
	}
	return fmt.Sprintf("linting error found %d instance(s)", len(e.Findings))
}

// Unwrap exposes the underlying failure to errors.Is and errors.As, including
// schema validation diagnostics carried by Err rather than lint findings.
func (e *LintError) Unwrap() error {
	return e.Err
}

// FailsAt reports whether findings meet the failure level. Operational and
// prerequisite failures always fail, regardless of the level.
func (e *LintError) FailsAt(level Severity) bool {
	if e.Err != nil {
		return true
	}
	for _, f := range e.Findings {
		switch f.Severity {
		case SevErr:
			return true
		case SevWarn:
			if level == SevWarn || level == SevInfo {
				return true
			}
		case SevInfo:
			if level == SevInfo {
				return true
			}
		}
	}
	return false
}

// Severity is the type of finding.
type Severity string

// Severity definitions.
const (
	SevErr  = "Error"
	SevWarn = "Warning"
	SevInfo = "Info"
)

// ParseFailureLevel parses a lowercase failure level. An empty level defaults to error.
func ParseFailureLevel(level string) (Severity, error) {
	switch level {
	case "", "error":
		return SevErr, nil
	case "warning":
		return SevWarn, nil
	case "info":
		return SevInfo, nil
	default:
		return "", fmt.Errorf("invalid failure level %q: must be error, warning, or info", level)
	}
}

// PackageFinding is a struct that contains a finding about something wrong with a package
type PackageFinding struct {
	// RuleID identifies the semantic lint rule.
	RuleID string
	// Source is the source file or resource locator, when known.
	Source string
	// Line and Column are one-based source coordinates, or zero when unavailable.
	Line   int
	Column int
	// YqPath is the path to the key where the error originated from, this is sometimes empty in the case of a general error
	YqPath      string
	Description string
	// Item is the value of a key that is causing an error, for example a bad image name
	Item string
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
