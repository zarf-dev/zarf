// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"testing"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
)

func TestGroupFindingsByPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		findings    []types.PackageFinding
		severity    types.Severity
		packageName string
		want        map[string][]types.PackageFinding
	}{
		{
			name: "same package multiple findings",
			findings: []types.PackageFinding{
				{Severity: types.SevWarn, PackageNameOverride: "import", PackagePathOverride: "path"},
				{Severity: types.SevWarn, PackageNameOverride: "import", PackagePathOverride: "path"},
			},
			severity:    types.SevWarn,
			packageName: "testPackage",
			want: map[string][]types.PackageFinding{
				"path": {
					{Severity: types.SevWarn, PackageNameOverride: "import", PackagePathOverride: "path"},
					{Severity: types.SevWarn, PackageNameOverride: "import", PackagePathOverride: "path"},
				},
			},
		},
		{
			name: "different packages single finding",
			findings: []types.PackageFinding{
				{Severity: types.SevWarn, PackageNameOverride: "import", PackagePathOverride: "path"},
				{Severity: types.SevErr, PackageNameOverride: "", PackagePathOverride: ""},
			},
			severity:    types.SevWarn,
			packageName: "testPackage",
			want: map[string][]types.PackageFinding{
				"path": {{Severity: types.SevWarn, PackageNameOverride: "import", PackagePathOverride: "path"}},
				".":    {{Severity: types.SevErr, PackageNameOverride: "testPackage", PackagePathOverride: "."}},
			},
		},
		{
			name: "Multiple findings, mixed severity",
			findings: []types.PackageFinding{
				{Severity: types.SevWarn, PackageNameOverride: "", PackagePathOverride: ""},
				{Severity: types.SevErr, PackageNameOverride: "", PackagePathOverride: ""},
			},
			severity:    types.SevErr,
			packageName: "testPackage",
			want: map[string][]types.PackageFinding{
				".": {{Severity: types.SevErr, PackageNameOverride: "testPackage", PackagePathOverride: "."}},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, GroupFindingsByPath(tt.findings, tt.severity, tt.packageName))
		})
	}
}

func TestHasSeverity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		severity types.Severity
		expected bool
		findings []types.PackageFinding
	}{
		{
			name: "error severity present",
			findings: []types.PackageFinding{
				{
					Severity: types.SevErr,
				},
			},
			severity: types.SevErr,
			expected: true,
		},
		{
			name: "error severity not present",
			findings: []types.PackageFinding{
				{
					Severity: types.SevWarn,
				},
			},
			severity: types.SevErr,
			expected: false,
		},
		{
			name: "err and warning severity present",
			findings: []types.PackageFinding{
				{
					Severity: types.SevWarn,
				},
				{
					Severity: types.SevErr,
				},
			},
			severity: types.SevErr,
			expected: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, HasSeverity(tt.findings, tt.severity))
		})
	}
}
