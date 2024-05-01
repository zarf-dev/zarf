// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package migrations

import (
	"fmt"
	"slices"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

// DefaultRequired migrates the package to change components to be required by default
type DefaultRequired struct{}

// String returns the name of the migration
func (DefaultRequired) String() string {
	return string(types.DefaultRequired)
}

// Run sets all components to be required by default
//
// and cleanly migrates components explicitly marked as required to be nil
func (DefaultRequired) Run(pkg types.ZarfPackage) (types.ZarfPackage, string) {
	if slices.Contains(pkg.Metadata.Features, types.DefaultRequired) {
		return pkg, fmt.Sprintf("%s feature flag already enabled", types.DefaultRequired)
	}

	pkg.Metadata.Features = append(pkg.Metadata.Features, types.DefaultRequired)

	for idx, component := range pkg.Components {
		if component.Required != nil && *component.Required {
			pkg.Components[idx].Required = nil
		}
		if component.Required == nil {
			pkg.Components[idx].Required = helpers.BoolPtr(false)
		}
	}

	return pkg, ""
}
