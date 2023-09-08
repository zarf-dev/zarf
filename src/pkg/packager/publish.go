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
	"github.com/defenseunicorns/zarf/src/pkg/packager/sources"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// Publish publishes the package to a registry
func (p *Packager) Publish() (err error) {
	var referenceSuffix string
	if utils.IsDir(p.cfg.PkgOpts.PackageSource) {
		referenceSuffix = oci.SkeletonSuffix
		err := p.loadSkeleton()
		if err != nil {
			return err
		}
	} else {
		if p.source == nil {
			p.source, err = sources.New(&p.cfg.PkgOpts, p.tmp)
			if err != nil {
				return err
			}
		}

		p.cfg.Pkg, p.tmp, err = p.source.LoadPackage()
		if err != nil {
			return err
		}

		referenceSuffix = config.GetArch(p.cfg.Pkg.Metadata.Architecture, p.cfg.Pkg.Build.Architecture)
	}

	// Get a reference to the registry for this package
	ref, err := oci.ReferenceFromMetadata(p.cfg.PublishOpts.PackageDestination, &p.cfg.Pkg.Metadata, referenceSuffix)
	if err != nil {
		return err
	}

	err = p.SetOCIRemote(ref)
	if err != nil {
		return err
	}

	// Sign the package if a key has been provided
	if p.cfg.PublishOpts.SigningKeyPath != "" {
		if err := p.signPackage(p.cfg.PublishOpts.SigningKeyPath, p.cfg.PublishOpts.SigningKeyPassword); err != nil {
			return err
		}
	}

	message.HeaderInfof("📦 PACKAGE PUBLISH %s:%s", p.cfg.Pkg.Metadata.Name, ref)

	// Publish the package/skeleton to the registry
	if err := p.remote.PublishPackage(&p.cfg.Pkg, p.tmp, config.CommonOptions.OCIConcurrency); err != nil {
		return err
	}
	if strings.HasSuffix(p.remote.Repo().Reference.String(), oci.SkeletonSuffix) {
		message.Title("How to import components from this skeleton:", "")
		ex := []types.ZarfComponent{}
		for _, c := range p.cfg.Pkg.Components {
			ex = append(ex, types.ZarfComponent{
				Name: fmt.Sprintf("import-%s", c.Name),
				Import: types.ZarfComponentImport{
					ComponentName: c.Name,
					URL:           fmt.Sprintf("oci://%s", p.remote.Repo().Reference),
				},
			})
		}
		utils.ColorPrintYAML(ex, nil, true)
	}
	return nil
}

func (p *Packager) loadSkeleton() error {
	base, err := filepath.Abs(p.cfg.PkgOpts.PackageSource)
	if err != nil {
		return err
	}
	if err := os.Chdir(base); err != nil {
		return err
	}
	if err := p.readYaml(types.ZarfYAML); err != nil {
		return fmt.Errorf("unable to read the zarf.yaml in %s: %s", base, err.Error())
	}

	if p.cfg.Pkg.Kind == types.ZarfInitConfig {
		p.cfg.Pkg.Metadata.Version = config.CLIVersion
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
	p.tmp.Unset(types.ComponentsDir)

	checksumChecksum, err := p.generatePackageChecksums()
	if err != nil {
		return fmt.Errorf("unable to generate checksums for skeleton package: %w", err)
	}
	p.cfg.Pkg.Metadata.AggregateChecksum = checksumChecksum

	return p.writeYaml()
}
