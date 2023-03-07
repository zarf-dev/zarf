// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pterm/pterm"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry"
)

// Inspect list the contents of a package.
func (p *Packager) Inspect(includeSBOM bool, outputSBOM string) error {
	if utils.IsOCIURL(p.cfg.DeployOpts.PackagePath) {
		spinner := message.NewProgressSpinner("Loading Zarf Package %s", p.cfg.DeployOpts.PackagePath)
		ref, err := registry.ParseReference(strings.TrimPrefix(p.cfg.DeployOpts.PackagePath, "oci://"))
		if err != nil {
			return err
		}

		dst, err := utils.NewOrasRemote(ref)
		if err != nil {
			return err
		}

		desc, err := dst.Resolve(dst.Context, ref.Reference)
		if err != nil {
			return err
		}

		// get the manifest
		spinner.Updatef("Fetching the manifest for %s", p.cfg.DeployOpts.PackagePath)
		manifestBytes, err := content.FetchAll(dst.Context, dst, desc)
		if err != nil {
			return err
		}
		manifest := ocispec.Manifest{}
		artifact := ocispec.Artifact{}
		var layers []ocispec.Descriptor
		// if the manifest is an artifact, unmarshal it as an artifact
		// otherwise, unmarshal it as a manifest
		if desc.MediaType == ocispec.MediaTypeArtifactManifest {
			if err = json.Unmarshal(manifestBytes, &artifact); err != nil {
				return err
			}
			layers = artifact.Blobs
		} else {
			if err = json.Unmarshal(manifestBytes, &manifest); err != nil {
				return err
			}
			layers = manifest.Layers
		}
		spinner.Updatef("Loading Zarf Package %s", p.cfg.DeployOpts.PackagePath)
		zarfYamlDesc := utils.Find(layers, func(d ocispec.Descriptor) bool {
			return d.Annotations["org.opencontainers.image.title"] == "zarf.yaml"
		})
		zarfYamlBytes, err := content.FetchAll(dst.Context, dst, zarfYamlDesc)
		if err != nil {
			return err
		}
		if err := utils.WriteFile(p.tmp.ZarfYaml, zarfYamlBytes); err != nil {
			return err
		}
		if includeSBOM {
			sbmomsTarDesc := utils.Find(layers, func(d ocispec.Descriptor) bool {
				return d.Annotations["org.opencontainers.image.title"] == "sboms.tar.zst"
			})
			sbmomsTarBytes, err := content.FetchAll(dst.Context, dst, sbmomsTarDesc)
			if err != nil {
				return err
			}
			if err := utils.WriteFile(filepath.Join(p.tmp.Base, "sboms.tar.zst"), sbmomsTarBytes); err != nil {
				return err
			}
			if err := archiver.Unarchive(filepath.Join(p.tmp.Base, "sboms.tar.zst"), filepath.Join(p.tmp.Base, "sboms")); err != nil {
				return err
			}
		}
		err = utils.ReadYaml(p.tmp.ZarfYaml, &p.cfg.Pkg)
		if err != nil {
			return err
		}
		spinner.Successf("Loaded Zarf Package %s", p.cfg.DeployOpts.PackagePath)
	} else {
		if err := p.loadZarfPkg(); err != nil {
			return fmt.Errorf("unable to load the package: %w", err)
		}
	}

	pterm.Println()
	pterm.Println()

	utils.ColorPrintYAML(p.cfg.Pkg)

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
