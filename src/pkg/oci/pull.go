// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"context"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
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

// PullLayers pulls the package from the remote repository and saves it to the given path.
func (o *OrasRemote) PullLayers(ctx context.Context, destinationDir string, concurrency int,
	layersToPull []ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	// de-duplicate layers
	layersToPull = RemoveDuplicateDescriptors(layersToPull)

	dst, err := file.New(destinationDir)
	if err != nil {
		return nil, err
	}
	defer dst.Close()

	copyOpts := o.CopyOpts
	copyOpts.Concurrency = concurrency

	return layersToPull, o.CopyToStore(ctx, layersToPull, dst, copyOpts)
}

// CopyToStore copies the given layers from the remote repository to the given store.
func (o *OrasRemote) CopyToStore(ctx context.Context, layers []ocispec.Descriptor, store oras.Target, copyOpts oras.CopyOptions) error {
	shas := []string{}
	for _, layer := range layers {
		if len(layer.Digest.String()) > 0 {
			shas = append(shas, layer.Digest.Encoded())
		}
	}

	preCopy := copyOpts.PreCopy
	copyOpts.PreCopy = func(ctx context.Context, desc ocispec.Descriptor) error {
		if preCopy != nil {
			if err := preCopy(ctx, desc); err != nil {
				return err
			}
		}
		for _, sha := range shas {
			if sha == desc.Digest.Encoded() {
				return nil
			}
		}
		return oras.SkipNode
	}

	_, err := oras.Copy(ctx, o.repo, o.repo.Reference.String(), store, o.repo.Reference.String(), copyOpts)
	if err != nil {
		return err
	}

	return nil
}

// PullLayer pulls a layer from the remote repository and saves it to `destinationDir/annotationTitle`.
func (o *OrasRemote) PullLayer(ctx context.Context, destinationDir string, desc ocispec.Descriptor) error {
	b, err := o.FetchLayer(ctx, desc)
	if err != nil {
		return err
	}

	rel := desc.Annotations[ocispec.AnnotationTitle]

	return os.WriteFile(filepath.Join(destinationDir, rel), b, 0644)
}

// PullPaths pulls multiple files from the remote repository and saves them to `destinationDir`.
func (o *OrasRemote) PullPaths(ctx context.Context, destinationDir string, paths []string) ([]ocispec.Descriptor, error) {
	paths = helpers.Unique(paths)
	root, err := o.FetchRoot(ctx)
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
			err = o.PullLayer(ctx, destinationDir, desc)
			if err != nil {
				return nil, err
			}
		}
	}
	return layersPulled, nil
}
