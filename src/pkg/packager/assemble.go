// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying zarf packages.
package packager

import (
	"fmt"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/images"
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
)

var (
	// verify that SkeletonAssembler implements Assembler
	_ Assembler = (*SkeletonAssembler)(nil)

	// verify that PackageAssembler implements Assembler
	_ Assembler = (*PackageAssembler)(nil)
)

// Assembler is an interface for assembling package assets (components, images, SBOMSs, etc) during package create.
type Assembler interface {
	Assemble(*Packager) error
}

// SkeletonAssembler is used to assemble assets for skeleton Zarf packages during package create.
type SkeletonAssembler struct{}

// Assemble assembles assets for skeleton Zarf packages during package create.
func (*SkeletonAssembler) Assemble(p *Packager) error {
	if err := p.skeletonizeExtensions(); err != nil {
		return err
	}
	for _, warning := range p.warnings {
		message.Warn(warning)
	}
	for idx, component := range p.cfg.Pkg.Components {
		if err := p.addComponent(idx, component); err != nil {
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

// PackageAssembler is used to assemble assets for normal (not skeleton) Zarf packages during package create.
type PackageAssembler struct{}

// Assemble assembles assets for normal (not skeleton) Zarf packages during package create.
func (*PackageAssembler) Assemble(p *Packager) error {
	componentSBOMs := map[string]*layout.ComponentSBOM{}
	var imageList []transform.Image
	for idx, component := range p.cfg.Pkg.Components {
		onCreate := component.Actions.OnCreate
		onFailure := func() {
			if err := p.runActions(onCreate.Defaults, onCreate.OnFailure, nil); err != nil {
				message.Debugf("unable to run component failure action: %s", err.Error())
			}
		}
		if err := p.addComponent(idx, component); err != nil {
			onFailure()
			return fmt.Errorf("unable to add component %q: %w", component.Name, err)
		}

		if err := p.runActions(onCreate.Defaults, onCreate.OnSuccess, nil); err != nil {
			onFailure()
			return fmt.Errorf("unable to run component success action: %w", err)
		}

		if !p.cfg.CreateOpts.SkipSBOM {
			componentSBOM, err := p.getFilesToSBOM(component)
			if err != nil {
				return fmt.Errorf("unable to create component SBOM: %w", err)
			}
			if componentSBOM != nil && len(componentSBOM.Files) > 0 {
				componentSBOMs[component.Name] = componentSBOM
			}
		}

		// Combine all component images into a single entry for efficient layer reuse.
		for _, src := range component.Images {
			refInfo, err := transform.ParseImageRef(src)
			if err != nil {
				return fmt.Errorf("failed to create ref for image %s: %w", src, err)
			}
			imageList = append(imageList, refInfo)
		}
	}

	imageList = helpers.Unique(imageList)
	var sbomImageList []transform.Image

	// Images are handled separately from other component assets.
	if len(imageList) > 0 {
		message.HeaderInfof("📦 PACKAGE IMAGES")

		p.layout = p.layout.AddImages()

		var pulled []images.ImgInfo
		var err error

		doPull := func() error {
			imgConfig := images.ImageConfig{
				ImagesPath:        p.layout.Images.Base,
				ImageList:         imageList,
				Insecure:          config.CommonOptions.Insecure,
				Architectures:     []string{p.cfg.Pkg.Metadata.Architecture, p.cfg.Pkg.Build.Architecture},
				RegistryOverrides: p.cfg.CreateOpts.RegistryOverrides,
			}

			pulled, err = imgConfig.PullAll()
			return err
		}

		if err := helpers.Retry(doPull, 3, 5*time.Second, message.Warnf); err != nil {
			return fmt.Errorf("unable to pull images after 3 attempts: %w", err)
		}

		for _, imgInfo := range pulled {
			if err := p.layout.Images.AddV1Image(imgInfo.Img); err != nil {
				return err
			}
			if imgInfo.HasImageLayers {
				sbomImageList = append(sbomImageList, imgInfo.RefInfo)
			}
		}
	}

	// Ignore SBOM creation if the flag is set.
	if p.cfg.CreateOpts.SkipSBOM {
		message.Debug("Skipping image SBOM processing per --skip-sbom flag")
	} else {
		p.layout = p.layout.AddSBOMs()
		if err := sbom.Catalog(componentSBOMs, sbomImageList, p.layout); err != nil {
			return fmt.Errorf("unable to create an SBOM catalog for the package: %w", err)
		}
	}

	return nil
}
