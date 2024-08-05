// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package rules checks Zarf packages and reports any findings or errors
package rules

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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

func TestHasSeverity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		severity Severity
		expected bool
		findings []PackageFinding
	}{
		{
			name: "error severity present",
			findings: []PackageFinding{
				{
					Severity: SevErr,
				},
			},
			severity: SevErr,
			expected: true,
		},
		{
			name: "error severity not present",
			findings: []PackageFinding{
				{
					Severity: SevWarn,
				},
			},
			severity: SevErr,
			expected: false,
		},
		{
			name: "err and warning severity present",
			findings: []PackageFinding{
				{
					Severity: SevWarn,
				},
				{
					Severity: SevErr,
				},
			},
			severity: SevErr,
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, HasSevOrHigher(tt.findings, tt.severity))
		})
	}
}
