// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/pkg/packager/providers"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// Inspect list the contents of a package.
func (p *Packager) Inspect() error {
	wantSBOM := p.cfg.InspectOpts.ViewSBOM || p.cfg.InspectOpts.SBOMOutputDir != ""

	if p.provider == nil {
		provider, err := providers.NewFromSource(&p.cfg.PkgOpts, p.tmp.Base())
		if err != nil {
			return err
		}
		p.provider = provider
	}

	pkg, loaded, err := p.provider.LoadPackageMetadata(wantSBOM)
	if err != nil {
		return err
	}

	utils.ColorPrintYAML(pkg, nil, false)

	sbomDir := loaded[types.SBOMDir]

	if p.cfg.InspectOpts.SBOMOutputDir != "" {
		out, err := sbom.OutputSBOMFiles(sbomDir, p.cfg.InspectOpts.SBOMOutputDir, pkg.Metadata.Name)
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
