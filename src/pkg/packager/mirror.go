// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
)

// Mirror pulls resources from a package (images, git repositories, etc) and pushes them to remotes in the air gap without deploying them
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
	if !p.confirmAction(config.ZarfMirrorStage, warnings, sbomViewFiles) {
		return fmt.Errorf("mirror cancelled")
	}

	for _, component := range p.cfg.Pkg.Components {
		componentPaths := p.layout.Components.Dirs[component.Name]

		// All components now require a name
		message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))

		if len(component.Images) > 0 {
			err := p.pushImagesToRegistry(ctx, p.cfg.InitOpts.RegistryInfo, component.Images, p.cfg.MirrorOpts.NoImgChecksum)
			if err != nil {
				return fmt.Errorf("unable to push images to the registry: %w", err)
			}
		}
		if len(component.Repos) > 0 {
			err := p.pushReposToRepository(ctx, p.cfg.InitOpts.GitServer, componentPaths.Repos, component.Repos)
			if err != nil {
				return fmt.Errorf("unable to push the repos to the repository: %w", err)
			}
		}
	}
	return nil
}
