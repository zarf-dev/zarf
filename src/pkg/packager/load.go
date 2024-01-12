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

// LoadPackageDefinition loads and configures normal (not skeleton) Zarf packages during package create.
func (*PackageCreator) LoadPackageDefinition(p *Packager) error {
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

// LoadPackageDefinition loads and configures skeleton Zarf packages during package create.
func (*SkeletonCreator) LoadPackageDefinition(p *Packager) error {
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
