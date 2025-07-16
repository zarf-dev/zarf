// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLintError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		findings     []PackageFinding
		onlyWarnings bool
	}{
		{
			name: "only warnings",
			findings: []PackageFinding{
				{
					Severity: SevWarn,
				},
			},
			onlyWarnings: true,
		},
		{
			name: "warnings and errors",
			findings: []PackageFinding{
				{
					Severity: SevWarn,
				},
				{
					Severity: SevErr,
				},
			},
			onlyWarnings: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lintErr := &LintError{
				Findings: tc.findings,
			}
			require.Equal(t, tc.onlyWarnings, lintErr.OnlyWarnings())
		})
	}
}
