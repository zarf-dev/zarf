// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"

	goyaml "github.com/goccy/go-yaml"
)

// Repo gives you access to the underlying remote repository
func (o *OrasRemote) Repo() *remote.Repository {
	return o.repo
}

// ResolveRoot returns the root descriptor for the remote repository
func (o *OrasRemote) ResolveRoot() (ocispec.Descriptor, error) {
	return o.repo.Resolve(o.ctx, o.repo.Reference.Reference)
}

// FetchRoot fetches the root manifest from the remote repository.
func (o *OrasRemote) FetchRoot() (*ZarfOCIManifest, error) {
	if o.root != nil {
		return o.root, nil
	}
	// get the manifest descriptor
	descriptor, err := o.repo.Resolve(o.ctx, o.repo.Reference.Reference)
	if err != nil {
		return nil, err
	}

	// get the manifest itself
	bytes, err := o.FetchLayer(descriptor)
	if err != nil {
		return nil, err
	}
	manifest := ocispec.Manifest{}

	if err = json.Unmarshal(bytes, &manifest); err != nil {
		return nil, err
	}
	root := NewZarfOCIManifest(&manifest)
	o.root = root
	return o.root, nil
}

// FetchManifest fetches the manifest with the given descriptor from the remote repository.
func (o *OrasRemote) FetchManifest(desc ocispec.Descriptor) (manifest *ZarfOCIManifest, err error) {
	bytes, err := o.FetchLayer(desc)
	if err != nil {
		return manifest, err
	}
	err = json.Unmarshal(bytes, &manifest)
	if err != nil {
		return manifest, err
	}
	return manifest, nil
}

// FetchLayer fetches the layer with the given descriptor from the remote repository.
func (o *OrasRemote) FetchLayer(desc ocispec.Descriptor) (bytes []byte, err error) {
	return content.FetchAll(o.ctx, o.repo, desc)
}

// FetchZarfYAML fetches the zarf.yaml file from the remote repository.
func (o *OrasRemote) FetchZarfYAML(manifest *ZarfOCIManifest) (pkg types.ZarfPackage, err error) {
	return FetchYAML[types.ZarfPackage](o.FetchLayer, manifest, config.ZarfYAML)
}

// FetchImagesIndex fetches the images/index.json file from the remote repository.
func (o *OrasRemote) FetchImagesIndex(manifest *ZarfOCIManifest) (index *ocispec.Index, err error) {
	return FetchJSON[*ocispec.Index](o.FetchLayer, manifest, manifest.indexPath)
}

// FetchJSON fetches the given JSON file from the remote repository.
func FetchJSON[T any](fetcher func(desc ocispec.Descriptor) (bytes []byte, err error), manifest *ZarfOCIManifest, path string) (result T, err error) {
	return FetchUnmarshal[T](fetcher, json.Unmarshal, manifest, path)
}

// FetchYAML fetches the given JSON file from the remote repository.
func FetchYAML[T any](fetcher func(desc ocispec.Descriptor) (bytes []byte, err error), manifest *ZarfOCIManifest, path string) (result T, err error) {
	return FetchUnmarshal[T](fetcher, goyaml.Unmarshal, manifest, path)
}

// FetchUnmarshal fetches the given descriptor from the remote repository and unmarshals it.
func FetchUnmarshal[T any](fetcher func(desc ocispec.Descriptor) (bytes []byte, err error), unmarshaler func(data []byte, v interface{}) error, manifest *ZarfOCIManifest, path string) (result T, err error) {
	descriptor := manifest.Locate(path)
	if IsEmptyDescriptor(descriptor) {
		return result, fmt.Errorf("unable to find %s in the manifest", path)
	}
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
