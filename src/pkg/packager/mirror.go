// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"strings"

	"slices"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

// Mirror pulls resources from a package (images, git repositories, etc) and pushes them to remotes in the air gap without deploying them
func (p *Packager) Mirror() (err error) {
	// Attempt to connect to a Kubernetes cluster.
	// Not all packages require Kubernetes, so we only want to log a debug message rather than return the error when we can't connect to a cluster.
	p.cluster, err = cluster.NewCluster()
	if err != nil {
		message.Debug(err)
	}

	spinner := message.NewProgressSpinner("Mirroring Zarf package %s", p.cfg.PkgOpts.PackageSource)
	defer spinner.Stop()

	if err = p.source.LoadPackage(p.layout, true); err != nil {
		return fmt.Errorf("unable to load the package: %w", err)
	}
	if err = p.readZarfYAML(p.layout.ZarfYAML); err != nil {
		return err
	}

	if err := p.stageSBOMViewFiles(); err != nil {
		return err
	}

	// Confirm the overall package mirror
	if !p.confirmAction(config.ZarfMirrorStage) {
		return fmt.Errorf("mirror cancelled")
	}

	state := &types.ZarfState{
		RegistryInfo: p.cfg.InitOpts.RegistryInfo,
		GitServer:    p.cfg.InitOpts.GitServer,
	}
	p.cfg.State = state

	// Filter out components that are not compatible with this system if we have loaded from a tarball
	p.filterComponents()
	requestedComponentNames := helpers.StringToSlice(p.cfg.PkgOpts.OptionalComponents)

	for _, component := range p.cfg.Pkg.Components {
		if len(requestedComponentNames) == 0 || slices.Contains(requestedComponentNames, component.Name) {
			if err := p.mirrorComponent(component); err != nil {
				return err
			}
		}
	}

	return nil
}

// mirrorComponent mirrors a Zarf Component.
func (p *Packager) mirrorComponent(component types.ZarfComponent) error {
	componentPaths := p.layout.Components.Dirs[component.Name]

	// All components now require a name
	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))

	hasImages := len(component.Images) > 0
	hasRepos := len(component.Repos) > 0

	if hasImages {
		if err := p.pushImagesToRegistry(component.Images, p.cfg.MirrorOpts.NoImgChecksum); err != nil {
			return fmt.Errorf("unable to push images to the registry: %w", err)
		}
	}

	if hasRepos {
		if err := p.pushReposToRepository(componentPaths.Repos, component.Repos); err != nil {
			return fmt.Errorf("unable to push the repos to the repository: %w", err)
		}
	}

	return nil
}
