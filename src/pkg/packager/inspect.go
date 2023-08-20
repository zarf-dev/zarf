// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// Inspect list the contents of a package.
func (p *Packager) Inspect(includeSBOM bool, outputSBOM string, inspectPublicKey string) error {
	wantSBOM := includeSBOM || outputSBOM != ""

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

	// Validate the package checksums and signatures if specified, and warn if the package was signed but a key was not provided
	// if err := p.provider.Validate(&types.LoadedPackagePaths{LoadedMetadataPaths: *metatdataPaths}, p.cfg.PkgOpts.PublicKeyPath); err != nil {
	// 	if err == ErrPkgSigButNoKey {
	// 		message.Warn("The package was signed but no public key was provided, skipping signature validation")
	// 	} else {
	// 		return fmt.Errorf("unable to validate the package signature: %w", err)
	// 	}
	// }

	if wantSBOM {
		return UnarchiveAndViewSBOMs(loaded[types.ZarfSBOMTar], outputSBOM, pkg.Metadata.Name, includeSBOM)
	}

	return nil
}
