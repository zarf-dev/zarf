// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package validate

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
)

func TestValidateOperationRequirements(t *testing.T) {
	// Save original CLI version and restore after test
	originalVersion := config.CLIVersion
	defer func() {
		config.CLIVersion = originalVersion
	}()

	tests := []struct {
		name        string
		pkg         v1alpha1.ZarfPackage
		operation   v1alpha1.PackageOperation
		cliVersion  string
		expectError bool
	}{
		{
			name: "no requirements",
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					OperationRequirements: []v1alpha1.OperationRequirement{},
				},
			},
			operation:   v1alpha1.OperationDeploy,
			cliVersion:  "v0.64.0",
			expectError: false,
		},
		{
			name: "requirement met for deploy",
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					OperationRequirements: []v1alpha1.OperationRequirement{
						{
							Version:    "v0.65.0",
							Operations: []v1alpha1.PackageOperation{v1alpha1.OperationDeploy},
							Reason:     "values field requires v0.65.0+",
						},
					},
				},
			},
			operation:   v1alpha1.OperationDeploy,
			cliVersion:  "v0.66.0",
			expectError: false,
		},
		{
			name: "requirement not met for deploy",
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					OperationRequirements: []v1alpha1.OperationRequirement{
						{
							Version:    "v0.65.0",
							Operations: []v1alpha1.PackageOperation{v1alpha1.OperationDeploy},
							Reason:     "values field requires v0.65.0+",
						},
					},
				},
			},
			operation:   v1alpha1.OperationDeploy,
			cliVersion:  "v0.64.0",
			expectError: true,
		},
		{
			name: "requirement applies to all operations",
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					OperationRequirements: []v1alpha1.OperationRequirement{
						{
							Version:    "v0.65.0",
							Operations: []v1alpha1.PackageOperation{}, // Empty means applies to all
							Reason:     "package structure changed",
						},
					},
				},
			},
			operation:   v1alpha1.OperationPublish,
			cliVersion:  "v0.64.0",
			expectError: true,
		},
		{
			name: "requirement applies to different operation",
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					OperationRequirements: []v1alpha1.OperationRequirement{
						{
							Version:    "v0.65.0",
							Operations: []v1alpha1.PackageOperation{v1alpha1.OperationDeploy},
							Reason:     "deploy-specific requirement",
						},
					},
				},
			},
			operation:   v1alpha1.OperationPublish,
			cliVersion:  "v0.64.0",
			expectError: false,
		},
		{
			name: "multiple requirements with one not met",
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					OperationRequirements: []v1alpha1.OperationRequirement{
						{
							Version:    "v0.60.0",
							Operations: []v1alpha1.PackageOperation{v1alpha1.OperationDeploy},
							Reason:     "older requirement",
						},
						{
							Version:    "v0.70.0",
							Operations: []v1alpha1.PackageOperation{v1alpha1.OperationDeploy},
							Reason:     "newer requirement",
						},
					},
				},
			},
			operation:   v1alpha1.OperationDeploy,
			cliVersion:  "v0.65.0",
			expectError: true,
		},
		{
			name: "development version skips validation",
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					OperationRequirements: []v1alpha1.OperationRequirement{
						{
							Version:    "v0.65.0",
							Operations: []v1alpha1.PackageOperation{v1alpha1.OperationDeploy},
							Reason:     "should be skipped in dev mode",
						},
					},
				},
			},
			operation:   v1alpha1.OperationDeploy,
			cliVersion:  config.UnsetCLIVersion,
			expectError: false,
		},
		{
			name: "requirement met at exact version",
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					OperationRequirements: []v1alpha1.OperationRequirement{
						{
							Version:    "v0.65.0",
							Operations: []v1alpha1.PackageOperation{v1alpha1.OperationDeploy},
							Reason:     "exact version match",
						},
					},
				},
			},
			operation:   v1alpha1.OperationDeploy,
			cliVersion:  "v0.65.0",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.CLIVersion = tt.cliVersion
			err := ValidateOperationRequirements(tt.pkg, tt.operation)
			if tt.expectError {
				require.Error(t, err)
				// Verify it's the right type of error
				var orErr *OperationRequirementsError
				require.ErrorAs(t, err, &orErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestOperationRequirementsError(t *testing.T) {
	err := &OperationRequirementsError{
		Requirements: []v1alpha1.OperationRequirement{
			{
				Version:    "v0.65.0",
				Operations: []v1alpha1.PackageOperation{v1alpha1.OperationDeploy},
				Reason:     "values field requires v0.65.0+",
			},
		},
		CurrentVersion: "v0.64.0",
		Operation:      v1alpha1.OperationDeploy,
	}

	errMsg := err.Error()
	require.Contains(t, errMsg, "v0.64.0")
	require.Contains(t, errMsg, "v0.65.0")
	require.Contains(t, errMsg, "values field requires v0.65.0+")
	require.Contains(t, errMsg, "deploy")
}
