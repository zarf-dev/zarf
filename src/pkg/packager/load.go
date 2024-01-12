// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"errors"
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

var (
	// verify that SkeletonLoader implements Loader
	_ Loader = (*SkeletonLoader)(nil)

	// verify that PackageLoader implements Loader
	_ Loader = (*PackageLoader)(nil)
)

// Loader is an interface for loading and configuring package definitions during package create.
type Loader interface {
	LoadPackageDefinition(*Packager) error
}

// SkeletonLoader is used to load and configure skeleton Zarf packages during package create.
type SkeletonLoader struct{}

// LoadPackageDefinition loads and configures skeleton Zarf packages during package create.
func (sl *SkeletonLoader) LoadPackageDefinition(pkgr *Packager) error {
	if err := utils.ReadYaml(layout.ZarfYAML, &pkgr.cfg.Pkg); err != nil {
		return err
	}

	pkgr.arch = config.GetArch(pkgr.cfg.Pkg.Metadata.Architecture, pkgr.cfg.Pkg.Build.Architecture)

	if pkgr.isInitConfig() {
		pkgr.cfg.Pkg.Metadata.Version = config.CLIVersion
	}

	// Compose components into a single zarf.yaml file
	return pkgr.composeComponents()
}

// PackageLoader is used to load and configure normal (not skeleton) Zarf packages during package create.
type PackageLoader struct{}

// LoadPackageDefinition loads and configures normal (not skeleton) Zarf packages during package create.
func (pl *PackageLoader) LoadPackageDefinition(pkgr *Packager) error {
	if err := utils.ReadYaml(layout.ZarfYAML, &pkgr.cfg.Pkg); err != nil {
		return err
	}
	pkgr.arch = config.GetArch(pkgr.cfg.Pkg.Metadata.Architecture, pkgr.cfg.Pkg.Build.Architecture)

	if pkgr.isInitConfig() {
		pkgr.cfg.Pkg.Metadata.Version = config.CLIVersion
	}

	// Compose components into a single zarf.yaml file
	if err := pkgr.composeComponents(); err != nil {
		return err
	}

	// After components are composed, template the active package.
	if err := pkgr.fillActiveTemplate(); err != nil {
		return fmt.Errorf("unable to fill values in template: %s", err.Error())
	}

	// After templates are filled process any create extensions
	if err := pkgr.processExtensions(); err != nil {
		return err
	}

	// After we have a full zarf.yaml remove unnecessary repos and images if we are building a differential package
	if pkgr.cfg.CreateOpts.DifferentialData.DifferentialPackagePath != "" {
		// Load the images and repos from the 'reference' package
		if err := pkgr.loadDifferentialData(); err != nil {
			return err
		}
		// Verify the package version of the package we're using as a 'reference' for the differential build is different than the package we're building
		// If the package versions are the same return an error
		if pkgr.cfg.CreateOpts.DifferentialData.DifferentialPackageVersion == pkgr.cfg.Pkg.Metadata.Version {
			return errors.New(lang.PkgCreateErrDifferentialSameVersion)
		}
		if pkgr.cfg.CreateOpts.DifferentialData.DifferentialPackageVersion == "" || pkgr.cfg.Pkg.Metadata.Version == "" {
			return fmt.Errorf("unable to build differential package when either the differential package version or the referenced package version is not set")
		}

		// Handle any potential differential images/repos before going forward
		if err := pkgr.removeCopiesFromDifferentialPackage(); err != nil {
			return err
		}
	}

	return nil
}