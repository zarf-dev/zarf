// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"fmt"
	"os"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// Inspect list the contents of a package.
func (p *Packager) Inspect(ctx context.Context, viewSBOM bool, sbomOutputDir string, listImages bool) (err error) {
	wantSBOM := viewSBOM || sbomOutputDir != ""

	p.cfg.Pkg, p.warnings, err = p.source.LoadPackageMetadata(ctx, p.layout, wantSBOM, true)
	if err != nil {
		return err
	}

	if listImages {
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

	sbomDir := p.layout.SBOMs.Path

	if sbomOutputDir != "" {
		out, err := p.layout.SBOMs.OutputSBOMFiles(sbomOutputDir, p.cfg.Pkg.Metadata.Name)
		if err != nil {
			return err
		}
		sbomDir = out
	}

	if viewSBOM {
		sbom.ViewSBOMFiles(sbomDir)
	}

	return nil
}
