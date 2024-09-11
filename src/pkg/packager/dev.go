// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager/creator"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
)

// DevDeploy creates + deploys a package in one shot
func (p *Packager) DevDeploy(ctx context.Context) error {
	config.CommonOptions.Confirm = true
	p.cfg.CreateOpts.SkipSBOM = !p.cfg.CreateOpts.NoYOLO

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := os.Chdir(p.cfg.CreateOpts.BaseDir); err != nil {
		return fmt.Errorf("unable to access directory %q: %w", p.cfg.CreateOpts.BaseDir, err)
	}

	pc := creator.NewPackageCreator(p.cfg.CreateOpts, cwd)

	if err := helpers.CreatePathAndCopy(layout.ZarfYAML, p.Layout.ZarfYAML); err != nil {
		return err
	}

	p.cfg.Pkg, _, err = pc.LoadPackageDefinition(ctx, p.Layout)
	if err != nil {
		return err
	}

	filter := filters.Combine(
		filters.ByLocalOS(runtime.GOOS),
		filters.ForDeploy(p.cfg.PkgOpts.OptionalComponents, false),
	)
	p.cfg.Pkg.Components, err = filter.Apply(p.cfg.Pkg)
	if err != nil {
		return err
	}

	if err := creator.Validate(p.cfg.Pkg, p.cfg.CreateOpts.BaseDir, p.cfg.CreateOpts.SetVariables); err != nil {
		return fmt.Errorf("package validation failed: %w", err)
	}

	if err := p.populatePackageVariableConfig(); err != nil {
		return fmt.Errorf("unable to set the active variables: %w", err)
	}

	// If building in yolo mode, strip out all images and repos
	if !p.cfg.CreateOpts.NoYOLO {
		for idx := range p.cfg.Pkg.Components {
			p.cfg.Pkg.Components[idx].Images = []string{}
			p.cfg.Pkg.Components[idx].Repos = []string{}
		}
	}

	if err := pc.Assemble(ctx, p.Layout, p.cfg.Pkg.Components, p.cfg.Pkg.Metadata.Architecture); err != nil {
		return err
	}

	message.HeaderInfof("📦 PACKAGE DEPLOY %s", p.cfg.Pkg.Metadata.Name)

	if !p.cfg.CreateOpts.NoYOLO {
		p.cfg.Pkg.Metadata.YOLO = true
	} else {
		p.hpaModified = false
		// Reset registry HPA scale down whether an error occurs or not
		defer p.resetRegistryHPA(ctx)
	}

	// Get a list of all the components we are deploying and actually deploy them
	deployedComponents, err := p.deployComponents(ctx)
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
