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

	lintErr := &LintError{
		Findings: []PackageFinding{
			{
				Severity: SevWarn,
			},
		},
	}
	require.Equal(t, "linting error found 1 instance(s)", lintErr.Error())
	require.True(t, lintErr.OnlyWarnings())

	lintErr = &LintError{
		Findings: []PackageFinding{
			{
				Severity: SevWarn,
			},
			{
				Severity: SevErr,
			},
		},
	}
	require.Equal(t, "linting error found 2 instance(s)", lintErr.Error())
	require.False(t, lintErr.OnlyWarnings())
}
