// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"

	goyaml "github.com/goccy/go-yaml"
)

// ResolveRoot returns the root descriptor for the remote repository
func (o *OrasRemote) ResolveRoot() (ocispec.Descriptor, error) {
	// first try to resolve the reference into an OCI descriptor directly
	desc, err := o.repo.Resolve(o.ctx, o.repo.Reference.Reference)
	// if we succeeded and it's not an index, return it
	// otherwise we will use oras.Resolve which will fetch the index, then resolve the manifest
	// w/ the target platform
	//
	// this error is purposefully ignored, as we want to try oras.Resolve if the first attempt fails
	if err == nil && desc.MediaType != ocispec.MediaTypeImageIndex {
		return desc, nil
	}

	if o.targetPlatform == nil && desc.MediaType == ocispec.MediaTypeImageIndex {
		return ocispec.Descriptor{}, fmt.Errorf("%q resolved to an image index, but no target platform was specified", o.repo.Reference.Reference)
	}

	resolveOpts := oras.ResolveOptions{
		TargetPlatform: o.targetPlatform,
	}
	// if the first attempt failed to resolve, or returned an index, try again with oras.Resolve
	return oras.Resolve(o.ctx, o.repo, o.repo.Reference.Reference, resolveOpts)
}

// FetchRoot fetches the root manifest from the remote repository.
func (o *OrasRemote) FetchRoot() (*OCIManifest, error) {
	if o.root != nil {
		return o.root, nil
	}
	// get the manifest descriptor
	descriptor, err := o.ResolveRoot()
	if err != nil {
		return nil, err
	}

	// fetch the manifest
	root, err := o.FetchManifest(descriptor)
	if err != nil {
		return nil, err
	}
	o.root = root
	return o.root, nil
}

// FetchManifest fetches the manifest with the given descriptor from the remote repository.
func (o *OrasRemote) FetchManifest(desc ocispec.Descriptor) (manifest *OCIManifest, err error) {
	return FetchUnmarshal[*OCIManifest](o.FetchLayer, json.Unmarshal, desc)
}

// FetchLayer fetches the layer with the given descriptor from the remote repository.
func (o *OrasRemote) FetchLayer(desc ocispec.Descriptor) (bytes []byte, err error) {
	return content.FetchAll(o.ctx, o.repo, desc)
}

// FetchZarfYAML fetches the zarf.yaml file from the remote repository.
func (o *OrasRemote) FetchZarfYAML() (pkg types.ZarfPackage, err error) {
	manifest, err := o.FetchRoot()
	if err != nil {
		return pkg, err
	}
	return FetchYAMLFile[types.ZarfPackage](o.FetchLayer, manifest, layout.ZarfYAML)
}

// FetchImagesIndex fetches the images/index.json file from the remote repository.
func (o *OrasRemote) FetchImagesIndex() (index *ocispec.Index, err error) {
	manifest, err := o.FetchRoot()
	if err != nil {
		return index, err
	}
	return FetchJSONFile[*ocispec.Index](o.FetchLayer, manifest, ZarfPackageIndexPath)
}

// FetchJSONFile fetches the given JSON file from the remote repository.
func FetchJSONFile[T any](fetcher func(desc ocispec.Descriptor) (bytes []byte, err error), manifest *OCIManifest, path string) (result T, err error) {
	descriptor := manifest.Locate(path)
	if IsEmptyDescriptor(descriptor) {
		return result, fmt.Errorf("unable to find %s in the manifest", path)
	}
	return FetchUnmarshal[T](fetcher, json.Unmarshal, descriptor)
}

// FetchYAMLFile fetches the given YAML file from the remote repository.
func FetchYAMLFile[T any](fetcher func(desc ocispec.Descriptor) (bytes []byte, err error), manifest *OCIManifest, path string) (result T, err error) {
	descriptor := manifest.Locate(path)
	if IsEmptyDescriptor(descriptor) {
		return result, fmt.Errorf("unable to find %s in the manifest", path)
	}
	return FetchUnmarshal[T](fetcher, goyaml.Unmarshal, descriptor)
}

// FetchUnmarshal fetches the given descriptor from the remote repository and unmarshals it.
func FetchUnmarshal[T any](fetcher func(desc ocispec.Descriptor) (bytes []byte, err error), unmarshaler func(data []byte, v interface{}) error, descriptor ocispec.Descriptor) (result T, err error) {
	bytes, err := fetcher(descriptor)
	if err != nil {
		return result, err
	}
	err = unmarshaler(bytes, &result)
	if err != nil {
		return result, err
	}
	return result, nil
}
