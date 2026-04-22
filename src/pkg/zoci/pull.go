// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"oras.land/oras-go/v2/content/file"
)

var (
	// PackageAlwaysPull is a list of paths that will always be pulled from the remote repository.
	PackageAlwaysPull = []string{layout.ZarfYAML, layout.Checksums, layout.Signature, layout.Bundle}
)

// PullPackage pulls the package from the remote repository and saves it to the given path.
func (r *Remote) PullPackage(ctx context.Context, destinationDir string, concurrency int, layersToPull ...ocispec.Descriptor) (_ []ocispec.Descriptor, err error) {
	start := time.Now()
	// layersToPull is an explicit requirement for pulling package layers
	if len(layersToPull) == 0 {
		return nil, fmt.Errorf("no layers to pull")
	}

	if concurrency == 0 {
		concurrency = DefaultConcurrency
	}

	layerSize := oci.SumDescsSize(layersToPull)
	logger.From(ctx).Info("pulling package", "name", r.Repo().Reference, "size", utils.ByteFormat(float64(layerSize), 2))

	dst, err := file.New(destinationDir)
	if err != nil {
		return nil, err
	}
	defer func(dst *file.Store) {
		err2 := dst.Close()
		err = errors.Join(err, err2)
	}(dst)

	copyOpts := r.GetDefaultCopyOpts()
	copyOpts.Concurrency = concurrency

	trackedDst := images.NewTrackedTarget(dst, layerSize, images.DefaultReport(r.Log(), "package pull in progress", r.Repo().Reference.String()))
	trackedDst.StartReporting(ctx)
	defer trackedDst.StopReporting()

	err = r.CopyToTarget(ctx, layersToPull, trackedDst, copyOpts)
	if err != nil {
		return nil, err
	}
	r.Log().Info("finished pulling package layers", "duration", time.Since(start).Round(time.Millisecond*100))
	return layersToPull, nil
}

// AssembleLayers returns the OCI layer descriptors for the requested components.
// The include parameter specifies which layer types to return.
// All layers are included if include is empty and Metadata layers are always included
func (r *Remote) AssembleLayers(ctx context.Context, requestedComponents []v1alpha1.ZarfComponent, include ...LayerType) ([]ocispec.Descriptor, error) {
	root, err := r.FetchRoot(ctx)
	if err != nil {
		return nil, err
	}

	if len(include) == 0 {
		include = GetAllLayerTypes()
	}
	// Metadata layers are always included
	layers := make([]ocispec.Descriptor, 0)
	for _, path := range PackageAlwaysPull {
		desc := root.Locate(path)
		if !oci.IsEmptyDescriptor(desc) {
			layers = append(layers, desc)
		}
	}

	pkg, err := r.FetchZarfYAML(ctx)
	if err != nil {
		return nil, err
	}

	if slices.Contains(include, ComponentLayers) || slices.Contains(include, ImageLayers) {
		componentLayers, images, err := r.LayersFromComponents(ctx, pkg, requestedComponents)
		if err != nil {
			return nil, err
		}
		if slices.Contains(include, ComponentLayers) {
			layers = append(layers, componentLayers...)
		}
		if slices.Contains(include, ImageLayers) && len(images) > 0 {
			imageLayers, err := r.LayersFromImages(ctx, images)
			if err != nil {
				return nil, err
			}
			layers = append(layers, imageLayers...)
		}
	}

	if slices.Contains(include, SbomLayers) {
		desc := root.Locate(layout.SBOMTar)
		if !oci.IsEmptyDescriptor(desc) {
			layers = append(layers, desc)
		}
	}

	if slices.Contains(include, DocLayers) {
		if len(pkg.Documentation) > 0 {
			desc := root.Locate(layout.DocumentationTar)
			if !oci.IsEmptyDescriptor(desc) {
				layers = append(layers, desc)
			}
		}
	}

	return layers, nil
}

// LayersFromComponents returns the layers for the given components to pull from OCI.
func (r *Remote) LayersFromComponents(ctx context.Context, pkg v1alpha1.ZarfPackage, requestedComponents []v1alpha1.ZarfComponent) ([]ocispec.Descriptor, map[string]bool, error) {
	root, err := r.FetchRoot(ctx)
	if err != nil {
		return []ocispec.Descriptor{}, map[string]bool{}, err
	}

	layers := make([]ocispec.Descriptor, 0)

	images := map[string]bool{}
	tarballFormat := "%s.tar"
	for _, rc := range requestedComponents {
		component := helpers.Find(pkg.Components, func(component v1alpha1.ZarfComponent) bool {
			return component.Name == rc.Name
		})
		if component.Name == "" {
			return nil, nil, fmt.Errorf("component %s does not exist in this package", rc.Name)
		}
		for _, image := range component.GetImages() {
			images[image] = true
		}
		desc := root.Locate(filepath.Join(layout.ComponentsDir, fmt.Sprintf(tarballFormat, component.Name)))
		layers = append(layers, desc)
	}
	return layers, images, nil
}

// LayersFromImages returns the layers for the given images to pull from OCI.
func (r *Remote) LayersFromImages(ctx context.Context, imageList map[string]bool) ([]ocispec.Descriptor, error) {
	root, err := r.FetchRoot(ctx)
	if err != nil {
		return []ocispec.Descriptor{}, err
	}

	index, err := r.FetchImagesIndex(ctx)
	if err != nil {
		return nil, err
	}

	layers := make([]ocispec.Descriptor, 0)

	layers = append(layers, root.Locate(layout.IndexPath), root.Locate(layout.OCILayoutPath))

	for image := range imageList {
		// use docker's transform lib to parse the image ref
		// this properly mirrors the logic within create
		refInfo, err := transform.ParseImageRef(image)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image ref %q: %w", image, err)
		}

		entry := helpers.Find(index.Manifests, func(layer ocispec.Descriptor) bool {
			return layer.Annotations[ocispec.AnnotationBaseImageName] == refInfo.Reference ||
				// A backwards compatibility shim for older Zarf versions that would leave docker.io off of image annotations
				(layer.Annotations[ocispec.AnnotationBaseImageName] == refInfo.Path+refInfo.TagOrDigest && refInfo.Host == "docker.io")
		})

		isIndex := images.IsIndex(entry.MediaType)

		// Always include the entry's own blob (manifest or index).
		layers = append(layers, root.Locate(filepath.Join(layout.ImagesBlobsDir, entry.Digest.Encoded())))

		if isIndex {
			childLayers, err := r.layersFromIndexChildren(ctx, root, entry)
			if err != nil {
				return nil, err
			}
			layers = append(layers, childLayers...)
			continue
		}

		// even though these are technically image manifests, we store them as Zarf blobs
		entry.MediaType = ZarfLayerMediaTypeBlob

		manifest, err := r.FetchManifest(ctx, entry)
		if err != nil {
			return nil, err
		}

		layers = append(layers, root.Locate(filepath.Join(layout.ImagesBlobsDir, manifest.Config.Digest.Encoded())))
		for _, layer := range manifest.Layers {
			layers = append(layers, root.Locate(filepath.Join(layout.ImagesBlobsDir, layer.Digest.Encoded())))
		}
	}
	// Remove duplicate descriptors in case of shared base layers
	return oci.RemoveDuplicateDescriptors(layers), nil
}

// layersFromIndexChildren walks an OCI image index's children and returns every blob
// (child manifests, their configs, and their layers) that must be pulled alongside the index.
// Recurses into nested indexes — the OCI spec allows an index entry to point at either
// an image manifest or another image index.
func (r *Remote) layersFromIndexChildren(ctx context.Context, root *oci.Manifest, indexDesc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	idx, err := oci.FetchJSONFile[*ocispec.Index](ctx, r.FetchLayer, root, filepath.Join(layout.ImagesBlobsDir, indexDesc.Digest.Encoded()))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch child index %s: %w", indexDesc.Digest, err)
	}
	layers := make([]ocispec.Descriptor, 0, len(idx.Manifests))
	for _, child := range idx.Manifests {
		layers = append(layers, root.Locate(filepath.Join(layout.ImagesBlobsDir, child.Digest.Encoded())))
		switch {
		case images.IsIndex(child.MediaType):
			nestedLayers, err := r.layersFromIndexChildren(ctx, root, child)
			if err != nil {
				return nil, err
			}
			layers = append(layers, nestedLayers...)
		case images.IsManifest(child.MediaType):
			childWithBlobType := child
			childWithBlobType.MediaType = ZarfLayerMediaTypeBlob
			childManifest, err := r.FetchManifest(ctx, childWithBlobType)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch child manifest %s: %w", child.Digest, err)
			}
			if childManifest.Config.Digest != "" {
				layers = append(layers, root.Locate(filepath.Join(layout.ImagesBlobsDir, childManifest.Config.Digest.Encoded())))
			}
			for _, layer := range childManifest.Layers {
				layers = append(layers, root.Locate(filepath.Join(layout.ImagesBlobsDir, layer.Digest.Encoded())))
			}
		default:
			return nil, fmt.Errorf("unexpected media type %q for index child %s", child.MediaType, child.Digest)
		}
	}
	return layers, nil
}
