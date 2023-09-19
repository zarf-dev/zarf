// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

// Mirror pulls resources from a package (images, git repositories, etc) and pushes them to remotes in the air gap without deploying them
func (p *Packager) Mirror() (err error) {
	spinner := message.NewProgressSpinner("Mirroring Zarf package %s", p.cfg.PkgOpts.PackageSource)
	defer spinner.Stop()

	if err = p.source.LoadPackage(p.layout); err != nil {
		return fmt.Errorf("unable to load the package: %w", err)
	}
	if p.cfg.Pkg, p.arch, err = ReadZarfYAML(p.layout.ZarfYAML); err != nil {
		return err
	}

	// If SBOMs were loaded, temporarily place them in the deploy directory
	var sbomViewFiles []string
	sbomDir := string(p.layout.SBOMs)
	if !utils.InvalidPath(sbomDir) {
		sbomViewFiles, _ = filepath.Glob(filepath.Join(sbomDir, "sbom-viewer-*"))
		_, err := sbom.OutputSBOMFiles(sbomDir, types.SBOMDir, "")
		if err != nil {
			// Don't stop the deployment, let the user decide if they want to continue the deployment
			message.Warnf("Unable to process the SBOM files for this package: %s", err.Error())
		}
	}

	// Confirm the overall package mirror
	if !p.confirmAction(config.ZarfMirrorStage, sbomViewFiles) {
		return fmt.Errorf("mirror cancelled")
	}

	state := &types.ZarfState{
		RegistryInfo: p.cfg.InitOpts.RegistryInfo,
		GitServer:    p.cfg.InitOpts.GitServer,
	}
	p.cfg.State = state

	// Filter out components that are not compatible with this system if we have loaded from a tarball
	p.filterComponents(&p.cfg.Pkg)
	requestedComponentNames := helpers.StringToSlice(p.cfg.PkgOpts.OptionalComponents)

	for _, component := range p.cfg.Pkg.Components {
		if len(requestedComponentNames) == 0 || helpers.SliceContains(requestedComponentNames, component.Name) {
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
