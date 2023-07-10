// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
)

// Publish publishes the package to a registry
//
// This is a wrapper around the oras library
// and much of the code was adapted from the oras CLI - https://github.com/oras-project/oras/blob/main/cmd/oras/push.go
//
// Authentication is handled via the Docker config file created w/ `zarf tools registry login`
func (p *Packager) Publish() error {
	var referenceSuffix string
	if utils.IsDir(p.cfg.PublishOpts.PackagePath) {
		referenceSuffix = oci.SkeletonSuffix
		err := p.loadSkeleton()
		if err != nil {
			return err
		}
	} else {
		// Extract the first layer of the tarball
		if err := archiver.Unarchive(p.cfg.PublishOpts.PackagePath, p.tmp.Base); err != nil {
			return fmt.Errorf("unable to extract the package: %w", err)
		}

		err := p.readYaml(p.tmp.ZarfYaml)
		if err != nil {
			return fmt.Errorf("unable to read the zarf.yaml in %s: %w", p.tmp.Base, err)
		}
		referenceSuffix = p.arch
	}

	// Get a reference to the registry for this package
	ref, err := oci.ReferenceFromMetadata(p.cfg.PublishOpts.PackageDestination, &p.cfg.Pkg.Metadata, referenceSuffix)
	if err != nil {
		return err
	}

	err = p.SetOCIRemote(ref.String())
	if err != nil {
		return err
	}

	if err := p.validatePackageChecksums(p.tmp.Base, p.cfg.Pkg.Metadata.AggregateChecksum, nil); err != nil {
		return fmt.Errorf("unable to publish package because checksums do not match: %w", err)
	}

	// Sign the package if a key has been provided
	if p.cfg.PublishOpts.SigningKeyPath != "" {
		_, err := utils.CosignSignBlob(p.tmp.ZarfYaml, p.tmp.ZarfSig, p.cfg.PublishOpts.SigningKeyPath, p.getSigPublishPassword)
		if err != nil {
			return fmt.Errorf("unable to sign the package: %w", err)
		}
	}

	message.HeaderInfof("📦 PACKAGE PUBLISH %s:%s", p.cfg.Pkg.Metadata.Name, ref)

	// Publish the package/skeleton to the registry
	return p.remote.PublishPackage(&p.cfg.Pkg, p.tmp.Base, config.CommonOptions.OCIConcurrency)
}

func (p *Packager) loadSkeleton() error {
	base, err := filepath.Abs(p.cfg.PublishOpts.PackagePath)
	if err != nil {
		return err
	}
	if err := os.Chdir(base); err != nil {
		return err
	}
	if err := p.readYaml(config.ZarfYAML); err != nil {
		return fmt.Errorf("unable to read the zarf.yaml in %s: %s", base, err.Error())
	}

	err = p.composeComponents()
	if err != nil {
		return err
	}

	err = p.skeletonizeExtensions()
	if err != nil {
		return err
	}

	for _, warning := range p.warnings {
		message.Warn(warning)
	}

	for idx, component := range p.cfg.Pkg.Components {
		isSkeleton := true
		err := p.addComponent(idx, component, isSkeleton)
		if err != nil {
			return err
		}

		err = p.archiveComponent(component)
		if err != nil {
			return fmt.Errorf("unable to archive component: %s", err.Error())
		}
	}

	checksumChecksum, err := generatePackageChecksums(p.tmp.Base)
	if err != nil {
		return fmt.Errorf("unable to generate checksums for skeleton package: %w", err)
	}
	p.cfg.Pkg.Metadata.AggregateChecksum = checksumChecksum

	return p.writeYaml()
}
