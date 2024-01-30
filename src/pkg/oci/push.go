// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

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
//
// the contents of this file are not used by Zarf
type ConfigPartial struct {
	Architecture string            `json:"architecture"`
	OCIVersion   string            `json:"ociVersion"`
	Annotations  map[string]string `json:"annotations,omitempty"`
}

// PushLayer pushes the given layer (bytes) to the remote repository.
func (o *OrasRemote) PushLayer(b []byte, mediaType string) (ocispec.Descriptor, error) {
	desc := content.NewDescriptorFromBytes(mediaType, b)
	return desc, o.repo.Push(o.ctx, desc, bytes.NewReader(b))
}

// pushManifestConfigFromMetadata pushes the manifest config with metadata to the remote repository.
func (o *OrasRemote) pushManifestConfigFromMetadata(annotations map[string]string) (ocispec.Descriptor, error) {
	manifestConfig := ConfigPartial{
		Architecture: o.targetPlatform.Architecture,
		OCIVersion:   "1.0.1",
		// ?! Should this be all the users annotations or just title and description ?
		Annotations: annotations,
	}
	manifestConfigBytes, err := json.Marshal(manifestConfig)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	// If Media type is not set it will be set to the default
	// ?! Is this the image config media type or the
	return o.PushLayer(manifestConfigBytes, annotations[ocispec.MediaTypeImageConfig])
}

func (o *OrasRemote) generatePackManifest(src *file.Store, descs []ocispec.Descriptor, configDesc *ocispec.Descriptor, annotations map[string]string) (ocispec.Descriptor, error) {
	packOpts := oras.PackManifestOptions{
		Layers:              descs,
		ConfigDescriptor:    configDesc,
		ManifestAnnotations: annotations,
	}

	root, err := oras.PackManifest(o.ctx, src, oras.PackManifestVersion1_1_RC4, "", packOpts)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	if err = src.Tag(o.ctx, root, root.Digest.String()); err != nil {
		return ocispec.Descriptor{}, err
	}

	return root, nil
}

// PublishPackage publishes the package to the remote repository.
// TODO: We need documentation to Library that they need to use the ocispec package for map keys
// TODO: ?! We need the package.build architecutre rather than the o.targetPlatform.Arch. How do we make this clear to users?
func (o *OrasRemote) PublishPackage(ctx context.Context, src *file.Store, annotations map[string]string, arch string, desc []ocispec.Descriptor, concurrency int, progressBar ProgressWriter) (err error) {
	if annotations[ocispec.AnnotationTitle] == "" {
		return errors.New("invalid annotations: please include ocispec.AnnotationTitle")
	}
	copyOpts := o.CopyOpts
	copyOpts.Concurrency = concurrency
	// assumes referrers API is not supported since OCI artifact
	// media type is not supported
	o.repo.SetReferrersCapability(false)

	// push the manifest config
	// since this config is so tiny, and the content is not used again
	// it is not logged to the progress, but will error if it fails
	manifestConfigDesc, err := o.pushManifestConfigFromMetadata(annotations)
	if err != nil {
		return err
	}
	root, err := o.generatePackManifest(src, desc, &manifestConfigDesc, annotations)
	if err != nil {
		return err
	}

	// ?! Do we care about the size of these?
	// Maybe we should add a total function or simliar to set the byte amount in the progress bar
	// total += root.Size + manifestConfigDesc.Size

	o.Transport.ProgressBar = progressBar

	publishedDesc, err := oras.Copy(ctx, src, root.Digest.String(), o.repo, "", copyOpts)
	if err != nil {
		return err
	}

	if err := o.UpdateIndex(o.repo.Reference.Reference, arch, publishedDesc); err != nil {
		return err
	}

	return nil
}

// UpdateIndex updates the index for the given package.
func (o *OrasRemote) UpdateIndex(tag string, arch string, publishedDesc ocispec.Descriptor) error {
	var index ocispec.Index

	o.repo.Reference.Reference = tag
	// since ref has changed, need to reset root
	o.root = nil

	platform := &ocispec.Platform{
		OS:           MultiOS,
		Architecture: arch,
	}

	_, err := o.repo.Resolve(o.ctx, o.repo.Reference.Reference)
	if err != nil {
		if errors.Is(err, errdef.ErrNotFound) {
			index = ocispec.Index{
				Versioned: specs.Versioned{
					SchemaVersion: 2,
				},
				Manifests: []ocispec.Descriptor{
					{
						MediaType: ocispec.MediaTypeImageManifest,
						Digest:    publishedDesc.Digest,
						Size:      publishedDesc.Size,
						Platform:  platform,
					},
				},
			}
			return o.pushIndex(&index, tag)
		}
		return err
	}

	desc, rc, err := o.repo.FetchReference(o.ctx, tag)
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
		if m.Platform != nil && m.Platform.Architecture == arch {
			index.Manifests[idx].Digest = publishedDesc.Digest
			index.Manifests[idx].Size = publishedDesc.Size
			index.Manifests[idx].Platform = platform
			found = true
			break
		}
	}
	if !found {
		index.Manifests = append(index.Manifests, ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageManifest,
			Digest:    publishedDesc.Digest,
			Size:      publishedDesc.Size,
			Platform:  platform,
		})
	}

	return o.pushIndex(&index, tag)
}

func (o *OrasRemote) pushIndex(index *ocispec.Index, tag string) error {
	indexBytes, err := json.Marshal(index)
	if err != nil {
		return err
	}
	indexDesc := content.NewDescriptorFromBytes(ocispec.MediaTypeImageIndex, indexBytes)
	return o.repo.Manifests().PushReference(o.ctx, indexDesc, bytes.NewReader(indexBytes), tag)
}
