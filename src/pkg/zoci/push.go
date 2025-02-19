// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
)

// PublishPackage publishes the zarf package to the remote repository.
func (r *Remote) PublishPackage(ctx context.Context, pkg *v1alpha1.ZarfPackage, paths *layout.PackagePaths, concurrency int) (err error) {
	src, err := file.New(paths.Base)
	if err != nil {
		return err
	}
	defer func(src *file.Store) {
		err2 := src.Close()
		err = errors.Join(err, err2)
	}(src)

	r.Log().Info(fmt.Sprintf("Publishing package to %s", r.Repo().Reference))
	spinner := message.NewProgressSpinner("")
	defer spinner.Stop()

	// Get all the layers in the package
	var descs []ocispec.Descriptor
	for name, path := range paths.Files() {
		spinner.Updatef("Preparing layer %s", helpers.First30Last30(name))

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
	err = r.Repo().SetReferrersCapability(false)
	if err != nil {
		return err
	}

	// push the manifest config
	manifestConfigDesc, err := r.CreateAndPushManifestConfig(ctx, annotations, ZarfConfigMediaType)
	if err != nil {
		return err
	}
	root, err := r.PackAndTagManifest(ctx, src, descs, manifestConfigDesc, annotations)
	if err != nil {
		return err
	}

	total += manifestConfigDesc.Size

	progressBar := message.NewProgressBar(total, fmt.Sprintf("Publishing %s:%s", r.Repo().Reference.Repository, r.Repo().Reference.Reference))
	defer func(progressBar *message.ProgressBar) {
		err2 := progressBar.Close()
		err = errors.Join(err, err2)
	}(progressBar)
	r.SetProgressWriter(progressBar)
	defer r.ClearProgressWriter()

	publishedDesc, err := oras.Copy(ctx, src, root.Digest.String(), r.Repo(), "", copyOpts)
	if err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}
	if err := r.UpdateIndex(ctx, r.Repo().Reference.Reference, publishedDesc); err != nil {
		return fmt.Errorf("failed to update index: %w", err)
	}

	progressBar.Successf("Published %s [%s]", r.Repo().Reference, ZarfLayerMediaTypeBlob)
	return nil
}

func annotationsFromMetadata(metadata *v1alpha1.ZarfMetadata) map[string]string {
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
	// annotations explicitly defined in `metadata.annotations` take precedence over legacy fields
	maps.Copy(annotations, metadata.Annotations)
	return annotations
}
