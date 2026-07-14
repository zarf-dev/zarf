// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024-Present Defense Unicorns

package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/errdef"
)

// ConfigPartial is a partial OCI config that is used to create the manifest config.
//
// Unless specified, an empty manifest config will be used: `{}`
// which causes an error on Google Artifact Registry
//
// to negate this, we create a simple manifest config with some build metadata
type ConfigPartial struct {
	Architecture string            `json:"architecture"`
	OCIVersion   string            `json:"ociVersion"`
	Annotations  map[string]string `json:"annotations,omitempty"`
}

// PushLayer pushes the given layer (bytes) to the remote repository.
func (o *OrasRemote) PushLayer(ctx context.Context, b []byte, mediaType string) (*ocispec.Descriptor, error) {
	desc := content.NewDescriptorFromBytes(mediaType, b)
	return &desc, o.repo.Push(ctx, desc, bytes.NewReader(b))
}

// CreateAndPushManifestConfig pushes the manifest config with metadata to the remote repository.
func (o *OrasRemote) CreateAndPushManifestConfig(ctx context.Context, annotations map[string]string, configMediaType string) (*ocispec.Descriptor, error) {
	if annotations[ocispec.AnnotationTitle] == "" {
		return nil, fmt.Errorf("invalid annotations: please include value for %q", ocispec.AnnotationTitle)
	}
	manifestConfig := ConfigPartial{
		Architecture: o.targetPlatform.Architecture,
		OCIVersion:   specs.Version,
		Annotations:  annotations,
	}
	manifestConfigBytes, err := json.Marshal(manifestConfig)
	if err != nil {
		return nil, err
	}
	// If media type is not set it will be set to the default
	return o.PushLayer(ctx, manifestConfigBytes, configMediaType)
}

// PackAndTagManifest generates an OCI Image Manifest based on the given parameters
// pushes that manifest to the remote repository and returns the manifest descriptor.
func (o *OrasRemote) PackAndTagManifest(ctx context.Context, src *file.Store, descs []ocispec.Descriptor,
	configDesc *ocispec.Descriptor, annotations map[string]string) (ocispec.Descriptor, error) {
	packOpts := oras.PackManifestOptions{
		Layers:              descs,
		ConfigDescriptor:    configDesc,
		ManifestAnnotations: annotations,
	}

	root, err := oras.PackManifest(ctx, src, oras.PackManifestVersion1_1, "", packOpts)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	if err = src.Tag(ctx, root, root.Digest.String()); err != nil {
		return ocispec.Descriptor{}, err
	}

	return root, nil
}

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
	defer rc.Close()

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
