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
func (*SkeletonLoader) LoadPackageDefinition(p *Packager) error {
	if err := utils.ReadYaml(layout.ZarfYAML, &p.cfg.Pkg); err != nil {
		return err
	}

	p.arch = config.GetArch(p.cfg.Pkg.Metadata.Architecture, p.cfg.Pkg.Build.Architecture)

	if p.isInitConfig() {
		p.cfg.Pkg.Metadata.Version = config.CLIVersion
	}

	// Compose components into a single zarf.yaml file
	return p.composeComponents()
}

// PackageLoader is used to load and configure normal (not skeleton) Zarf packages during package create.
type PackageLoader struct{}

// LoadPackageDefinition loads and configures normal (not skeleton) Zarf packages during package create.
func (*PackageLoader) LoadPackageDefinition(p *Packager) error {
	if err := utils.ReadYaml(layout.ZarfYAML, &p.cfg.Pkg); err != nil {
		return err
	}
	p.arch = config.GetArch(p.cfg.Pkg.Metadata.Architecture, p.cfg.Pkg.Build.Architecture)

	if p.isInitConfig() {
		p.cfg.Pkg.Metadata.Version = config.CLIVersion
	}

	// Compose components into a single zarf.yaml file
	if err := p.composeComponents(); err != nil {
		return err
	}

	// After components are composed, template the active package.
	if err := p.fillActiveTemplate(); err != nil {
		return fmt.Errorf("unable to fill values in template: %s", err.Error())
	}

	// After templates are filled process any create extensions
	if err := p.processExtensions(); err != nil {
		return err
	}

	// If we are building a differential package, remove duplicate repos and images.
	if p.cfg.CreateOpts.DifferentialData.DifferentialPackagePath != "" {
		if err := p.loadDifferentialData(); err != nil {
			return err
		}
		versionsMatch := p.cfg.CreateOpts.DifferentialData.DifferentialPackageVersion == p.cfg.Pkg.Metadata.Version
		if versionsMatch {
			return errors.New(lang.PkgCreateErrDifferentialSameVersion)
		}
		noVersionSet := p.cfg.CreateOpts.DifferentialData.DifferentialPackageVersion == "" || p.cfg.Pkg.Metadata.Version == ""
		if noVersionSet {
			return errors.New(lang.PkgCreateErrDifferentialNoVersion)
		}
		return p.removeCopiesFromDifferentialPackage()
	}

	return nil
}
