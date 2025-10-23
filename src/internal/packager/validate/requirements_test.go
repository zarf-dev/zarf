// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package validate

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
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
		pkg         v1alpha1.ZarfPackage
		cliVersion  string
		expectError bool
	}{
		{
			name: "no requirements",
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					VersionRequirements: []v1alpha1.VersionRequirement{},
				},
			},
			cliVersion:  "v0.64.0",
			expectError: false,
		},
		{
			name: "requirement met",
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					VersionRequirements: []v1alpha1.VersionRequirement{
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
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					VersionRequirements: []v1alpha1.VersionRequirement{
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
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					VersionRequirements: []v1alpha1.VersionRequirement{
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
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					VersionRequirements: []v1alpha1.VersionRequirement{
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
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					VersionRequirements: []v1alpha1.VersionRequirement{
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
			err := ValidateVersionRequirements(tt.pkg)
			if tt.expectError {
				require.Error(t, err)
				// Verify it's the right type of error
				var orErr *VersionRequirementsError
				require.ErrorAs(t, err, &orErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestVersionRequirementsError(t *testing.T) {
	err := &VersionRequirementsError{
		Requirements: []v1alpha1.VersionRequirement{
			{
				Version: "v0.65.0",
				Reason:  "values field requires v0.65.0+",
			},
		},
		CurrentVersion: "v0.64.0",
	}

	errMsg := err.Error()
	require.Contains(t, errMsg, "v0.64.0")
	require.Contains(t, errMsg, "v0.65.0")
	require.Contains(t, errMsg, "values field requires v0.65.0+")
}

func TestVersionRequirementsError_HighestVersion(t *testing.T) {
	err := &VersionRequirementsError{
		Requirements: []v1alpha1.VersionRequirement{
			{
				Version: "v0.60.0",
				Reason:  "older requirement",
			},
			{
				Version: "v0.70.0",
				Reason:  "newer requirement",
			},
			{
				Version: "v0.65.0",
				Reason:  "middle requirement",
			},
		},
		CurrentVersion: "v0.64.0",
	}

	errMsg := err.Error()
	// Should display the highest version (v0.70.0)
	require.Contains(t, errMsg, "v0.70.0")
	require.Contains(t, errMsg, "v0.64.0")
	// Should include all reasons
	require.Contains(t, errMsg, "older requirement")
	require.Contains(t, errMsg, "newer requirement")
	require.Contains(t, errMsg, "middle requirement")
}
