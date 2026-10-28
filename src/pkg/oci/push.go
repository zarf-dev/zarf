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
func (o *OrasRemote) UpdateIndex(ctx context.Context, tag string, publishedDesc ocispec.Descriptor) error {
	var index ocispec.Index

	o.repo.Reference.Reference = tag
	// since ref has changed, need to reset root
	o.root = nil

	_, err := o.repo.Resolve(ctx, o.repo.Reference.Reference)
	if err != nil {
		if errors.Is(err, errdef.ErrNotFound) {
			index = ocispec.Index{
				MediaType: ocispec.MediaTypeImageIndex,
				Versioned: specs.Versioned{
					SchemaVersion: 2,
				},
				Manifests: []ocispec.Descriptor{
					{
						MediaType: ocispec.MediaTypeImageManifest,
						Digest:    publishedDesc.Digest,
						Size:      publishedDesc.Size,
						Platform:  o.targetPlatform,
					},
				},
			}
			return o.pushIndex(ctx, &index, tag)
		}
		return err
	}

	desc, rc, err := o.repo.FetchReference(ctx, tag)
	if err != nil {
		return err
	}
	defer rc.Close() //nolint:errcheck

	b, err := content.ReadAll(rc, desc)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(b, &index); err != nil {
		return err
	}

	found := false
	for idx, m := range index.Manifests {
		if m.Platform != nil && m.Platform.Architecture == o.targetPlatform.Architecture {
			index.Manifests[idx].Digest = publishedDesc.Digest
			index.Manifests[idx].Size = publishedDesc.Size
			index.Manifests[idx].Platform = o.targetPlatform
			found = true
			break
		}
	}
	if !found {
		index.Manifests = append(index.Manifests, ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageManifest,
			Digest:    publishedDesc.Digest,
			Size:      publishedDesc.Size,
			Platform:  o.targetPlatform,
		})
	}

	return o.pushIndex(ctx, &index, tag)
}

func (o *OrasRemote) pushIndex(ctx context.Context, index *ocispec.Index, tag string) error {
	indexBytes, err := json.Marshal(index)
	if err != nil {
		return err
	}
	indexDesc := content.NewDescriptorFromBytes(ocispec.MediaTypeImageIndex, indexBytes)
	return o.repo.Manifests().PushReference(ctx, indexDesc, bytes.NewReader(indexBytes), tag)
}
