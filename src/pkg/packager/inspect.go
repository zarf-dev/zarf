// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// Inspect list the contents of a package.
func (p *Packager) Inspect() (err error) {
	wantSBOM := p.cfg.InspectOpts.ViewSBOM || p.cfg.InspectOpts.SBOMOutputDir != ""

	if err = p.source.LoadPackageMetadata(p.tmp, wantSBOM); err != nil {
		return err
	}

	if p.cfg.Pkg, p.arch, err = ReadZarfYAML(p.tmp[types.ZarfYAML]); err != nil {
		return err
	}

	utils.ColorPrintYAML(p.cfg.Pkg, nil, false)

	sbomDir := p.tmp[types.SBOMDir]

	if p.cfg.InspectOpts.SBOMOutputDir != "" {
		out, err := sbom.OutputSBOMFiles(sbomDir, p.cfg.InspectOpts.SBOMOutputDir, p.cfg.Pkg.Metadata.Name)
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
