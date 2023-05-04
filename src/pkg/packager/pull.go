// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry"
)

// Pull pulls a Zarf package and saves it as a compressed tarball.
func (p *Packager) Pull() error {
	err := p.handleOciPackage(p.cfg.DeployOpts.PackagePath, p.tmp.Base)
	if err != nil {
		return err
	}
	p.cfg.DeployOpts.PackagePath = p.tmp.Base
	err = utils.ReadYaml(p.tmp.ZarfYaml, &p.cfg.Pkg)
	if err != nil {
		return err
	}

	if err = p.validatePackageSignature(p.cfg.PullOpts.PublicKeyPath); err != nil {
		return err
	} else if !config.CommonOptions.Insecure {
		message.Successf("Package signature is valid")
	}

	if p.cfg.Pkg.Metadata.AggregateChecksum != "" {
		if err = p.validatePackageChecksums(); err != nil {
			return fmt.Errorf("unable to validate the package checksums: %w", err)
		}
	}

	// Get all the layers from within the temp directory
	allTheLayers, err := filepath.Glob(filepath.Join(p.tmp.Base, "*"))
	if err != nil {
		return err
	}

	name := fmt.Sprintf("zarf-package-%s-%s-%s.tar.zst", p.cfg.Pkg.Metadata.Name, p.cfg.Pkg.Build.Architecture, p.cfg.Pkg.Metadata.Version)
	err = archiver.Archive(allTheLayers, name)
	if err != nil {
		return err
	}
	return nil
}

// pullPackageSpecLayer pulls the `zarf.yaml` and `zarf.yaml.sig` (if it exists) layers from the published package
func (p *Packager) pullPackageLayers(packagePath string, targetDir string, layersToPull []string) error {
	ref, err := registry.ParseReference(strings.TrimPrefix(packagePath, "oci://"))
	if err != nil {
		return err
	}

	dst, err := utils.NewOrasRemote(ref)
	if err != nil {
		return err
	}

	// get the manifest
	layers, err := getLayers(dst)
	if err != nil {
		return err
	}

	for _, layerToPull := range layersToPull {
		layerDesc := utils.Find(layers, func(d ocispec.Descriptor) bool {
			return d.Annotations["org.opencontainers.image.title"] == layerToPull
		})
		if len(layerDesc.Digest) == 0 {
			return fmt.Errorf("unable to find layer (%s) from the OCI package %s", layerToPull, packagePath)
		}
		if err := pullLayer(dst, layerDesc, filepath.Join(targetDir, layerToPull)); err != nil {
			return fmt.Errorf("unable to pull the layer (%s) from the OCI package %s", layerToPull, packagePath)
		}
	}
	return nil
}
