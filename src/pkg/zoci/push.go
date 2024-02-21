// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"context"
	"fmt"

	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
)

// PublishPackage publishes the zarf package to the remote repository.
func (r *Remote) PublishPackage(ctx context.Context, pkg *types.ZarfPackage, paths *layout.PackagePaths, concurrency int) error {
	src, err := file.New(paths.Base)
	if err != nil {
		return err
	}
	defer src.Close()

	r.Log().Info(fmt.Sprintf("Publishing package to %s", r.Repo().Reference))
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

	copyOpts := r.GetDefaultCopyOpts()
	copyOpts.Concurrency = concurrency
	total := oci.SumDescsSize(descs)

	annotations := annotationsFromMetadata(&pkg.Metadata)

	// assumes referrers API is not supported since OCI artifact
	// media type is not supported
	r.Repo().SetReferrersCapability(false)

	// push the manifest config
	// since this config is so tiny, and the content is not used again
	// it is not logged to the progress, but will error if it fails
	manifestConfigDesc, err := r.PushManifestConfigFromMetadata(ctx, annotations, ZarfConfigMediaType)
	if err != nil {
		return err
	}
	root, err := r.CreateAndPushManifest(ctx, src, descs, manifestConfigDesc, annotations)
	if err != nil {
		return err
	}

	progressBar := message.NewProgressBar(total, fmt.Sprintf("Publishing %s:%s", r.Repo().Reference.Repository, r.Repo().Reference.Reference))
	r.Transport.ProgressBar = progressBar

	publishedDesc, err := oras.Copy(ctx, src, root.Digest.String(), r.Repo(), "", copyOpts)
	if err != nil {
		return err
	}

	if err := r.UpdateIndex(ctx, r.Repo().Reference.Reference, publishedDesc); err != nil {
		return err
	}

	if err != nil {
		progressBar.Stop()
		return fmt.Errorf("unable to publish package: %w", err)
	}

	progressBar.Successf("Published %s [%s]", r.Repo().Reference, ZarfLayerMediaTypeBlob)
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
