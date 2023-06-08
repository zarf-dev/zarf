// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
	"github.com/pterm/pterm"
)

// Inspect list the contents of a package.
func (p *Packager) Inspect(includeSBOM bool, outputSBOM string, inspectPublicKey string) error {
	wantSBOM := includeSBOM || outputSBOM != ""

	requestedFiles := []string{config.ZarfYAML}
	if wantSBOM {
		requestedFiles = append(requestedFiles, config.ZarfSBOMTar)
	}

	// Handle OCI packages that have been published to a registry
	if utils.IsOCIURL(p.cfg.DeployOpts.PackagePath) {

		message.Debugf("Pulling layers %v from %s", requestedFiles, p.cfg.DeployOpts.PackagePath)

		client, err := oci.NewOrasRemote(p.cfg.DeployOpts.PackagePath)
		if err != nil {
			return err
		}
		layersToPull, err := client.LayersFromPaths(requestedFiles)
		if err != nil {
			return err
		}
		if err := client.PullPackage(p.tmp.Base, p.cfg.PullOpts.OCIConcurrency, layersToPull...); err != nil {
			return fmt.Errorf("unable to pull the package: %w", err)
		}
		if err := p.readYaml(p.tmp.ZarfYaml); err != nil {
			return fmt.Errorf("unable to read the zarf.yaml in %s: %w", p.tmp.Base, err)
		}
	} else {
		// This package exists on the local file system - extract the first layer of the tarball
		if err := archiver.Unarchive(p.cfg.DeployOpts.PackagePath, p.tmp.Base); err != nil {
			return fmt.Errorf("unable to extract the package: %w", err)
		}
		if err := p.readYaml(p.tmp.ZarfYaml); err != nil {
			return fmt.Errorf("unable to read the zarf.yaml in %s: %w", p.tmp.Base, err)
		}

	}

	pterm.Println()
	pterm.Println()

	utils.ColorPrintYAML(p.cfg.Pkg)

	if err := utils.ValidatePackageChecksums(p.tmp.Base, p.cfg.Pkg.Metadata.AggregateChecksum, requestedFiles); err != nil {
		message.Warnf("Unable to validate the package checksums, the package may have been tampered with: %s", err.Error())
	}

	// Validate the package checksums and signatures if specified, and warn if the package was signed but a key was not provided
	sigExist := !utils.InvalidPath(p.tmp.ZarfSig)
	if inspectPublicKey != "" {
		if err := p.validatePackageSignature(inspectPublicKey); err != nil {
			return fmt.Errorf("unable to validate the package signature: %w", err)
		}
	} else if sigExist {
		message.Warnf("The package you are inspecting has been signed but a public key was not provided.")
	}

	if wantSBOM {
		// Extract the SBOM files from the sboms.tar file
		tarExists := !utils.InvalidPath(p.tmp.SbomTar)
		if tarExists {
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
