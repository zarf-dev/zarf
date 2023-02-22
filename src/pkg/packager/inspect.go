// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pterm/pterm"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
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

type InspectOCIOutput struct {
	Tags   []string `yaml:"tags"`
	Latest struct {
		Tag        string             `yaml:"tag"`
		Descriptor ocispec.Descriptor `yaml:"descriptor"`
	} `yaml:"latest"`
}

func (p *Packager) InspectOCI() error {
	message.Debug("packager.InspectOCI()")
	ref, err := registry.ParseReference(strings.TrimPrefix(p.cfg.DeployOpts.PackagePath, "oci://"))
	if err != nil {
		return err
	}

	// patch docker.io to registry-1.docker.io
	if ref.Registry == "docker.io" {
		ref.Registry = "registry-1.docker.io"
	}
	ctx := p.orasCtxWithScopes(ref)
	repo, err := remote.NewRepository(ref.String())
	if err != nil {
		return err
	}
	repo.PlainHTTP = config.CommonOptions.Insecure
	authClient, err := p.orasAuthClient(ref)
	if err != nil {
		return err
	}
	repo.Client = authClient

	payload := InspectOCIOutput{}
	// get the tags
	err = repo.Tags(ctx, "", func(tags []string) error {
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
	payload.Latest.Tag = payload.Tags[0]
	// get the manifest descriptor
	payload.Latest.Descriptor, err = repo.Resolve(ctx, payload.Latest.Tag)
	if err != nil {
		return err
	}

	utils.ColorPrintYAML(payload)

	return nil
}	
