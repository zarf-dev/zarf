// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
)

// Inspect list the contents of a package.
func (p *Packager) Inspect(includeSBOM bool, outputSBOM string, inspectPublicKey string) error {
	wantSBOM := includeSBOM || outputSBOM != ""

	partialPaths := []string{config.ZarfYAML}
	if wantSBOM {
		partialPaths = append(partialPaths, config.ZarfSBOMTar)
	}

	// Handle OCI packages that have been published to a registry
	if utils.IsOCIURL(p.cfg.DeployOpts.PackagePath) {

		message.Debugf("Pulling layers %v from %s", partialPaths, p.cfg.DeployOpts.PackagePath)

		err := p.SetOCIRemote(p.cfg.DeployOpts.PackagePath)
		if err != nil {
			return err
		}
		layersToPull, err := p.remote.LayersFromPaths(partialPaths)
		if err != nil {
			return err
		}
		if partialPaths, err = p.remote.PullPackage(p.tmp.Base, config.CommonOptions.OCIConcurrency, layersToPull...); err != nil {
			return fmt.Errorf("unable to pull the package: %w", err)
		}
		if err := p.readYaml(p.tmp.ZarfYaml); err != nil {
			return fmt.Errorf("unable to read the zarf.yaml in %s: %w", p.tmp.Base, err)
		}
	} else {
		// This package exists on the local file system - extract the first layer of the tarball
		if err := archiver.Extract(p.cfg.DeployOpts.PackagePath, config.ZarfChecksumsTxt, p.tmp.Base); err != nil {
			return fmt.Errorf("unable to extract %s: %w", config.ZarfChecksumsTxt, err)
		}

		if err := archiver.Extract(p.cfg.DeployOpts.PackagePath, config.ZarfYAML, p.tmp.Base); err != nil {
			return fmt.Errorf("unable to extract %s: %w", config.ZarfYAML, err)
		}
		if err := archiver.Extract(p.cfg.DeployOpts.PackagePath, config.ZarfYAMLSignature, p.tmp.Base); err != nil {
			return fmt.Errorf("unable to extract %s: %w", config.ZarfYAMLSignature, err)
		}
		if err := p.readYaml(p.tmp.ZarfYaml); err != nil {
			return fmt.Errorf("unable to read the zarf.yaml in %s: %w", p.tmp.Base, err)
		}
		if wantSBOM {
			if err := archiver.Extract(p.cfg.DeployOpts.PackagePath, config.ZarfSBOMTar, p.tmp.Base); err != nil {
				return fmt.Errorf("unable to extract %s: %w", config.ZarfSBOMTar, err)
			}
		}
	}

	utils.ColorPrintYAML(p.cfg.Pkg, nil, false)

	if err := p.validatePackageChecksums(p.tmp.Base, p.cfg.Pkg.Metadata.AggregateChecksum, partialPaths); err != nil {
		return fmt.Errorf("unable to validate the package checksums, the package may have been tampered with: %s", err.Error())
	}

	// Validate the package checksums and signatures if specified, and warn if the package was signed but a key was not provided
	if err := p.validatePackageSignature(inspectPublicKey); err != nil {
		if err == ErrPkgSigButNoKey {
			message.Warn("The package was signed but no public key was provided, skipping signature validation")
		} else {
			return fmt.Errorf("unable to validate the package signature: %w", err)
		}
	}

	if wantSBOM {
		// Extract the SBOM files from the sboms.tar file
		if err := archiver.Unarchive(p.tmp.SbomTar, p.tmp.Sboms); err != nil {
			return fmt.Errorf("unable to extract the SBOM files: %w", err)
		}
	}

	// Open a browser to view the SBOM if specified
	if includeSBOM {
		sbom.ViewSBOMFiles(p.tmp)
	}

	// Output the SBOM files into a directory if specified
	if outputSBOM != "" {
		if err := sbom.OutputSBOMFiles(p.tmp, outputSBOM, p.cfg.Pkg.Metadata.Name); err != nil {
			return err
		}
	}

	return nil
}
