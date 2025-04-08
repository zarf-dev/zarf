// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"fmt"
	"runtime"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/types"
)

// Mirror pulls resources from a package (images, git repositories, etc) and pushes them to remotes in the airgap without deploying them
func (p *Packager) Mirror(ctx context.Context) error {
	filter := filters.Combine(
		filters.ByLocalOS(runtime.GOOS),
		filters.BySelectState(p.cfg.PkgOpts.OptionalComponents),
	)

	pkg, warnings, err := p.source.LoadPackage(ctx, p.layout, filter, true)
	if err != nil {
		return fmt.Errorf("unable to load the package: %w", err)
	}
	p.cfg.Pkg = pkg

	sbomViewFiles, sbomWarnings, err := p.layout.SBOMs.StageSBOMViewFiles()
	if err != nil {
		return err
	}
	warnings = append(warnings, sbomWarnings...)

	// Confirm the overall package mirror
	if !p.confirmAction(ctx, config.ZarfMirrorStage, warnings, sbomViewFiles) {
		return fmt.Errorf("mirror cancelled")
	}

	p.state = &types.ZarfState{
		RegistryInfo: p.cfg.InitOpts.RegistryInfo,
		GitServer:    p.cfg.InitOpts.GitServer,
	}

	for _, component := range p.cfg.Pkg.Components {
		if err := p.mirrorComponent(ctx, component); err != nil {
			return err
		}
	}
	return nil
}

// mirrorComponent mirrors a Zarf Component.
func (p *Packager) mirrorComponent(ctx context.Context, component v1alpha1.ZarfComponent) error {
	componentPaths := p.layout.Components.Dirs[component.Name]

	logger.From(ctx).Info("mirroring component", "component", component.Name)

	hasImages := len(component.Images) > 0
	hasRepos := len(component.Repos) > 0

	if hasImages {
		if err := p.pushImagesToRegistry(ctx, component.Images, p.cfg.MirrorOpts.NoImgChecksum); err != nil {
			return fmt.Errorf("unable to push images to the registry: %w", err)
		}
	}

	if hasRepos {
		if err := p.pushReposToRepository(ctx, componentPaths.Repos, component.Repos); err != nil {
			return fmt.Errorf("unable to push the repos to the repository: %w", err)
		}
	}

	return nil
}
