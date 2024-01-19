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
type PackageCreator struct{}

// LoadPackageDefinition loads and configures a zarf.yaml file during package create.
func (pc *PackageCreator) LoadPackageDefinition(pkg *types.ZarfPackage, createOpts *types.ZarfCreateOptions, layout *layout.PackagePaths) (loadedPkg *types.ZarfPackage, warnings []string, err error) {
	pkg, err = setPackageMetadata(pkg, createOpts)
	if err != nil {
		message.Warn(err.Error())
	}

	// Compose components into a single zarf.yaml file
	pkg, composeWarnings, err := ComposeComponents(pkg, createOpts)
	if err != nil {
		return nil, nil, err
	}

	warnings = append(warnings, composeWarnings...)

	// After components are composed, template the active package.
	templateWarnings, err := FillActiveTemplate(pkg, createOpts)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to fill values in template: %w", err)
	}

	warnings = append(warnings, templateWarnings...)

	// After templates are filled process any create extensions
	pkg, err = ProcessExtensions(pkg, createOpts, layout)
	if err != nil {
		return nil, nil, err
	}

	// If we are creating a differential package, remove duplicate images and repos.
	if pkg.Build.Differential {
		diffData, err := LoadDifferentialData(&createOpts.DifferentialData)
		if err != nil {
			return nil, nil, err
		}

		if createOpts.DifferentialData.DifferentialPackageVersion == pkg.Metadata.Version {
			return nil, nil, errors.New(lang.PkgCreateErrDifferentialSameVersion)
		}
		if createOpts.DifferentialData.DifferentialPackageVersion == "" || pkg.Metadata.Version == "" {
			return nil, nil, fmt.Errorf("unable to build differential package when either the differential package version or the referenced package version is not set")
		}

		pkg, err = RemoveCopiesFromDifferentialPackage(pkg, diffData)
		if err != nil {
			return nil, nil, err
		}
	}

	return pkg, warnings, nil
}
