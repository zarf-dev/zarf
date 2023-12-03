// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
)

// DevDeploy creates + deploys a package in one shot
func (p *Packager) DevDeploy() error {
	config.CommonOptions.Confirm = true

	p.cfg.CreateOpts.Mode = types.CreateModeDev

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := p.Create(); err != nil {
		return err
	}

	// Set variables and prompt if --confirm is not set
	if err := p.setVariableMapInConfig(); err != nil {
		return fmt.Errorf("unable to set the active variables: %w", err)
	}

	// TODO: is HPA needed for dev?

	// Filter out components that are not compatible with this system
	p.filterComponents()

	// Remove images + git repos from each component
	for idx := range p.cfg.Pkg.Components {
		p.cfg.Pkg.Components[idx].Images = []string{}
		p.cfg.Pkg.Components[idx].Repos = []string{}
	}
	// Set YOLO to always be true for dev
	// TODO: this will be a flag
	p.cfg.Pkg.Metadata.YOLO = true

	// Get a list of all the components we are deploying and actually deploy them
	deployedComponents, err := p.deployComponents()
	if err != nil {
		return fmt.Errorf("unable to deploy all components in this Zarf Package: %w", err)
	}
	if len(deployedComponents) == 0 {
		message.Warn("No components were selected for deployment.  Inspect the package to view the available components and select components interactively or by name with \"--components\"")
	}

	// Notify all the things about the successful deployment
	message.Successf("Zarf dev deployment complete")

	// cd back
	return os.Chdir(cwd)
}
