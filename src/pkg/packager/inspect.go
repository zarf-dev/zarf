// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"fmt"
	"os"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/internal/packager/sbom"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// Inspect list the contents of a package.
func (p *Packager) Inspect(ctx context.Context) error {
	wantSBOM := p.cfg.InspectOpts.ViewSBOM || p.cfg.InspectOpts.SBOMOutputDir != ""

	pkg, _, err := p.source.LoadPackageMetadata(ctx, p.Layout, wantSBOM, true)
	if err != nil {
		return err
	}
	p.cfg.Pkg = pkg

	if p.cfg.InspectOpts.ListImages && len(p.cfg.Pkg.Components) > 0 {
		imageList := []string{}
		for _, component := range p.cfg.Pkg.Components {
			imageList = append(imageList, component.Images...)
		}
		imageList = helpers.Unique(imageList)
		for _, image := range imageList {
			fmt.Fprintln(os.Stdout, "-", image)
		}
	} else {
		utils.ColorPrintYAML(p.cfg.Pkg, nil, false)
	}

	sbomDir := p.Layout.SBOMs.Path

	if p.cfg.InspectOpts.SBOMOutputDir != "" {
		out, err := p.Layout.SBOMs.OutputSBOMFiles(p.cfg.InspectOpts.SBOMOutputDir, p.cfg.Pkg.Metadata.Name)
		if err != nil {
			return err
		}
		sbomDir = out
	}

	if p.cfg.InspectOpts.ViewSBOM {
		err := sbom.ViewSBOMFiles(sbomDir)
		if err != nil {
			return err
		}
	}

	return nil
}
