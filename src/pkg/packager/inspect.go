// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// Inspect list the contents of a package.
func (p *Packager) Inspect() (err error) {
	wantSBOM := p.cfg.InspectOpts.ViewSBOM || p.cfg.InspectOpts.SBOMOutputDir != ""

	p.cfg.Pkg, p.warnings, err = p.source.LoadPackageMetadata(p.layout, wantSBOM, true)
	if err != nil {
		return err
	}

	if p.cfg.InspectOpts.ListImages {
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

	if p.cfg.InspectOpts.SBOMOutputDir != "" {
		out, err := p.layout.SBOMs.OutputSBOMFiles(p.cfg.InspectOpts.SBOMOutputDir, p.cfg.Pkg.Metadata.Name)
		if err != nil {
			return err
		}
		sbomDir = out
	}

	if p.cfg.InspectOpts.ViewSBOM {
		sbom.ViewSBOMFiles(sbomDir)
	}

	return nil
}
