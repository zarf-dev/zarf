// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci provides Zarf-owned OCI registry operations.
package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
	registryremote "oras.land/oras-go/v2/registry/remote"
)

// UpdateIndexWithDescriptor updates tag's OCI index for platform and returns the
// exact immutable descriptor pushed by this invocation.
func UpdateIndexWithDescriptor(ctx context.Context, repo *registryremote.Repository, tag string, platform ocispec.Platform, manifest ocispec.Descriptor) (ocispec.Descriptor, error) {
	index, err := fetchIndex(ctx, repo, tag)
	if err != nil {
		if !errors.Is(err, errdef.ErrNotFound) {
			return ocispec.Descriptor{}, err
		}
		index = ocispec.Index{
			MediaType: ocispec.MediaTypeImageIndex,
			Versioned: specs.Versioned{SchemaVersion: 2},
		}
	}

	manifest.Platform = &platform
	updated := false
	for i, existing := range index.Manifests {
		if existing.Platform != nil && existing.Platform.Architecture == platform.Architecture {
			index.Manifests[i] = manifest
			updated = true
			break
		}
	}
	if !updated {
		index.Manifests = append(index.Manifests, manifest)
	}

	indexBytes, err := json.Marshal(index)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("marshal OCI index: %w", err)
	}
	indexDescriptor := content.NewDescriptorFromBytes(ocispec.MediaTypeImageIndex, indexBytes)
	if err := repo.Manifests().PushReference(ctx, indexDescriptor, bytes.NewReader(indexBytes), tag); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("push OCI index: %w", err)
	}

	return indexDescriptor, nil
}

func fetchIndex(ctx context.Context, repo *registryremote.Repository, tag string) (index ocispec.Index, err error) {
	descriptor, reader, err := repo.FetchReference(ctx, tag)
	if err != nil {
		return ocispec.Index{}, err
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close OCI index: %w", closeErr)
		}
	}()

	if descriptor.MediaType != ocispec.MediaTypeImageIndex {
		return ocispec.Index{}, fmt.Errorf("OCI reference %q is %s, not an image index", tag, descriptor.MediaType)
	}

	indexBytes, err := io.ReadAll(reader)
	if err != nil {
		return ocispec.Index{}, fmt.Errorf("read OCI index: %w", err)
	}
	if err := json.Unmarshal(indexBytes, &index); err != nil {
		return ocispec.Index{}, fmt.Errorf("unmarshal OCI index: %w", err)
	}
	return index, nil
}
