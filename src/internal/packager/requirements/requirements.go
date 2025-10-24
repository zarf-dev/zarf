// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package requirements validates that Zarf meets the version requirements defined by the package
package requirements

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
)

// VersionRequirementsError is returned when operational requirements are not met
type VersionRequirementsError struct {
	RequiredVersion string
	Requirements    []v1alpha1.VersionRequirement
	CurrentVersion  string
}

func (e *VersionRequirementsError) Error() string {
	msg := fmt.Sprintf("package requires Zarf version '%s' (current version: '%s'):\n",
		e.RequiredVersion, e.CurrentVersion)
	for _, req := range e.Requirements {
		if req.Reason != "" {
			msg += fmt.Sprintf("Reason: %s\n", req.Reason)
		}
	}
	return msg
}

// ValidateVersionRequirements checks if the config.CLIVersion meets the operational requirements.
func ValidateVersionRequirements(pkg v1alpha1.ZarfPackage) error {
	if len(pkg.Build.VersionRequirements) == 0 {
		return nil
	}

	currentVersion := config.CLIVersion
	if currentVersion == config.UnsetCLIVersion {
		return nil
	}

	currentVer, err := semver.NewVersion(currentVersion)
	if err != nil {
		return fmt.Errorf("failed to parse current Zarf version '%s': %w", currentVersion, err)
	}

	var unmetRequirements []v1alpha1.VersionRequirement

	for _, req := range pkg.Build.VersionRequirements {
		requiredVer, err := semver.NewVersion(req.Version)
		if err != nil {
			return fmt.Errorf("failed to parse required version '%s': %w", req.Version, err)
		}

		if currentVer.LessThan(requiredVer) {
			unmetRequirements = append(unmetRequirements, req)
		}
	}

	// Find the highest version requirement
	highestVersion := unmetRequirements[0].Version
	var highestSemver *semver.Version

	for _, req := range unmetRequirements {
		if v, err := semver.NewVersion(req.Version); err == nil {
			if highestSemver == nil || v.GreaterThan(highestSemver) {
				highestSemver = v
				highestVersion = req.Version
			}
		}
	}

	if len(unmetRequirements) > 0 {
		return &VersionRequirementsError{
			RequiredVersion: highestVersion,
			Requirements:    unmetRequirements,
			CurrentVersion:  currentVersion,
		}
	}

	return nil
}
