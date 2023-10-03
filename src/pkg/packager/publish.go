// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/packager/sources"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

// Publish publishes the package to a registry
func (p *Packager) Publish() (err error) {
	_, isOCISource := p.source.(*sources.OCISource)
	if isOCISource {
		ctx := context.TODO()
		// oci --> oci is a special case, where we will use oci.CopyPackage so that we can transfer the package
		// w/o layers touching the filesystem
		srcRemote := p.source.(*sources.OCISource).OrasRemote
		srcRemote.WithContext(ctx)

		parts := strings.Split(srcRemote.Repo().Reference.Repository, "/")
		packageName := parts[len(parts)-1]

		p.cfg.PublishOpts.PackageDestination = p.cfg.PublishOpts.PackageDestination + "/" + packageName

		err = p.setOCIRemote(p.cfg.PublishOpts.PackageDestination)
		if err != nil {
			return err
		}
		p.remote.WithContext(ctx)

		if err := oci.CopyPackage(ctx, srcRemote, p.remote, nil, config.CommonOptions.OCIConcurrency); err != nil {
			return err
		}

		srcManifest, err := srcRemote.FetchRoot()
		if err != nil {
			return err
		}
		b, err := srcManifest.MarshalJSON()
		if err != nil {
			return err
		}
		expected := content.NewDescriptorFromBytes(ocispec.MediaTypeImageManifest, b)

		// tag the manifest the same as the source
		if err := p.remote.Repo().Manifests().PushReference(ctx, expected, bytes.NewReader(b), srcRemote.Repo().Reference.Reference); err != nil {
			return err
		}
		message.Infof("Published %s to %s", srcRemote.Repo().Reference, p.remote.Repo().Reference)
		return nil
	}

	var referenceSuffix string
	if p.cfg.CreateOpts.BaseDir != "" {
		referenceSuffix = oci.SkeletonSuffix
		err := p.loadSkeleton()
		if err != nil {
			return err
		}
	} else {
		if err = p.source.LoadPackage(p.layout); err != nil {
			return fmt.Errorf("unable to load the package: %w", err)
		}
		if err = p.readZarfYAML(p.layout.ZarfYAML); err != nil {
			return err
		}

		referenceSuffix = p.arch
	}

	// Get a reference to the registry for this package
	ref, err := oci.ReferenceFromMetadata(p.cfg.PublishOpts.PackageDestination, &p.cfg.Pkg.Metadata, referenceSuffix)
	if err != nil {
		return err
	}

	err = p.setOCIRemote(ref)
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
	if err := p.remote.PublishPackage(&p.cfg.Pkg, p.layout, config.CommonOptions.OCIConcurrency); err != nil {
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
					URL:           helpers.OCIURLPrefix + p.remote.Repo().Reference.String(),
				},
			})
		}
		utils.ColorPrintYAML(ex, nil, true)
	}
	return nil
}

func (p *Packager) loadSkeleton() (err error) {
	if err := os.Chdir(p.cfg.CreateOpts.BaseDir); err != nil {
		return err
	}
	if err = p.readZarfYAML(layout.ZarfYAML); err != nil {
		return fmt.Errorf("unable to read the zarf.yaml file: %s", err.Error())
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
		if err := p.addComponent(idx, component, isSkeleton); err != nil {
			return err
		}

		if err := p.layout.Components.Archive(component, false); err != nil {
			return err
		}
	}

	checksumChecksum, err := p.generatePackageChecksums()
	if err != nil {
		return fmt.Errorf("unable to generate checksums for skeleton package: %w", err)
	}
	p.cfg.Pkg.Metadata.AggregateChecksum = checksumChecksum

	return p.writeYaml()
}
