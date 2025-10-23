// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package validate provides validation functions for package operations
package validate

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
)

// OperationRequirementsError is returned when operational requirements are not met
type OperationRequirementsError struct {
	Requirements   []v1alpha1.OperationRequirement
	CurrentVersion string
}

func (e *OperationRequirementsError) Error() string {
	// Find the highest version requirement
	highestVersion := e.Requirements[0].Version
	var highestSemver *semver.Version

	for _, req := range e.Requirements {
		if v, err := semver.NewVersion(req.Version); err == nil {
			if highestSemver == nil || v.GreaterThan(highestSemver) {
				highestSemver = v
				highestVersion = req.Version
			}
		}
	}

	msg := fmt.Sprintf("package requires Zarf version '%s' (current version: '%s'):\n",
		highestVersion, e.CurrentVersion)
	for _, req := range e.Requirements {
		if req.Reason != "" {
			msg += fmt.Sprintf("  Reason: %s\n", req.Reason)
		}
	}
	return msg
}

// ValidateOperationRequirements checks if the config.CLIVersion meets the operational requirements.
func ValidateOperationRequirements(pkg v1alpha1.ZarfPackage) error {
	if len(pkg.Build.OperationRequirements) == 0 {
		return nil
	}

	currentVersion := config.CLIVersion
	if currentVersion == config.UnsetCLIVersion {
		return nil
	}

	// Parse current CLI version
	currentSemver, err := semver.NewVersion(currentVersion)
	if err != nil {
		return fmt.Errorf("failed to parse current Zarf version '%s': %w", currentVersion, err)
	}

	var unmetRequirements []v1alpha1.OperationRequirement

	for _, req := range pkg.Build.OperationRequirements {
		// Parse required version
		requiredSemver, err := semver.NewVersion(req.Version)
		if err != nil {
			return fmt.Errorf("failed to parse required version '%s': %w", req.Version, err)
		}

		// Check if current version meets the requirement
		if currentSemver.LessThan(requiredSemver) {
			unmetRequirements = append(unmetRequirements, req)
		}
	}

	if len(unmetRequirements) > 0 {
		return &OperationRequirementsError{
			Requirements:   unmetRequirements,
			CurrentVersion: currentVersion,
		}
	}

	return nil
}
