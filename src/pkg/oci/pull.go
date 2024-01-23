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

	"slices"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
)

var (
	// ZarfPackageIndexPath is the path to the index.json file in the OCI package.
	ZarfPackageIndexPath = filepath.Join("images", "index.json")
	// ZarfPackageLayoutPath is the path to the oci-layout file in the OCI package.
	ZarfPackageLayoutPath = filepath.Join("images", "oci-layout")
	// ZarfPackageImagesBlobsDir is the path to the directory containing the image blobs in the OCI package.
	ZarfPackageImagesBlobsDir = filepath.Join("images", "blobs", "sha256")
)

// FileDescriptorExists returns true if the given file exists in the given directory with the expected SHA.
func (o *OrasRemote) FileDescriptorExists(desc ocispec.Descriptor, destinationDir string) bool {
	rel := desc.Annotations[ocispec.AnnotationTitle]
	destinationPath := filepath.Join(destinationDir, rel)

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

// PullPackage pulls the package from the remote repository and saves it to the given path.
//
// layersToPull is an optional parameter that allows the caller to specify which layers to pull.
//
// ?! Now that we are going to 100% going to call this function with parameters do we still want layerstopull to be optional?
func (o *OrasRemote) PullPackage(destinationDir string, concurrency int, layersToPull ...ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	// de-duplicate layers
	layersToPull = RemoveDuplicateDescriptors(layersToPull)

	dst, err := file.New(destinationDir)
	if err != nil {
		return nil, err
	}
	defer dst.Close()

	copyOpts := o.CopyOpts
	copyOpts.Concurrency = concurrency

	return layersToPull, o.CopyWithProgress(layersToPull, dst, copyOpts, destinationDir)
}

// CopyWithProgress copies the given layers from the remote repository to the given store.
func (o *OrasRemote) CopyWithProgress(layers []ocispec.Descriptor, store oras.Target, copyOpts oras.CopyOptions, destinationDir string) error {
	estimatedBytes := int64(0)
	shas := []string{}
	for _, layer := range layers {
		estimatedBytes += layer.Size
		if len(layer.Digest.String()) > 0 {
			shas = append(shas, layer.Digest.Encoded())
		}
	}

	if copyOpts.FindSuccessors == nil {
		copyOpts.FindSuccessors = func(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			nodes, err := content.Successors(ctx, fetcher, desc)
			if err != nil {
				return nil, err
			}
			if desc.MediaType == ocispec.MediaTypeImageIndex {
				manifestDescs := nodes
				nodes = []ocispec.Descriptor{}
				// expand the manifests
				for _, node := range manifestDescs {
					manifest, err := o.FetchManifest(node)
					if err != nil {
						return nil, err
					}
					nodes = append(nodes, manifest.Layers...)
					nodes = append(nodes, manifest.Config)
				}
			}

			var ret []ocispec.Descriptor
			for _, node := range nodes {
				if slices.Contains(shas, node.Digest.Encoded()) {
					ret = append(ret, node)
				}
			}
			return ret, nil
		}
	}

	// Create a thread to update a progress bar as we save the package to disk
	doneSaving := make(chan int)
	encounteredErr := make(chan int)
	var wg sync.WaitGroup
	wg.Add(1)
	successText := fmt.Sprintf("Pulling %q", helpers.OCIURLPrefix+o.repo.Reference.String())
	go utils.RenderProgressBarForLocalDirWrite(destinationDir, estimatedBytes, &wg, doneSaving, encounteredErr, "Pulling", successText)
	_, err := oras.Copy(o.ctx, o.repo, o.repo.Reference.String(), store, o.repo.Reference.String(), copyOpts)
	if err != nil {
		encounteredErr <- 1
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

	rel := desc.Annotations[ocispec.AnnotationTitle]

	return utils.WriteFile(filepath.Join(destinationDir, rel), b)
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
