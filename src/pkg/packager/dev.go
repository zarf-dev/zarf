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
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/creator"
	"github.com/defenseunicorns/zarf/src/pkg/packager/filters"
	"github.com/defenseunicorns/zarf/src/types"
)

// DevDeploy creates + deploys a package in one shot
func (p *Packager) DevDeploy(ctx context.Context) error {
	config.CommonOptions.Confirm = true
	p.cfg.CreateOpts.SkipSBOM = true

	// todo: implement dry-run for helm charts
	// diff images before push

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := os.Chdir(p.cfg.CreateOpts.BaseDir); err != nil {
		return fmt.Errorf("unable to access directory %q: %w", p.cfg.CreateOpts.BaseDir, err)
	}

	pc := creator.NewPackageCreator(p.cfg.CreateOpts, cwd)

	if err := helpers.CreatePathAndCopy(layout.ZarfYAML, p.layout.ZarfYAML); err != nil {
		return err
	}

	p.cfg.Pkg, p.warnings, err = pc.LoadPackageDefinition(ctx, p.layout)
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

	if err := p.cfg.Pkg.Validate(); err != nil {
		return fmt.Errorf("unable to validate package: %w", err)
	}

	if err := p.populatePackageVariableConfig(); err != nil {
		return fmt.Errorf("unable to set the active variables: %w", err)
	}

	preserveComponents := p.cfg.Pkg.Components

	// If building in yolo mode, strip out all images and repos
	if !p.cfg.CreateOpts.NoYOLO {
		for idx := range p.cfg.Pkg.Components {
			p.cfg.Pkg.Components[idx].Images = []string{}
			p.cfg.Pkg.Components[idx].Repos = []string{}
		}
	} else {
		cluster, _ := cluster.NewCluster()

		dp, _ := cluster.GetDeployedPackage(ctx, p.cfg.Pkg.Metadata.Name)

		booleanDiff := func(a, b []string) []string {
			m := make(map[string]bool, len(a))
			for _, item := range a {
				m[item] = true
			}
			for _, item := range b {
				// if the item only exists in b,
				// add it to m
				if _, ok := m[item]; !ok {
					m[item] = true
				} else {
					// if item is in both a and b,
					// delete it from m
					delete(m, item)
				}
			}
			var diff []string
			for item := range m {
				diff = append(diff, item)
			}
			return diff
		}

		if dp != nil {
			// boolean diff the deployed package images with the package images
			for _, c := range dp.Data.Components {
				for idx, pending := range p.cfg.Pkg.Components {
					if c.Name == pending.Name {
						p.cfg.Pkg.Components[idx].Images = booleanDiff(c.Images, pending.Images)
					}
				}
			}
		}
	}

	if err := pc.Assemble(ctx, p.layout, p.cfg.Pkg.Components, p.cfg.Pkg.Metadata.Architecture); err != nil {
		return err
	}

	if p.cfg.CreateOpts.NoYOLO {
		// restore potentially mutate components
		p.cfg.Pkg.Components = preserveComponents
	}

	message.HeaderInfof("📦 PACKAGE DEPLOY %s", p.cfg.Pkg.Metadata.Name)

	p.connectStrings = make(types.ConnectStrings)

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
