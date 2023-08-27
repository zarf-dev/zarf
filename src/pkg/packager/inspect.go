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
func (p *Packager) Inspect(includeSBOM bool, outputSBOM string, inspectPublicKey string) error {
	wantSBOM := includeSBOM || outputSBOM != ""

	p.cfg.PkgOpts.PublicKeyPath = inspectPublicKey

	if p.provider == nil {
		provider, err := ProviderFromSource(&p.cfg.PkgOpts, p.tmp.Base())
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

	sbomDir := loaded[types.ZarfSBOMDir]

	if outputSBOM != "" {
		out, err := sbom.OutputSBOMFiles(loaded[types.ZarfSBOMDir], outputSBOM, pkg.Metadata.Name)
		if err != nil {
			return err
		}
		sbomDir = out
	}

	if includeSBOM {
		sbom.ViewSBOMFiles(sbomDir)
	}

	return nil
}
