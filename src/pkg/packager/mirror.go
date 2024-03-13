// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/filters"
	"github.com/defenseunicorns/zarf/src/types"
)

// Mirror pulls resources from a package (images, git repositories, etc) and pushes them to remotes in the air gap without deploying them
func (p *Packager) Mirror() (err error) {
	spinner := message.NewProgressSpinner("Mirroring Zarf package %s", p.cfg.PkgOpts.PackageSource)
	defer spinner.Stop()

	filter := filters.Combine(
		filters.BySelectState(p.cfg.PkgOpts.OptionalComponents),
		p.archAndOSFilter(),
	)

	if err = p.source.LoadPackage(p.layout, filter, true); err != nil {
		return fmt.Errorf("unable to load the package: %w", err)
	}

	p.cfg.Pkg, p.warnings, err = p.layout.ReadZarfYAML()
	if err != nil {
		return err
	}

	sbomWarnings, err := p.layout.SBOMs.StageSBOMViewFiles()
	if err != nil {
		return err
	}

	p.warnings = append(p.warnings, sbomWarnings...)

	// Confirm the overall package mirror
	if !p.confirmAction(config.ZarfMirrorStage) {
		return fmt.Errorf("mirror cancelled")
	}

	p.cfg.Pkg.Components, err = filter.Apply(p.cfg.Pkg)
	if err != nil {
		return err
	}

	p.cfg.State = &types.ZarfState{
		RegistryInfo: p.cfg.InitOpts.RegistryInfo,
		GitServer:    p.cfg.InitOpts.GitServer,
	}

	for _, component := range p.cfg.Pkg.Components {
		if err := p.mirrorComponent(component); err != nil {
			return err
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
