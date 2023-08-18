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
		provider, err := ProviderFromSource(p.cfg.PkgOpts.PackagePath, p.cfg.PkgOpts.Shasum, p.tmp, p.cfg.PkgOpts.PublicKeyPath)
		if err != nil {
			return err
		}
		p.provider = provider
	}

	pkg, err := p.provider.LoadPackageMetadata(wantSBOM)
	if err != nil {
		return err
	}

	// // Handle OCI packages that have been published to a registry
	// if helpers.IsOCIURL(p.cfg.PkgOpts.PackagePath) {
	// 	message.Debugf("Pulling layers %v from %s", partialPaths, p.cfg.PkgOpts.PackagePath)

	// 	err := p.SetOCIRemote(p.cfg.PkgOpts.PackagePath)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	layersToPull, err := p.remote.LayersFromPaths(partialPaths)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	if partialPaths, err = p.remote.PullPackage(p.tmp.Base, config.CommonOptions.OCIConcurrency, layersToPull...); err != nil {
	// 		return fmt.Errorf("unable to pull the package: %w", err)
	// 	}
	// 	if err := p.readYaml(p.tmp.ZarfYaml); err != nil {
	// 		return fmt.Errorf("unable to read the zarf.yaml in %s: %w", p.tmp.Base, err)
	// 	}
	// } else {

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
		return UnarchiveAndViewSBOMs(p.tmp[types.ZarfSBOMTar], outputSBOM, pkg.Metadata.Name, includeSBOM)
	}

	return nil
}
