// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
)

// Pull pulls a Zarf package and saves it as a compressed tarball.
func (p *Packager) Pull() error {
	err := p.SetOCIRemote(p.cfg.PullOpts.PackageSource)
	if err != nil {
		return err
	}
	_, err = p.remote.PullPackage(p.tmp.Base, config.CommonOptions.OCIConcurrency)
	if err != nil {
		return err
	}
	err = p.readYaml(p.tmp.ZarfYaml)
	if err != nil {
		return err
	}

	if err = p.validatePackageSignature(p.cfg.PullOpts.PublicKeyPath); err != nil {
		return err
	} else if !config.CommonOptions.Insecure {
		message.Successf("Package signature is valid")
	}

	if err = p.validatePackageChecksums(p.tmp.Base, p.cfg.Pkg.Metadata.AggregateChecksum, nil); err != nil {
		return fmt.Errorf("unable to validate the package checksums: %w", err)
	}

	// Get all the layers from within the temp directory
	allTheLayers, err := filepath.Glob(filepath.Join(p.tmp.Base, "*"))
	if err != nil {
		return err
	}

	var name string
	if strings.HasSuffix(p.cfg.PullOpts.PackageSource, oci.SkeletonSuffix) {
		name = fmt.Sprintf("zarf-package-%s-skeleton-%s.tar.zst", p.cfg.Pkg.Metadata.Name, p.cfg.Pkg.Metadata.Version)
	} else {
		name = fmt.Sprintf("zarf-package-%s-%s-%s.tar.zst", p.cfg.Pkg.Metadata.Name, p.cfg.Pkg.Build.Architecture, p.cfg.Pkg.Metadata.Version)
	}
	output := filepath.Join(p.cfg.PullOpts.OutputDirectory, name)
	_ = os.Remove(output)
	err = archiver.Archive(allTheLayers, output)
	if err != nil {
		return err
	}
	return nil
}
