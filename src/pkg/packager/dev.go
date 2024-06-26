// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"slices"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/images"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/creator"
	"github.com/defenseunicorns/zarf/src/pkg/packager/filters"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/google/go-containerregistry/pkg/crane"
)

// DevDeploy creates + deploys a package in one shot
func (p *Packager) DevDeploy(ctx context.Context) error {
	config.CommonOptions.Confirm = true
	p.cfg.CreateOpts.SkipSBOM = true

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

	// If building in yolo mode, strip out all images and repos
	if !p.cfg.CreateOpts.NoYOLO {
		for idx := range p.cfg.Pkg.Components {
			p.cfg.Pkg.Components[idx].Images = []string{}
			p.cfg.Pkg.Components[idx].Repos = []string{}
		}
	} else {
		diff := func(a, b []string, normalizer func(string) string) []string {
			var diff []string

			for _, ele := range b {
				if !slices.ContainsFunc(a, func(s string) bool {
					return normalizer(s) == normalizer(ele)
				}) {
					diff = append(diff, ele)
				}
			}

			return diff
		}

		normalizeImageName := func(s string) string {
			imgInfo, _ := transform.ParseImageRef(s)
			return imgInfo.Path
		}

		if c, err := cluster.NewCluster(); err == nil { // if NO error
			if zarfState, err := c.LoadZarfState(ctx); err == nil { // if NO error
				authOption := images.WithPullAuth(zarfState.RegistryInfo)

				if registryEndpoint, tunnel, err := c.ConnectToZarfRegistryEndpoint(ctx, zarfState.RegistryInfo); err == nil { // if NO error
					defer tunnel.Close()

					if names, err := crane.Catalog(registryEndpoint, authOption); err == nil { // if NO error
						for idx, pending := range p.cfg.Pkg.Components {
							p.cfg.Pkg.Components[idx].Images = diff(names, pending.Images, normalizeImageName)
						}
					}
				}
			}
		}
	}

	if err := pc.Assemble(ctx, p.layout, p.cfg.Pkg.Components, p.cfg.Pkg.Metadata.Architecture); err != nil {
		return err
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
