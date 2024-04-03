// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package migrations

import (
	"slices"

	"github.com/defenseunicorns/zarf/src/types"
)

// DefaultRequired migrates the package to change components to be required by default
type DefaultRequired struct{}

// ID returns the ID of the migration
func (DefaultRequired) ID() string {
	return string(types.DefaultRequired)
}

// Run sets all components to be required by default
//
// and cleanly migrates components explicitly marked as required to be nil
func (DefaultRequired) Run(pkg types.ZarfPackage) types.ZarfPackage {
	for idx, component := range pkg.Components {
		if component.Required != nil && *component.Required {
			pkg.Components[idx].Required = nil
		}
	}

	if !slices.Contains(pkg.Metadata.BetaFeatures, types.DefaultRequired) {
		pkg.Metadata.BetaFeatures = append(pkg.Metadata.BetaFeatures, types.DefaultRequired)
	}

	return pkg
}
