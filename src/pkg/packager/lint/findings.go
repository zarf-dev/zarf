// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/types"
)

// GroupFindingsByPath groups findings by their package path
func GroupFindingsByPath(findings []types.PackageFinding, severity types.Severity, packageName string) map[string][]types.PackageFinding {
	findings = helpers.RemoveMatches(findings, func(finding types.PackageFinding) bool {
		return finding.Severity > severity
	})
	for i := range findings {
		if findings[i].PackageNameOverride == "" {
			findings[i].PackageNameOverride = packageName
		}
		if findings[i].PackagePathOverride == "" {
			findings[i].PackagePathOverride = "."
		}
	}

	mapOfFindingsByPath := make(map[string][]types.PackageFinding)
	for _, finding := range findings {
		mapOfFindingsByPath[finding.PackagePathOverride] = append(mapOfFindingsByPath[finding.PackagePathOverride], finding)
	}
	return mapOfFindingsByPath
}

// HasSeverity returns true if the findings contain a severity equal to or greater than the given severity
func HasSeverity(findings []types.PackageFinding, severity types.Severity) bool {
	for _, finding := range findings {
		if finding.Severity <= severity {
			return true
		}
	}
	return false
}
