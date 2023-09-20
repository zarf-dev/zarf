// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"strings"

	"slices"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

// Mirror pulls resources from a package (images, git repositories, etc) and pushes them to remotes in the air gap without deploying them
func (p *Packager) Mirror() (err error) {
	spinner := message.NewProgressSpinner("Mirroring Zarf package %s", p.cfg.PkgOpts.PackagePath)
	defer spinner.Stop()

	if helpers.IsOCIURL(p.cfg.PkgOpts.PackagePath) {
		err := p.SetOCIRemote(p.cfg.PkgOpts.PackagePath)
		if err != nil {
			return err
		}
	}

	if err := p.loadZarfPkg(); err != nil {
		return fmt.Errorf("unable to load the Zarf Package: %w", err)
	}

	if err := ValidatePackageSignature(p.tmp.Base, p.cfg.PkgOpts.PublicKeyPath); err != nil {
		return err
	}

	// Confirm the overall package mirror
	if !p.confirmAction(config.ZarfMirrorStage, p.cfg.SBOMViewFiles) {
		return fmt.Errorf("mirror cancelled")
	}

	state := &types.ZarfState{
		RegistryInfo: p.cfg.InitOpts.RegistryInfo,
		GitServer:    p.cfg.InitOpts.GitServer,
	}
	p.cfg.State = state

	// Filter out components that are not compatible with this system if we have loaded from a tarball
	p.filterComponents(true)
	requestedComponentNames := getRequestedComponentList(p.cfg.PkgOpts.OptionalComponents)

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

	componentPath, err := p.createOrGetComponentPaths(component)
	if err != nil {
		return fmt.Errorf("unable to create the component paths: %w", err)
	}

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
		if err = p.pushReposToRepository(componentPath.Repos, component.Repos); err != nil {
			return fmt.Errorf("unable to push the repos to the repository: %w", err)
		}
	}

	return nil
}
