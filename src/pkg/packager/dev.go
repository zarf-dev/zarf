// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/filters"
	"github.com/defenseunicorns/zarf/src/types"
)

// DevDeploy creates + deploys a package in one shot
func (p *Packager) DevDeploy() error {
	config.CommonOptions.Confirm = true
	p.cfg.CreateOpts.SkipSBOM = !p.cfg.CreateOpts.NoYOLO

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := p.cdToBaseDir(p.cfg.CreateOpts.BaseDir, cwd); err != nil {
		return err
	}

	if err := p.load(); err != nil {
		return err
	}

	// Set the package version to the CLI version
	p.cfg.Pkg.Build.Version = config.CLIVersion

	// Filter out components that are not compatible with this system
	if err := p.filterComponentsByArchAndOS(); err != nil {
		return err
	}

	// Also filter out components that are not required, nor requested via --components
	// This is different from the above filter, as it is not based on the system, but rather
	// the user's selection and the component's fields
	// This is also different from regular package creation, where we still assemble and package up
	// all components and their dependencies, regardless of whether they are required or not
	filter := filters.ForDeploy(p.cfg.PkgOpts.OptionalComponents, false)
	p.cfg.Pkg.Components, err = filter.Apply(p.cfg.Pkg)
	if err != nil {
		return err
	}

	if err := validate.Run(p.cfg.Pkg); err != nil {
		return fmt.Errorf("unable to validate package: %w", err)
	}

	// If building in yolo mode, strip out all images and repos
	if !p.cfg.CreateOpts.NoYOLO {
		for idx := range p.cfg.Pkg.Components {
			p.cfg.Pkg.Components[idx].Images = []string{}
			p.cfg.Pkg.Components[idx].Repos = []string{}
		}
	}

	if err := p.assemble(); err != nil {
		return err
	}

	message.HeaderInfof("📦 PACKAGE DEPLOY %s", p.cfg.Pkg.Metadata.Name)

	// Set variables and prompt if --confirm is not set
	if err := p.setVariableMapInConfig(); err != nil {
		return fmt.Errorf("unable to set the active variables: %w", err)
	}

	p.connectStrings = make(types.ConnectStrings)

	if !p.cfg.CreateOpts.NoYOLO {
		p.cfg.Pkg.Metadata.YOLO = true
	} else {
		p.hpaModified = false
		// Reset registry HPA scale down whether an error occurs or not
		defer p.resetRegistryHPA()
	}

	// Get a list of all the components we are deploying and actually deploy them
	deployedComponents, err := p.deployComponents()
	if err != nil {
		return err
	}
	if len(deployedComponents) == 0 {
		message.Warn("No components were selected for deployment.  Inspect the package to view the available components and select components interactively or by name with \"--components\"")
	}

	// Notify all the things about the successful deployment
	message.Successf("Zarf dev deployment complete")

	message.HorizontalRule()
	message.Title("Next steps:", "")

	message.ZarfCommand("package inspect %s", p.cfg.Pkg.Metadata.Name)

	// cd back
	return os.Chdir(cwd)
}
