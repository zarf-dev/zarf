// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"oras.land/oras-go/v2/content/file"
)

var (
	// PackageAlwaysPull is a list of paths that will always be pulled from the remote repository.
	PackageAlwaysPull = []string{layout.ZarfYAML, layout.Checksums, layout.Signature}
)

// PullPackage pulls the package from the remote repository and saves it to the given path.
func (r *Remote) PullPackage(ctx context.Context, destinationDir string, concurrency int, layersToPull ...ocispec.Descriptor) (_ []ocispec.Descriptor, err error) {
	layerSize := oci.SumDescsSize(layersToPull)
	// TODO (@austinabro321) change this and other r.Log() calls to the proper slog format
	r.Log().Info(fmt.Sprintf("Pulling %s, size: %s", r.Repo().Reference, utils.ByteFormat(float64(layerSize), 2)))

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

	err = r.CopyToTarget(ctx, layersToPull, dst, copyOpts)
	if err != nil {
		return nil, err
	}
	return layersToPull, nil
}

// AssembleLayers returns all layers for the given zarf package to pull from OCI.
func (r *Remote) AssembleLayers(ctx context.Context, requestedComponents []v1alpha1.ZarfComponent, isSkeleton bool) (map[string][]ocispec.Descriptor, error) {
	layerMap := make(map[string][]ocispec.Descriptor, 0)

	// fetching the root manifest is the common denominator for all layers
	root, err := r.FetchRoot(ctx)
	if err != nil {
		return nil, err
	}

	// Store all layers
	layerMap["all"] = root.Layers

	// We always pull the metadata layers provided we can locate them
	alwaysPull := make([]ocispec.Descriptor, 0)
	for _, path := range PackageAlwaysPull {
		desc := root.Locate(path)
		if !oci.IsEmptyDescriptor(desc) {
			alwaysPull = append(alwaysPull, desc)
		}
	}
	layerMap[layout.MetadataLayers] = alwaysPull
	// component layers are required for standard pulls and manifest inspects
	pkg, err := r.FetchZarfYAML(ctx)
	if err != nil {
		return nil, err
	}
	componentLayers, images, err := LayersFromComponents(root, pkg, requestedComponents)
	if err != nil {
		return nil, err
	}
	layerMap[layout.ComponentLayers] = componentLayers
	// there may not be any image layers - let's create the slice such that map key is present
	imageLayers := make([]ocispec.Descriptor, 0)
	if len(images) > 0 && !isSkeleton {
		// images layers are required for standard pulls
		index, err := r.FetchImagesIndex(ctx)
		if err != nil {
			return nil, err
		}
		imageLayers, err = r.LayersFromImages(ctx, root, index, images)
		if err != nil {
			return nil, err
		}
	}
	layerMap[layout.ImageLayers] = imageLayers
	// there may not be any sbom layers - let's create the slice such that map key is present
	sbomLayers := make([]ocispec.Descriptor, 0)
	sbomsDescriptor := root.Locate(layout.SBOMTar)
	if !oci.IsEmptyDescriptor(sbomsDescriptor) {
		sbomLayers = append(sbomLayers, sbomsDescriptor)
	}
	layerMap[layout.SbomLayers] = sbomLayers

	return layerMap, nil
}

// LayersFromComponents returns the layers for the given components to pull from OCI.
func LayersFromComponents(root *oci.Manifest, pkg v1alpha1.ZarfPackage, requestedComponents []v1alpha1.ZarfComponent) ([]ocispec.Descriptor, map[string]bool, error) {
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
		for _, image := range component.Images {
			images[image] = true
		}
		desc := root.Locate(filepath.Join(layout.ComponentsDir, fmt.Sprintf(tarballFormat, component.Name)))
		layers = append(layers, desc)
	}
	return layers, images, nil
}

// LayersFromImages returns the layers for the given images to pull from OCI.
func (r *Remote) LayersFromImages(ctx context.Context, root *oci.Manifest, index *ocispec.Index, images map[string]bool) ([]ocispec.Descriptor, error) {
	layers := make([]ocispec.Descriptor, 0)

	layers = append(layers, root.Locate(layout.IndexPath), root.Locate(layout.OCILayoutPath))

	for image := range images {
		// use docker's transform lib to parse the image ref
		// this properly mirrors the logic within create
		refInfo, err := transform.ParseImageRef(image)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image ref %q: %w", image, err)
		}

		manifestDescriptor := helpers.Find(index.Manifests, func(layer ocispec.Descriptor) bool {
			return layer.Annotations[ocispec.AnnotationBaseImageName] == refInfo.Reference ||
				// A backwards compatibility shim for older Zarf versions that would leave docker.io off of image annotations
				(layer.Annotations[ocispec.AnnotationBaseImageName] == refInfo.Path+refInfo.TagOrDigest && refInfo.Host == "docker.io")
		})

		// even though these are technically image manifests, we store them as Zarf blobs
		manifestDescriptor.MediaType = ZarfLayerMediaTypeBlob

		manifest, err := r.FetchManifest(ctx, manifestDescriptor)
		if err != nil {
			return nil, err
		}

		layers = append(layers, root.Locate(filepath.Join(layout.ImagesBlobsDir, manifestDescriptor.Digest.Encoded())))
		layers = append(layers, root.Locate(filepath.Join(layout.ImagesBlobsDir, manifest.Config.Digest.Encoded())))

		for _, layer := range manifest.Layers {
			layerPath := filepath.Join(layout.ImagesBlobsDir, layer.Digest.Encoded())
			layers = append(layers, root.Locate(layerPath))
		}
	}
	return layers, nil
}

// FilterLayers filters the layers based on the inspect target.
func FilterLayers(layerMap map[string][]ocispec.Descriptor, inspectTarget string) ([]ocispec.Descriptor, error) {
	layers := make([]ocispec.Descriptor, 0)

	switch inspectTarget {
	case "":
		layers = append(layers, layerMap[layout.MetadataLayers]...)
		layers = append(layers, layerMap[layout.ComponentLayers]...)
		layers = append(layers, layerMap[layout.ImageLayers]...)
		layers = append(layers, layerMap[layout.SbomLayers]...)
	case "metadata":
		layers = append(layers, layerMap[layout.MetadataLayers]...)
	case "manifests":
		layers = append(layers, layerMap[layout.MetadataLayers]...)
		layers = append(layers, layerMap[layout.ComponentLayers]...)
	case "sbom":
		layers = append(layers, layerMap[layout.MetadataLayers]...)
		layers = append(layers, layerMap[layout.SbomLayers]...)
	default:
		return nil, fmt.Errorf("unknown inspect target %s", inspectTarget)
	}
	return layers, nil
}

// PullPackageMetadata pulls the package metadata from the remote repository and saves it to `destinationDir`.
func (r *Remote) PullPackageMetadata(ctx context.Context, destinationDir string) ([]ocispec.Descriptor, error) {
	return r.PullPaths(ctx, destinationDir, PackageAlwaysPull)
}

// PullPackageSBOM pulls the package's sboms.tar from the remote repository and saves it to `destinationDir`.
func (r *Remote) PullPackageSBOM(ctx context.Context, destinationDir string) ([]ocispec.Descriptor, error) {
	return r.PullPaths(ctx, destinationDir, []string{layout.SBOMTar})
}
