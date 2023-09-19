// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
)

var (
	// PackageAlwaysPull is a list of paths that will always be pulled from the remote repository.
	PackageAlwaysPull = []string{config.ZarfYAML, config.ZarfChecksumsTxt, config.ZarfYAMLSignature}
)

// FileDescriptorExists returns true if the given file exists in the given directory with the expected SHA.
func (o *OrasRemote) FileDescriptorExists(desc ocispec.Descriptor, destinationDir string) bool {
	destinationPath := filepath.Join(destinationDir, desc.Annotations[ocispec.AnnotationTitle])
	info, err := os.Stat(destinationPath)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	if info.Size() != desc.Size {
		return false
	}

	f, err := os.Open(destinationPath)
	if err != nil {
		return false
	}
	defer f.Close()

	actual, err := helpers.GetSHA256Hash(f)
	if err != nil {
		return false
	}
	return actual == desc.Digest.Encoded()
}

// LayersFromPaths returns the descriptors for the given paths from the root manifest.
func (o *OrasRemote) LayersFromPaths(requestedPaths []string) (layers []ocispec.Descriptor, err error) {
	manifest, err := o.FetchRoot()
	if err != nil {
		return nil, err
	}
	for _, path := range requestedPaths {
		layer := manifest.Locate(path)
		if IsEmptyDescriptor(layer) {
			return nil, fmt.Errorf("path %s does not exist in this package", path)
		}
		layers = append(layers, layer)
	}
	return layers, nil
}

// LayersFromRequestedComponents returns the descriptors for the given components from the root manifest.
//
// It also retrieves the descriptors for all image layers that are required by the components.
//
// It also respects the `required` flag on components, and will retrieve all necessary layers for required components.
func (o *OrasRemote) LayersFromRequestedComponents(requestedComponents []string) (layers []ocispec.Descriptor, err error) {
	root, err := o.FetchRoot()
	if err != nil {
		return nil, err
	}

	pkg, err := o.FetchZarfYAML(root)
	if err != nil {
		return nil, err
	}
	images := map[string]bool{}
	tarballFormat := "%s.tar"
	for _, name := range requestedComponents {
		component := helpers.Find(pkg.Components, func(component types.ZarfComponent) bool {
			return component.Name == name
		})
		if component.Name == "" {
			return nil, fmt.Errorf("component %s does not exist in this package", name)
		}
	}
	for _, component := range pkg.Components {
		// If we requested this component, or it is required, we need to pull its images and tarball
		if helpers.SliceContains(requestedComponents, component.Name) || component.Required {
			for _, image := range component.Images {
				images[image] = true
			}
			layers = append(layers, root.Locate(filepath.Join(config.ZarfComponentsDir, fmt.Sprintf(tarballFormat, component.Name))))
		}
	}
	// Append the sboms.tar layer if it exists
	//
	// Since sboms.tar is not a heavy addition 99% of the time, we'll just always pull it
	sbomsDescriptor := root.Locate(config.ZarfSBOMTar)
	if !IsEmptyDescriptor(sbomsDescriptor) {
		layers = append(layers, sbomsDescriptor)
	}
	if len(images) > 0 {
		// Add the image index and the oci-layout layers
		layers = append(layers, root.Locate(ZarfPackageIndexPath), root.Locate(ZarfPackageLayoutPath))
		index, err := o.FetchImagesIndex(root)
		if err != nil {
			return nil, err
		}
		for image := range images {
			manifestDescriptor := helpers.Find(index.Manifests, func(layer ocispec.Descriptor) bool {
				return layer.Annotations[ocispec.AnnotationBaseImageName] == image
			})

			// even though these are technically image manifests, we store them as Zarf blobs
			manifestDescriptor.MediaType = ZarfLayerMediaTypeBlob

			manifest, err := o.FetchManifest(manifestDescriptor)
			if err != nil {
				return nil, err
			}
			// Add the manifest and the manifest config layers
			layers = append(layers, root.Locate(filepath.Join(ZarfPackageImagesBlobsDir, manifestDescriptor.Digest.Encoded())))
			layers = append(layers, root.Locate(filepath.Join(ZarfPackageImagesBlobsDir, manifest.Config.Digest.Encoded())))

			// Add all the layers from the manifest
			for _, layer := range manifest.Layers {
				layerPath := filepath.Join(ZarfPackageImagesBlobsDir, layer.Digest.Encoded())
				layers = append(layers, root.Locate(layerPath))
			}
		}
	}
	return layers, nil
}

// PullPackage pulls the package from the remote repository and saves it to the given path.
//
// layersToPull is an optional parameter that allows the caller to specify which layers to pull.
//
// The following layers will ALWAYS be pulled if they exist:
//   - zarf.yaml
//   - checksums.txt
//   - zarf.yaml.sig
func (o *OrasRemote) PullPackage(destinationDir string, concurrency int, layersToPull ...ocispec.Descriptor) (partialPaths []string, err error) {
	isPartialPull := len(layersToPull) > 0
	ref := o.repo.Reference

	message.Debug("Pulling", ref)

	manifest, err := o.FetchRoot()
	if err != nil {
		return partialPaths, err
	}

	if isPartialPull {
		for _, path := range PackageAlwaysPull {
			desc := manifest.Locate(path)
			layersToPull = append(layersToPull, desc)
		}
	} else {
		layersToPull = append(layersToPull, manifest.Layers...)
	}
	layersToPull = append(layersToPull, manifest.Config)

	// de-duplicate layers
	layersToPull = RemoveDuplicateDescriptors(layersToPull)

	dst, err := file.New(destinationDir)
	if err != nil {
		return partialPaths, err
	}
	defer dst.Close()

	copyOpts := o.CopyOpts
	copyOpts.Concurrency = concurrency
	if isPartialPull {
		for _, layer := range layersToPull {
			path := layer.Annotations[ocispec.AnnotationTitle]
			// partial paths are relative to the destination directory
			// only layers w/ a title annotation are considered
			if len(path) > 0 {
				partialPaths = append(partialPaths, path)
			}
		}
		partialPaths = helpers.Unique(partialPaths)
	}

	return partialPaths, o.CopyWithProgress(layersToPull, dst, &copyOpts, destinationDir)
}

// CopyWithProgress copies the given layers from the remote repository to the given store.
func (o *OrasRemote) CopyWithProgress(layers []ocispec.Descriptor, store oras.Target, copyOpts *oras.CopyOptions, destinationDir string) error {
	estimatedBytes := int64(0)
	shas := []string{}
	for _, layer := range layers {
		estimatedBytes += layer.Size
		if len(layer.Digest.String()) > 0 {
			shas = append(shas, layer.Digest.Encoded())
		}
	}

	copyOpts.FindSuccessors = func(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		nodes, err := content.Successors(ctx, fetcher, desc)
		if err != nil {
			return nil, err
		}
		var ret []ocispec.Descriptor
		for _, node := range nodes {
			if helpers.SliceContains(shas, node.Digest.Encoded()) {
				ret = append(ret, node)
			}
		}
		return ret, nil
	}

	// Create a thread to update a progress bar as we save the package to disk
	doneSaving := make(chan int)
	var wg sync.WaitGroup
	wg.Add(1)
	go utils.RenderProgressBarForLocalDirWrite(destinationDir, estimatedBytes, &wg, doneSaving, "Pulling")
	_, err := oras.Copy(o.ctx, o.repo, o.repo.Reference.String(), store, o.repo.Reference.String(), *copyOpts)
	if err != nil {
		return err
	}

	// Send a signal to the progress bar that we're done and wait for it to finish
	doneSaving <- 1
	wg.Wait()

	return nil
}

// PullLayer pulls a layer from the remote repository and saves it to `destinationDir/annotationTitle`.
func (o *OrasRemote) PullLayer(desc ocispec.Descriptor, destinationDir string) error {
	if desc.MediaType != ZarfLayerMediaTypeBlob {
		return fmt.Errorf("invalid media type for file layer: %s", desc.MediaType)
	}
	b, err := o.FetchLayer(desc)
	if err != nil {
		return err
	}
	return utils.WriteFile(filepath.Join(destinationDir, desc.Annotations[ocispec.AnnotationTitle]), b)
}

// PullPackagePaths pulls multiple files from the remote repository and saves them to `destinationDir`.
func (o *OrasRemote) PullPackagePaths(paths []string, destinationDir string) ([]ocispec.Descriptor, error) {
	paths = helpers.Unique(paths)
	root, err := o.FetchRoot()
	if err != nil {
		return nil, err
	}
	layersPulled := []ocispec.Descriptor{}
	for _, path := range paths {
		desc := root.Locate(path)
		if !IsEmptyDescriptor(desc) {
			layersPulled = append(layersPulled, desc)
			if o.FileDescriptorExists(desc, destinationDir) {
				continue
			}
			err = o.PullLayer(desc, destinationDir)
			if err != nil {
				return nil, err
			}
		}
	}
	return layersPulled, nil
}

// PullPackageMetadata pulls the package metadata from the remote repository and saves it to `destinationDir`.
func (o *OrasRemote) PullPackageMetadata(destinationDir string) ([]ocispec.Descriptor, error) {
	return o.PullPackagePaths(PackageAlwaysPull, destinationDir)
}
