// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/pkg/oci/cache"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
)

// ResolveRoot returns the root descriptor for the remote repository
func (remote *OrasRemote) ResolveRoot(ctx context.Context) (ocispec.Descriptor, error) {
	descriptor, err := remote.repo.Resolve(ctx, remote.repo.Reference.Reference)
	if err == nil && descriptor.MediaType != ocispec.MediaTypeImageIndex {
		return descriptor, nil
	}
	if remote.targetPlatform == nil && descriptor.MediaType == ocispec.MediaTypeImageIndex {
		return ocispec.Descriptor{}, fmt.Errorf("%q resolved to an image index, but no target platform was specified", remote.repo.Reference.Reference)
	}
	return oras.Resolve(ctx, remote.repo, remote.repo.Reference.Reference, oras.ResolveOptions{TargetPlatform: remote.targetPlatform})
}

// FetchRoot fetches the root manifest from the remote repository.
func (remote *OrasRemote) FetchRoot(ctx context.Context) (*Manifest, error) {
	if remote.root != nil {
		return remote.root, nil
	}
	descriptor, err := remote.ResolveRoot(ctx)
	if err != nil {
		return nil, err
	}
	remote.root, err = remote.FetchManifest(ctx, descriptor)
	return remote.root, err
}

// FetchManifest fetches the manifest with the given descriptor from the remote repository.
func (remote *OrasRemote) FetchManifest(ctx context.Context, descriptor ocispec.Descriptor) (*Manifest, error) {
	return FetchUnmarshal[*Manifest](ctx, remote, json.Unmarshal, descriptor)
}

// source returns the read target for layer fetches, wrapping the repository with the
// layer cache when one is configured.
func (remote *OrasRemote) source() oras.ReadOnlyTarget {
	if remote.cache != nil {
		return cache.New(remote.repo, remote.cache)
	}
	return remote.repo
}

// Fetch fetches the content for the given descriptor, honoring the layer cache
// when configured. This satisfies oras content.Fetcher, so an OrasRemote can be
// passed directly to oras helpers such as content.FetchAll and FetchJSONFile.
func (remote *OrasRemote) Fetch(ctx context.Context, descriptor ocispec.Descriptor) (io.ReadCloser, error) {
	return remote.source().Fetch(ctx, descriptor)
}

// FetchLayer fetches (and digest-verifies) the layer with the given descriptor.
func (remote *OrasRemote) FetchLayer(ctx context.Context, descriptor ocispec.Descriptor) ([]byte, error) {
	return content.FetchAll(ctx, remote, descriptor)
}

// FetchLayerReader fetches the layer with the given descriptor from the remote repository.
func (remote *OrasRemote) FetchLayerReader(ctx context.Context, descriptor ocispec.Descriptor) (*content.VerifyReader, error) {
	reader, err := remote.Fetch(ctx, descriptor)
	if err != nil {
		return nil, err
	}
	return content.NewVerifyReader(reader, descriptor), nil
}

// FetchJSONFile fetches and unmarshals the JSON file at path, located via the manifest.
func FetchJSONFile[T any](ctx context.Context, fetcher content.Fetcher, manifest *Manifest, path string) (result T, err error) {
	descriptor := manifest.Locate(path)
	if IsEmptyDescriptor(descriptor) {
		return result, fmt.Errorf("unable to find %s in the manifest", path)
	}
	return FetchUnmarshal[T](ctx, fetcher, json.Unmarshal, descriptor)
}

// FetchUnmarshal fetches (and digest-verifies, via content.FetchAll) the descriptor and unmarshals it.
func FetchUnmarshal[T any](ctx context.Context, fetcher content.Fetcher, unmarshaler func([]byte, any) error, descriptor ocispec.Descriptor) (result T, err error) {
	data, err := content.FetchAll(ctx, fetcher, descriptor)
	if err != nil {
		return result, err
	}
	if err := unmarshaler(data, &result); err != nil {
		return result, err
	}
	return result, nil
}
