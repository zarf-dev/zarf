// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package requirements validates minimum Zarf versions declared by packages and components.
package requirements

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/config"
)

// VersionRequirementsError is returned when operational requirements are not met
type VersionRequirementsError struct {
	RequiredVersion string
	Requirements    []api.VersionRequirement
	CurrentVersion  string
}

func (e *VersionRequirementsError) Error() string {
	msg := fmt.Sprintf("Zarf version '%s' is required (current version: '%s'):\n",
		e.RequiredVersion, e.CurrentVersion)
	for _, req := range e.Requirements {
		if req.Reason != "" {
			msg += fmt.Sprintf("Reason: %s\n", req.Reason)
		}
	}
	return msg
}

// calculateRequiredVersion finds the highest version from a list of version requirements
func calculateRequiredVersion(requirements []api.VersionRequirement) (string, error) {
	if len(requirements) == 0 {
		return "", nil
	}

	highestVersion := requirements[0].Version
	highestSemver, err := semver.NewVersion(highestVersion)
	if err != nil {
		return "", err
	}

	for _, req := range requirements[1:] {
		v, err := semver.NewVersion(req.Version)
		if err != nil {
			return "", err
		}
		if v.GreaterThan(highestSemver) {
			highestSemver = v
			highestVersion = req.Version
		}
	}

	return highestVersion, nil
}

// ValidateVersionRequirements checks if config.CLIVersion meets the given minimum versions.
func ValidateVersionRequirements(requirements []api.VersionRequirement) error {
	if len(requirements) == 0 {
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

	var unmetRequirements []api.VersionRequirement

	for _, req := range requirements {
		requiredVer, err := semver.NewVersion(req.Version)
		if err != nil {
			return fmt.Errorf("failed to parse required version '%s': %w", req.Version, err)
		}

		if currentVer.LessThan(requiredVer) {
			unmetRequirements = append(unmetRequirements, req)
		}
	}

	if len(unmetRequirements) == 0 {
		return nil
	}

	// Find the highest version requirement
	highestVersion, err := calculateRequiredVersion(unmetRequirements)
	if err != nil {
		return err
	}

	return &VersionRequirementsError{
		RequiredVersion: highestVersion,
		Requirements:    unmetRequirements,
		CurrentVersion:  currentVersion,
	}
}
