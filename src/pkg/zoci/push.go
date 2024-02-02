// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"context"
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/file"
)

// PublishZarfPackage publishes the zarf package to the remote repository.
func (o *ZarfOrasRemote) PublishZarfPackage(ctx context.Context, pkg *types.ZarfPackage, paths *layout.PackagePaths, concurrency int) error {
	src, err := file.New(paths.Base)
	if err != nil {
		return err
	}
	defer src.Close()

	message.Infof("Publishing package to %s", o.Repo().Reference)
	spinner := message.NewProgressSpinner("")
	defer spinner.Stop()

	// Get all of the layers in the package
	var descs []ocispec.Descriptor
	for name, path := range paths.Files() {
		spinner.Updatef("Preparing layer %s", helpers.First30last30(name))

		mediaType := ZarfLayerMediaTypeBlob

		desc, err := src.Add(ctx, name, mediaType, path)
		if err != nil {
			return err
		}
		descs = append(descs, desc)
	}
	spinner.Successf("Prepared all layers")

	copyOpts := o.CopyOpts
	copyOpts.Concurrency = concurrency
	var total int64
	for _, desc := range descs {
		total += desc.Size
	}

	progressBar := message.NewProgressBar(total, fmt.Sprintf("Publishing %s:%s", o.Repo().Reference.Repository, o.Repo().Reference.Reference))
	annotations := annotationsFromMetadata(&pkg.Metadata)
	err = o.PublishArtifact(ctx, src, annotations, descs, config.CommonOptions.OCIConcurrency, progressBar)
	if err != nil {
		progressBar.Stop()
		return fmt.Errorf("unable to publish package: %w", err)
	}

	progressBar.Successf("Published %s [%s]", o.Repo().Reference, ZarfLayerMediaTypeBlob)
	return nil
}

func annotationsFromMetadata(metadata *types.ZarfMetadata) map[string]string {

	annotations := map[string]string{
		ocispec.AnnotationTitle:       metadata.Name,
		ocispec.AnnotationDescription: metadata.Description,
	}

	if url := metadata.URL; url != "" {
		annotations[ocispec.AnnotationURL] = url
	}
	if authors := metadata.Authors; authors != "" {
		annotations[ocispec.AnnotationAuthors] = authors
	}
	if documentation := metadata.Documentation; documentation != "" {
		annotations[ocispec.AnnotationDocumentation] = documentation
	}
	if source := metadata.Source; source != "" {
		annotations[ocispec.AnnotationSource] = source
	}
	if vendor := metadata.Vendor; vendor != "" {
		annotations[ocispec.AnnotationVendor] = vendor
	}

	return annotations
}
