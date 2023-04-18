// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pterm/pterm"
	"oras.land/oras-go/v2/registry"
)

// Inspect list the contents of a package.
func (p *Packager) Inspect(includeSBOM bool, outputSBOM string, inspectPublicKey string) error {
	// Handle OCI packages that have been published to a registry
	if utils.IsOCIURL(p.cfg.DeployOpts.PackagePath) {

		// Download all the layers we need
		pullSBOM := includeSBOM || outputSBOM != ""
		pullZarfSig := inspectPublicKey != ""
		if err := pullLayersForInspect(p.cfg.DeployOpts.PackagePath, p.tmp, pullSBOM, pullZarfSig); err != nil {
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

func pullLayersForInspect(packagePath string, tmpPath types.TempPaths, includeSBOM bool, includeSig bool) error {
	spinner := message.NewProgressSpinner("Loading Zarf Package %s", packagePath)
	ref, err := registry.ParseReference(strings.TrimPrefix(packagePath, "oci://"))
	if err != nil {
		return err
	}

	dst, err := utils.NewOrasRemote(ref)
	if err != nil {
		return err
	}

	// get the manifest
	spinner.Updatef("Fetching the manifest for %s", packagePath)
	layers, err := getLayers(dst)
	if err != nil {
		return err
	}
	spinner.Updatef("Loading Zarf Package %s", packagePath)
	zarfYamlDesc := utils.Find(layers, func(d ocispec.Descriptor) bool {
		return d.Annotations["org.opencontainers.image.title"] == "zarf.yaml"
	})
	err = pullLayer(dst, zarfYamlDesc, tmpPath.ZarfYaml)
	if err != nil {
		return err
	}

	if includeSBOM {
		sbmomsTarDesc := utils.Find(layers, func(d ocispec.Descriptor) bool {
			return d.Annotations["org.opencontainers.image.title"] == "sboms.tar"
		})
		err = pullLayer(dst, sbmomsTarDesc, tmpPath.SbomTar)
		if err != nil {
			return err
		}
		if err := archiver.Unarchive(tmpPath.SbomTar, filepath.Join(tmpPath.Base, "sboms")); err != nil {
			return err
		}
	}

	if includeSig {
		sigTarDesc := utils.Find(layers, func(d ocispec.Descriptor) bool {
			return d.Annotations["org.opencontainers.image.title"] == "zarf.yaml.sig"
		})
		err = pullLayer(dst, sigTarDesc, tmpPath.ZarfSig)
		if err != nil {
			return err
		}
	}

	return nil
}
