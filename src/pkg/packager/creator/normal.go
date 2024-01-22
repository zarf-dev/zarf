// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"errors"
	"fmt"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
)

var (
	// veryify that PackageCreator implements Creator
	_ Creator = (*PackageCreator)(nil)
)

// PackageCreator provides methods for creating normal (not skeleton) Zarf packages.
type PackageCreator struct {
	pkg        *types.ZarfPackage
	createOpts *types.ZarfCreateOptions
	layout     *layout.PackagePaths
}

// LoadPackageDefinition loads and configures a zarf.yaml file during package create.
func (pc *PackageCreator) LoadPackageDefinition() (pkg *types.ZarfPackage, warnings []string, err error) {
	configuredPkg, err := setPackageMetadata(pc.pkg, pc.createOpts)
	if err != nil {
		message.Warn(err.Error())
	}

	// Compose components into a single zarf.yaml file
	composedPkg, composeWarnings, err := ComposeComponents(configuredPkg, pc.createOpts)
	if err != nil {
		return nil, nil, err
	}

	warnings = append(warnings, composeWarnings...)

	// After components are composed, template the active package.
	templateWarnings, err := FillActiveTemplate(composedPkg, pc.createOpts)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to fill values in template: %w", err)
	}

	warnings = append(warnings, templateWarnings...)

	// After templates are filled process any create extensions
	extendedPkg, err := ProcessExtensions(composedPkg, pc.createOpts, pc.layout)
	if err != nil {
		return nil, nil, err
	}

	// If we are creating a differential package, remove duplicate images and repos.
	if pc.pkg.Build.Differential {
		diffData, err := loadDifferentialData(&pc.createOpts.DifferentialData)
		if err != nil {
			return nil, nil, err
		}

		if pc.createOpts.DifferentialData.DifferentialPackageVersion == pc.pkg.Metadata.Version {
			return nil, nil, errors.New(lang.PkgCreateErrDifferentialSameVersion)
		}
		if pc.createOpts.DifferentialData.DifferentialPackageVersion == "" || pc.pkg.Metadata.Version == "" {
			return nil, nil, fmt.Errorf("unable to build differential package when either the differential package version or the referenced package version is not set")
		}

		diffPkg, err := removeCopiesFromDifferentialPackage(extendedPkg, diffData)
		if err != nil {
			return nil, nil, err
		}
		return diffPkg, nil, nil
	}

	return extendedPkg, warnings, nil
}
