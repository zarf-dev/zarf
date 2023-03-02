// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pterm/pterm"
	"oras.land/oras-go/v2/registry"
)

// Inspect list the contents of a package.
func (p *Packager) Inspect(includeSBOM bool, outputSBOM string) error {

	if err := p.loadZarfPkg(); err != nil {
		return fmt.Errorf("unable to load the package: %w", err)
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

// InspectOCIOutput is the output of the InspectOCI command
type InspectOCIOutput struct {
	Tags   []string `yaml:"tags"`
	Latest struct {
		Tag        string             `yaml:"tag"`
		Descriptor ocispec.Descriptor `yaml:"descriptor"`
	} `yaml:"latest"`
}

// InspectOCI inspects an OCI image and returns the tags and latest tag with descriptor
func (p *Packager) InspectOCI() error {
	message.Debug("packager.InspectOCI()")
	ref, err := registry.ParseReference(strings.TrimPrefix(p.cfg.DeployOpts.PackagePath, "oci://"))
	if err != nil {
		return err
	}

	dst, err := utils.NewOrasRemote(ref)
	if err != nil {
		return err
	}

	payload := InspectOCIOutput{}
	// get the tags
	err = dst.Tags(dst.Context, "", func(tags []string) error {
		for _, tag := range tags {
			// skeleton refs are not used during `zarf package deploy oci://`, but used within `zarf package create` w/ composition
			if strings.HasSuffix(tag, "-skeleton") {
				continue
			}
			payload.Tags = append(payload.Tags, tag)
		}
		return nil
	})
	if err != nil {
		return err
	}
	payload.Latest.Tag = payload.Tags[len(payload.Tags)-1]
	// get the manifest descriptor
	payload.Latest.Descriptor, err = dst.Resolve(dst.Context, payload.Latest.Tag)
	if err != nil {
		return err
	}

	utils.ColorPrintYAML(payload)
	pterm.Println()

	return nil
}
