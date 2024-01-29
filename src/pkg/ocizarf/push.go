// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package ocizarf

import (
	"context"
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/file"
)

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

		mediaType := oci.ZarfLayerMediaTypeBlob

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
	// manifestConfigDesc, err := o.pushManifestConfigFromMetadata(&pkg.Metadata, &pkg.Build)
	// if err != nil {
	// 	return err
	// }
	// root, err := o.generatePackManifest(src, descs, &manifestConfigDesc, &pkg.Metadata)
	// if err != nil {
	// 	return err
	// }
	// total += root.Size + manifestConfigDesc.Size

	progressBar := message.NewProgressBar(total, fmt.Sprintf("Publishing %s:%s", o.Repo().Reference.Repository, o.Repo().Reference.Reference))
	err = o.PublishPackage(ctx, src, pkg, descs, config.CommonOptions.OCIConcurrency, progressBar)
	if err != nil {
		progressBar.Stop()
		return fmt.Errorf("unable to publish package: %w", err)
	}

	// ?! Do I know the media type 100% at this point
	progressBar.Successf("Published %s [%s]", o.Repo().Reference, oci.ZarfLayerMediaTypeBlob)
	return nil
}
