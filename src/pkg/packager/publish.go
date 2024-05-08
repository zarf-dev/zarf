// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/creator"
	"github.com/defenseunicorns/zarf/src/pkg/packager/filters"
	"github.com/defenseunicorns/zarf/src/pkg/packager/sources"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/zoci"
	"github.com/defenseunicorns/zarf/src/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Publish publishes the package to a registry
func (p *Packager) Publish() (err error) {
	_, isOCISource := p.source.(*sources.OCISource)
	if isOCISource && p.cfg.PublishOpts.SigningKeyPath == "" {
		ctx := context.TODO()
		// oci --> oci is a special case, where we will use oci.CopyPackage so that we can transfer the package
		// w/o layers touching the filesystem
		srcRemote := p.source.(*sources.OCISource).Remote

		parts := strings.Split(srcRemote.Repo().Reference.Repository, "/")
		packageName := parts[len(parts)-1]

		p.cfg.PublishOpts.PackageDestination = p.cfg.PublishOpts.PackageDestination + "/" + packageName

		arch := config.GetArch()

		dstRemote, err := zoci.NewRemote(p.cfg.PublishOpts.PackageDestination, oci.PlatformForArch(arch))
		if err != nil {
			return err
		}

		return zoci.CopyPackage(ctx, srcRemote, dstRemote, config.CommonOptions.OCIConcurrency)
	}

	if p.cfg.CreateOpts.IsSkeleton {
		if err := os.Chdir(p.cfg.CreateOpts.BaseDir); err != nil {
			return fmt.Errorf("unable to access directory %q: %w", p.cfg.CreateOpts.BaseDir, err)
		}

		sc := creator.NewSkeletonCreator(p.cfg.CreateOpts, p.cfg.PublishOpts)

		if err := helpers.CreatePathAndCopy(layout.ZarfYAML, p.layout.ZarfYAML); err != nil {
			return err
		}

		p.cfg.Pkg, p.warnings, err = sc.LoadPackageDefinition(p.layout)
		if err != nil {
			return err
		}

		if err := sc.Assemble(p.layout, p.cfg.Pkg.Components, ""); err != nil {
			return err
		}

		if err := sc.Output(p.layout, &p.cfg.Pkg); err != nil {
			return err
		}
	} else {
		filter := filters.Empty()
		p.cfg.Pkg, p.warnings, err = p.source.LoadPackage(p.layout, filter, false)
		if err != nil {
			return fmt.Errorf("unable to load the package: %w", err)
		}

		// Sign the package if a key has been provided
		if err := p.layout.SignPackage(p.cfg.PublishOpts.SigningKeyPath, p.cfg.PublishOpts.SigningKeyPassword, !config.CommonOptions.Confirm); err != nil {
			return err
		}
	}

	// Get a reference to the registry for this package
	ref, err := zoci.ReferenceFromMetadata(p.cfg.PublishOpts.PackageDestination, &p.cfg.Pkg.Metadata, &p.cfg.Pkg.Build)
	if err != nil {
		return err
	}
	var platform ocispec.Platform
	if p.cfg.CreateOpts.IsSkeleton {
		platform = zoci.PlatformForSkeleton()
	} else {
		platform = oci.PlatformForArch(p.cfg.Pkg.Build.Architecture)
	}
	remote, err := zoci.NewRemote(ref, platform)
	if err != nil {
		return err
	}

	message.HeaderInfof("📦 PACKAGE PUBLISH %s:%s", p.cfg.Pkg.Metadata.Name, ref)

	// Publish the package/skeleton to the registry
	ctx := context.TODO()
	if err := remote.PublishPackage(ctx, &p.cfg.Pkg, p.layout, config.CommonOptions.OCIConcurrency); err != nil {
		return err
	}
	if p.cfg.CreateOpts.IsSkeleton {
		message.Title("How to import components from this skeleton:", "")
		ex := []types.ZarfComponent{}
		for _, c := range p.cfg.Pkg.Components {
			ex = append(ex, types.ZarfComponent{
				Name: fmt.Sprintf("import-%s", c.Name),
				Import: types.ZarfComponentImport{
					ComponentName: c.Name,
					URL:           helpers.OCIURLPrefix + remote.Repo().Reference.String(),
				},
			})
		}
		utils.ColorPrintYAML(ex, nil, true)
	}
	return nil
}
