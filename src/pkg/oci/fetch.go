// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with artifacts stored in OCI registries.
package oci

import (
	"context"
	"encoding/json"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"

	goyaml "github.com/goccy/go-yaml"
)

// ResolveRoot returns the root descriptor for the remote repository
func (o *OrasRemote) ResolveRoot(ctx context.Context) (ocispec.Descriptor, error) {
	// first try to resolve the reference into an OCI descriptor directly
	desc, err := o.repo.Resolve(ctx, o.repo.Reference.Reference)
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
	return oras.Resolve(ctx, o.repo, o.repo.Reference.Reference, resolveOpts)
}

// FetchRoot fetches the root manifest from the remote repository.
func (o *OrasRemote) FetchRoot(ctx context.Context) (*Manifest, error) {
	if o.root != nil {
		return o.root, nil
	}
	// get the manifest descriptor
	descriptor, err := o.ResolveRoot(ctx)
	if err != nil {
		return nil, err
	}

	// fetch the manifest
	root, err := o.FetchManifest(ctx, descriptor)
	if err != nil {
		return nil, err
	}
	o.root = root
	return o.root, nil
}

// FetchManifest fetches the manifest with the given descriptor from the remote repository.
func (o *OrasRemote) FetchManifest(ctx context.Context, desc ocispec.Descriptor) (manifest *Manifest, err error) {
	return FetchUnmarshal[*Manifest](ctx, o.FetchLayer, json.Unmarshal, desc)
}

// FetchLayer fetches the layer with the given descriptor from the remote repository.
func (o *OrasRemote) FetchLayer(ctx context.Context, desc ocispec.Descriptor) (bytes []byte, err error) {
	return content.FetchAll(ctx, o.repo, desc)
}

// FetchJSONFile fetches the given JSON file from the remote repository.
func FetchJSONFile[T any](ctx context.Context, fetcher func(ctx context.Context, desc ocispec.Descriptor) (bytes []byte, err error), manifest *Manifest, path string) (result T, err error) {
	descriptor := manifest.Locate(path)
	if IsEmptyDescriptor(descriptor) {
		return result, fmt.Errorf("unable to find %s in the manifest", path)
	}
	return FetchUnmarshal[T](ctx, fetcher, json.Unmarshal, descriptor)
}

// FetchYAMLFile fetches the given YAML file from the remote repository.
func FetchYAMLFile[T any](ctx context.Context, fetcher func(ctx context.Context, desc ocispec.Descriptor) (bytes []byte, err error), manifest *Manifest, path string) (result T, err error) {
	descriptor := manifest.Locate(path)
	if IsEmptyDescriptor(descriptor) {
		return result, fmt.Errorf("unable to find %s in the manifest", path)
	}
	return FetchUnmarshal[T](ctx, fetcher, goyaml.Unmarshal, descriptor)
}

// FetchUnmarshal fetches the given descriptor from the remote repository and unmarshals it.
func FetchUnmarshal[T any](ctx context.Context, fetcher func(ctx context.Context, desc ocispec.Descriptor) (bytes []byte, err error), unmarshaler func(data []byte, v interface{}) error, descriptor ocispec.Descriptor) (result T, err error) {
	bytes, err := fetcher(ctx, descriptor)
	if err != nil {
		return result, err
	}
	err = unmarshaler(bytes, &result)
	if err != nil {
		return result, err
	}
	return result, nil
}
