// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
)

// UpdateIndex updates the index for the given package.
func (remote *OrasRemote) UpdateIndex(ctx context.Context, tag string, published ocispec.Descriptor) error {
	remote.repo.Reference.Reference = tag
	remote.root = nil
	_, err := remote.repo.Resolve(ctx, tag)
	if errors.Is(err, errdef.ErrNotFound) {
		index := ocispec.Index{
			MediaType: ocispec.MediaTypeImageIndex,
			Versioned: specs.Versioned{SchemaVersion: 2},
			Manifests: []ocispec.Descriptor{{
				MediaType: ocispec.MediaTypeImageManifest,
				Digest:    published.Digest,
				Size:      published.Size,
				Platform:  remote.targetPlatform,
			}},
		}
		return remote.pushIndex(ctx, &index, tag)
	}
	if err != nil {
		return err
	}
	descriptor, reader, err := remote.repo.FetchReference(ctx, tag)
	if err != nil {
		return err
	}
	defer reader.Close() //nolint:errcheck
	data, err := content.ReadAll(reader, descriptor)
	if err != nil {
		return err
	}
	var index ocispec.Index
	if err := json.Unmarshal(data, &index); err != nil {
		return err
	}
	for position := range index.Manifests {
		if index.Manifests[position].Platform != nil && index.Manifests[position].Platform.Architecture == remote.targetPlatform.Architecture {
			index.Manifests[position].Digest = published.Digest
			index.Manifests[position].Size = published.Size
			index.Manifests[position].Platform = remote.targetPlatform
			return remote.pushIndex(ctx, &index, tag)
		}
	}
	index.Manifests = append(index.Manifests, ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    published.Digest,
		Size:      published.Size,
		Platform:  remote.targetPlatform,
	})
	return remote.pushIndex(ctx, &index, tag)
}

func (remote *OrasRemote) pushIndex(ctx context.Context, index *ocispec.Index, tag string) error {
	data, err := json.Marshal(index)
	if err != nil {
		return err
	}
	descriptor := content.NewDescriptorFromBytes(ocispec.MediaTypeImageIndex, data)
	return remote.repo.Manifests().PushReference(ctx, descriptor, bytes.NewReader(data), tag)
}
