// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"sort"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	layout2 "github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
)

// OCITimestampFormat is the format used for the OCI timestamp annotation
const OCITimestampFormat = time.RFC3339

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

	// Get all the layers in the package
	var descs []ocispec.Descriptor
	for name, path := range paths.Files() {
		mediaType := ZarfLayerMediaTypeBlob

		desc, err := src.Add(ctx, name, mediaType, path)
		if err != nil {
			return err
		}
		descs = append(descs, desc)
	}

	copyOpts := r.GetDefaultCopyOpts()
	copyOpts.Concurrency = concurrency

	annotations := annotationsFromMetadata(pkg.Metadata)

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

	publishedDesc, err := oras.Copy(ctx, src, root.Digest.String(), r.Repo(), "", copyOpts)
	if err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}

	return r.UpdateIndex(ctx, r.Repo().Reference.Reference, publishedDesc)
}

// PushPackage publishes the zarf package to the remote repository.
func (r *Remote) PushPackage(ctx context.Context, pkgLayout *layout2.PackageLayout, concurrency int) (err error) {
	logger.From(ctx).Info("pushing package to registry",
		"destination", r.OrasRemote.Repo().Reference.String(),
		"architecture", pkgLayout.Pkg.Build.Architecture)

	src, err := file.New("")
	if err != nil {
		return err
	}
	defer func(src *file.Store) {
		err2 := src.Close()
		err = errors.Join(err, err2)
	}(src)

	descs := []ocispec.Descriptor{}
	files, err := pkgLayout.Files()
	if err != nil {
		return err
	}
	for path, name := range files {
		desc, err := src.Add(ctx, name, ZarfLayerMediaTypeBlob, path)
		if err != nil {
			return err
		}
		descs = append(descs, desc)
	}

	// Sort by Digest string
	sort.Slice(descs, func(i, j int) bool {
		return descs[i].Digest < descs[j].Digest
	})

	annotations := annotationsFromMetadata(pkgLayout.Pkg.Metadata)

	// Perform the conversion of the string timestamp to the appropriate format in order to maintain backwards compatibility
	t, err := time.Parse(v1alpha1.BuildTimestampFormat, pkgLayout.Pkg.Build.Timestamp)
	if err != nil {
		// if we change the format of the timestamp, we need to update the conversion here
		// and also account for an error state for mismatch with older formats
		return fmt.Errorf("unable to parse timestamp: %w", err)
	}
	annotations[ocispec.AnnotationCreated] = t.Format(OCITimestampFormat)

	manifestConfigDesc, err := r.OrasRemote.CreateAndPushManifestConfig(ctx, annotations, ZarfConfigMediaType)
	if err != nil {
		return err
	}
	// here is where the manifest is created and written to the filesystem given the file.store Push() functionality
	root, err := r.OrasRemote.PackAndTagManifest(ctx, src, descs, manifestConfigDesc, annotations)
	if err != nil {
		return err
	}

	defer func() {
		// remove the dangling manifest file created by the PackAndTagManifest
		// should this behavior change, we should expect this to begin producing an error
		err2 := os.Remove(pkgLayout.Pkg.Metadata.Name)
		err = errors.Join(err, err2)
	}()

	copyOpts := r.OrasRemote.GetDefaultCopyOpts()
	copyOpts.Concurrency = concurrency
	publishedDesc, err := oras.Copy(ctx, src, root.Digest.String(), r.OrasRemote.Repo(), "", copyOpts)
	if err != nil {
		return err
	}

	err = r.OrasRemote.UpdateIndex(ctx, r.OrasRemote.Repo().Reference.Reference, publishedDesc)
	if err != nil {
		return err
	}

	return nil
}

func annotationsFromMetadata(metadata v1alpha1.ZarfMetadata) map[string]string {
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
