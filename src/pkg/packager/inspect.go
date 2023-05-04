// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
	"github.com/pterm/pterm"
)

// Inspect list the contents of a package.
func (p *Packager) Inspect(includeSBOM bool, outputSBOM string, inspectPublicKey string) error {
	// Handle OCI packages that have been published to a registry
	if utils.IsOCIURL(p.cfg.DeployOpts.PackagePath) {

		// Download all the layers we need
		pullSBOM := includeSBOM || outputSBOM != ""
		pullZarfSig := inspectPublicKey != ""

		layersToPull := []string{config.ZarfYAML}
		if pullSBOM {
			layersToPull = append(layersToPull, "sboms.tar")
		}
		if pullZarfSig {
			layersToPull = append(layersToPull, "zarf.yaml.sig")
		}

		message.Debugf("Pulling layers %v from %s", layersToPull, p.cfg.DeployOpts.PackagePath)
		if err := p.pullPackageLayers(p.cfg.DeployOpts.PackagePath, p.tmp.Base, layersToPull); err != nil {
			return fmt.Errorf("unable to pull layers for inspect: %w", err)
		}
		err := utils.ReadYaml(p.tmp.ZarfYaml, &p.cfg.Pkg)
		if err != nil {
			return fmt.Errorf("unable to read the zarf yaml for the inspect: %w", err)
		}
	} else {
		// This package exists on the local file system - extract the first layer of the tarball
		if err := archiver.Unarchive(p.cfg.DeployOpts.PackagePath, p.tmp.Base); err != nil {
			return fmt.Errorf("unable to extract the package: %w", err)
		}
		if err := p.readYaml(p.tmp.ZarfYaml, true); err != nil {
			return fmt.Errorf("unable to read the zarf.yaml in %s: %w", p.tmp.Base, err)
		}

	}

	pterm.Println()
	pterm.Println()

	utils.ColorPrintYAML(p.cfg.Pkg)

	// Attempt to validate the checksums, or explain why we cannot validate them
	if !utils.IsOCIURL(p.cfg.DeployOpts.PackagePath) {
		// If the package is not a remote OCI package, we can validate the checksums
		if err := p.validatePackageChecksums(); err != nil {
			message.Warnf("Unable to validate the package checksums, the package may have been tampered with: %s", err.Error())
		}
	} else {
		message.Warnf("Zarf is unable to validate the checksums of remote OCI packages. We are unable to determine the integrity of the package without downloading the entire package.")
	}

	// Validate the package checksums and signatures if specified, and warn if the package was signed but a key was not provided
	_, sigExistErr := os.Stat(p.tmp.ZarfSig)
	if inspectPublicKey != "" {
		if err := p.validatePackageSignature(inspectPublicKey); err != nil {
			return fmt.Errorf("unable to validate the package signature: %w", err)
		}
	} else if sigExistErr == nil {
		message.Warnf("The package you are inspecting has been signed but a public key was not provided.")
	}

	if includeSBOM || outputSBOM != "" {
		// Extract the SBOM files from the sboms.tar file
		_, tarErr := os.Stat(p.tmp.SbomTar)
		if tarErr == nil {
			if err := archiver.Unarchive(p.tmp.SbomTar, p.tmp.Sboms); err != nil {
				return fmt.Errorf("unable to extract the SBOM files: %w", err)
			}
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
