// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package validate provides validation functions for package operations
package validate

import (
	"fmt"
	"slices"

	"github.com/Masterminds/semver/v3"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
)

// OperationRequirementsError is returned when operational requirements are not met
type OperationRequirementsError struct {
	Requirements   []v1alpha1.OperationRequirement
	CurrentVersion string
	Operation      v1alpha1.PackageOperation
}

func (e *OperationRequirementsError) Error() string {
	msg := fmt.Sprintf("package requires Zarf CLI version requirements for operation '%s' that are not met by current version '%s':\n",
		e.Operation, e.CurrentVersion)
	for _, req := range e.Requirements {
		msg += fmt.Sprintf("  - Required version: %s", req.Version)
		if req.Reason != "" {
			msg += fmt.Sprintf(" (Reason: %s)", req.Reason)
		}
		msg += "\n"
	}
	return msg
}

// ValidateOperationRequirements checks if the current Zarf CLI version meets the operational requirements
// for a given operation. Returns an error if requirements are not met.
func ValidateOperationRequirements(pkg v1alpha1.ZarfPackage, operation v1alpha1.PackageOperation) error {
	if len(pkg.Build.OperationRequirements) == 0 {
		return nil
	}

	currentVersion := config.CLIVersion
	if currentVersion == config.UnsetCLIVersion {
		// In development mode, skip version validation
		return nil
	}

	// Parse current CLI version
	currentSemver, err := semver.NewVersion(currentVersion)
	if err != nil {
		return fmt.Errorf("failed to parse current Zarf version '%s': %w", currentVersion, err)
	}

	var unmetRequirements []v1alpha1.OperationRequirement

	for _, req := range pkg.Build.OperationRequirements {
		// If Operations is empty, requirement applies to all operations
		// Otherwise, check if current operation is in the list
		appliesToOperation := len(req.Operations) == 0
		if !appliesToOperation {
			appliesToOperation = slices.Contains(req.Operations, operation)
		}

		if !appliesToOperation {
			continue
		}

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
			Operation:      operation,
		}
	}

	return nil
}
