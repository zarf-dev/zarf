// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package requirements

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/config"
)

func TestValidateVersionRequirements(t *testing.T) {
	// Save original CLI version and restore after test
	originalVersion := config.CLIVersion
	defer func() {
		config.CLIVersion = originalVersion
	}()

	tests := []struct {
		name        string
		pkg         api.Package
		cliVersion  string
		expectError bool
	}{
		{
			name: "no requirements",
			pkg: api.Package{
				Build: api.BuildData{
					VersionRequirements: []api.VersionRequirement{},
				},
			},
			cliVersion:  "v0.64.0",
			expectError: false,
		},
		{
			name: "requirement met",
			pkg: api.Package{
				Build: api.BuildData{
					VersionRequirements: []api.VersionRequirement{
						{
							Version: "v0.65.0",
							Reason:  "values field requires v0.65.0+",
						},
					},
				},
			},
			cliVersion:  "v0.66.0",
			expectError: false,
		},
		{
			name: "requirement not met",
			pkg: api.Package{
				Build: api.BuildData{
					VersionRequirements: []api.VersionRequirement{
						{
							Version: "v0.65.0",
							Reason:  "values field requires v0.65.0+",
						},
					},
				},
			},
			cliVersion:  "v0.64.0",
			expectError: true,
		},
		{
			name: "multiple requirements with one not met",
			pkg: api.Package{
				Build: api.BuildData{
					VersionRequirements: []api.VersionRequirement{
						{
							Version: "v0.60.0",
							Reason:  "older requirement",
						},
						{
							Version: "v0.70.0",
							Reason:  "newer requirement",
						},
					},
				},
			},
			cliVersion:  "v0.65.0",
			expectError: true,
		},
		{
			name: "development version skips validation",
			pkg: api.Package{
				Build: api.BuildData{
					VersionRequirements: []api.VersionRequirement{
						{
							Version: "v0.65.0",
							Reason:  "should be skipped in dev mode",
						},
					},
				},
			},
			cliVersion:  config.UnsetCLIVersion,
			expectError: false,
		},
		{
			name: "requirement met at exact version",
			pkg: api.Package{
				Build: api.BuildData{
					VersionRequirements: []api.VersionRequirement{
						{
							Version: "v0.65.0",
							Reason:  "exact version match",
						},
					},
				},
			},
			cliVersion:  "v0.65.0",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.CLIVersion = tt.cliVersion
			err := ValidateVersionRequirements(tt.pkg.Build.VersionRequirements)
			if tt.expectError {
				var orErr *VersionRequirementsError
				require.ErrorAs(t, err, &orErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateVersionRequirements_ReleaseCandidate(t *testing.T) {
	originalVersion := config.CLIVersion
	t.Cleanup(func() { config.CLIVersion = originalVersion })

	tests := []struct {
		name        string
		cliVersion  string
		required    string
		expectError bool
	}{
		{name: "matching release", cliVersion: "v0.88.0-rc1", required: "v0.88.0"},
		{name: "dotted release candidate", cliVersion: "v0.88.0-rc.1", required: "v0.88.0"},
		{name: "build metadata", cliVersion: "v0.88.0-rc1+build.123", required: "v0.88.0"},
		{name: "matching patch release", cliVersion: "v0.88.1-rc1", required: "v0.88.1"},
		{name: "older requirement", cliVersion: "v0.88.0-rc1", required: "v0.87.0"},
		{name: "newer patch required", cliVersion: "v0.88.0-rc1", required: "v0.88.1", expectError: true},
		{name: "newer minor required", cliVersion: "v0.88.0-rc1", required: "v0.89.0", expectError: true},
		{name: "newer major required", cliVersion: "v0.88.0-rc1", required: "v1.88.0", expectError: true},
		{name: "exact release candidate", cliVersion: "v0.88.0-rc.1", required: "v0.88.0-rc.1"},
		{name: "later release candidate", cliVersion: "v0.88.0-rc.2", required: "v0.88.0-rc.1"},
		{name: "earlier release candidate", cliVersion: "v0.88.0-rc.1", required: "v0.88.0-rc.2", expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.CLIVersion = tt.cliVersion
			reqs := []api.VersionRequirement{{Version: tt.required, Reason: "required feature"}}
			err := ValidateVersionRequirements(reqs)
			if tt.expectError {
				var requirementErr *VersionRequirementsError
				require.ErrorAs(t, err, &requirementErr)
				require.Equal(t, tt.required, requirementErr.RequiredVersion)
				require.Equal(t, tt.cliVersion, requirementErr.CurrentVersion)
				require.Equal(t, reqs, requirementErr.Requirements)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestVersionRequirementsError_HighestVersion(t *testing.T) {
	tests := []struct {
		name         string
		requirements []api.VersionRequirement
		expected     string
		expectError  bool
	}{
		{
			name: "multiple versions",
			requirements: []api.VersionRequirement{
				{
					Version: "v0.70.0",
					Reason:  "newer requirement",
				},
				{
					Version: "v0.65.0",
					Reason:  "older requirement",
				},
			},
			expected:    "v0.70.0",
			expectError: false,
		},
		{
			name: "single version",
			requirements: []api.VersionRequirement{
				{
					Version: "v0.65.0",
					Reason:  "single requirement",
				},
			},
			expected:    "v0.65.0",
			expectError: false,
		},
		{
			name:         "empty requirements",
			requirements: []api.VersionRequirement{},
			expected:     "",
			expectError:  false,
		},
		{
			name: "invalid version",
			requirements: []api.VersionRequirement{
				{
					Version: "invalid",
					Reason:  "bad version",
				},
			},
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := calculateRequiredVersion(tt.requirements)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}
