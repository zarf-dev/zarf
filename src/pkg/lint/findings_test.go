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

func TestGroupFindingsByPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		findings    []PackageFinding
		severity    Severity
		packageName string
		want        map[string][]PackageFinding
	}{
		{
			name: "same package multiple findings",
			findings: []PackageFinding{
				{Severity: SevWarn, PackageNameOverride: "import", PackagePathOverride: "path"},
				{Severity: SevWarn, PackageNameOverride: "import", PackagePathOverride: "path"},
			},
			packageName: "testPackage",
			want: map[string][]PackageFinding{
				"path": {
					{Severity: SevWarn, PackageNameOverride: "import", PackagePathOverride: "path"},
					{Severity: SevWarn, PackageNameOverride: "import", PackagePathOverride: "path"},
				},
			},
		},
		{
			name: "different packages single finding",
			findings: []PackageFinding{
				{Severity: SevWarn, PackageNameOverride: "import", PackagePathOverride: "path"},
				{Severity: SevErr, PackageNameOverride: "", PackagePathOverride: ""},
			},
			packageName: "testPackage",
			want: map[string][]PackageFinding{
				"path": {{Severity: SevWarn, PackageNameOverride: "import", PackagePathOverride: "path"}},
				".":    {{Severity: SevErr, PackageNameOverride: "testPackage", PackagePathOverride: "."}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, GroupFindingsByPath(tt.findings, tt.packageName))
		})
	}
}
