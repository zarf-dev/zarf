// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/schema"
)

func TestParseFailureLevel(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		input string
		want  Severity
	}{
		{input: "", want: SevErr},
		{input: "error", want: SevErr},
		{input: "warning", want: SevWarn},
		{input: "info", want: SevInfo},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			level, err := ParseFailureLevel(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.want, level)
		})
	}
	for _, input := range []string{"Error", "Warning", "Info", "ERROR", "warn", "none", " warning", "info "} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			_, err := ParseFailureLevel(input)
			require.ErrorContains(t, err, "invalid failure level")
		})
	}
}

func TestLintErrorFailsAt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		findings []PackageFinding
		err      error
		want     [3]bool // error, warning, info thresholds
	}{
		{name: "no findings"},
		{
			name:     "info",
			findings: []PackageFinding{{Severity: SevInfo}},
			want:     [3]bool{false, false, true},
		},
		{
			name:     "warning",
			findings: []PackageFinding{{Severity: SevWarn}},
			want:     [3]bool{false, true, true},
		},
		{
			name:     "error",
			findings: []PackageFinding{{Severity: SevErr}},
			want:     [3]bool{true, true, true},
		},
		{
			name:     "info and warning",
			findings: []PackageFinding{{Severity: SevInfo}, {Severity: SevWarn}},
			want:     [3]bool{false, true, true},
		},
		{
			name:     "all severities",
			findings: []PackageFinding{{Severity: SevInfo}, {Severity: SevWarn}, {Severity: SevErr}},
			want:     [3]bool{true, true, true},
		},
		{
			name: "operational failure without findings",
			err:  errors.New("resource unavailable"),
			want: [3]bool{true, true, true},
		},
		{
			name:     "operational failure with info",
			findings: []PackageFinding{{Severity: SevInfo}},
			err:      errors.New("resource unavailable"),
			want:     [3]bool{true, true, true},
		},
		{
			name:     "operational failure with warning",
			findings: []PackageFinding{{Severity: SevWarn}},
			err:      errors.New("resource unavailable"),
			want:     [3]bool{true, true, true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lintErr := &LintError{Findings: tc.findings, Err: tc.err}
			for i, level := range []Severity{SevErr, SevWarn, SevInfo} {
				require.Equal(t, tc.want[i], lintErr.FailsAt(level), "level %s", level)
			}
		})
	}
}

func TestLintErrorUnwrap(t *testing.T) {
	t.Parallel()
	cause := errors.New("resource unavailable")
	for _, findings := range [][]PackageFinding{nil, {{Severity: SevInfo}}, {{Severity: SevWarn}}} {
		lintErr := &LintError{Findings: findings, Err: fmt.Errorf("reading values: %w", cause)}
		wrapped := fmt.Errorf("lint source: %w", lintErr)
		require.ErrorIs(t, wrapped, cause)
		var result *LintError
		require.ErrorAs(t, wrapped, &result)
		require.Same(t, lintErr, result)
		require.ErrorContains(t, lintErr, "reading values: resource unavailable")
	}
	// Schema diagnostics stay discoverable through the lint result.
	cause = schema.Validate(schema.GetV1Beta1Schema(), map[string]any{"kind": "ZarfPackageConfig"})
	var validationErr *schema.ValidationError
	require.ErrorAs(t, fmt.Errorf("lint source: %w", &LintError{Err: cause}), &validationErr)
	require.NotEmpty(t, validationErr.Errors)
	findingsOnly := &LintError{Findings: []PackageFinding{{Severity: SevWarn}}}
	require.NoError(t, findingsOnly.Unwrap())
	require.EqualError(t, findingsOnly, "linting error found 1 instance(s)")
}
