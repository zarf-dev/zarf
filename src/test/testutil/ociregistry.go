// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package testutil

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry/remote"
)

// NewRepo returns a plaintext-HTTP oras-go Repository suitable for pushing fixtures into an
// in-memory registry during tests.
func NewRepo(t *testing.T, refStr string) *remote.Repository {
	t.Helper()
	repo, err := remote.NewRepository(refStr)
	require.NoError(t, err)
	repo.PlainHTTP = true
	return repo
}

// RandomBytes returns n cryptographically random bytes; used as blob content that hashes
// differently on every test run.
func RandomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b
}

// PushBlob pushes raw bytes with the given media type and returns the resulting descriptor.
func PushBlob(ctx context.Context, t *testing.T, repo *remote.Repository, mediaType string, data []byte) ocispec.Descriptor {
	t.Helper()
	desc := ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    digest.FromBytes(data),
		Size:      int64(len(data)),
	}
	if exists, err := repo.Exists(ctx, desc); err == nil && exists {
		return desc
	}
	require.NoError(t, repo.Push(ctx, desc, bytes.NewReader(data)))
	return desc
}

// PushManifest constructs an image manifest pointing at the given config and layers, pushes it,
// and returns its descriptor.
func PushManifest(ctx context.Context, t *testing.T, repo *remote.Repository, config ocispec.Descriptor, layers []ocispec.Descriptor) ocispec.Descriptor {
	t.Helper()
	manifest := ocispec.Manifest{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    config,
		Layers:    layers,
	}
	body, err := json.Marshal(manifest)
	require.NoError(t, err)
	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.FromBytes(body),
		Size:      int64(len(body)),
	}
	require.NoError(t, repo.Push(ctx, desc, bytes.NewReader(body)))
	return desc
}

// PushIndex builds and pushes an OCI image index referencing the given child descriptors.
// Children may themselves be manifests or indexes; nested indexes are supported by the OCI spec.
func PushIndex(ctx context.Context, t *testing.T, repo *remote.Repository, children []ocispec.Descriptor) ocispec.Descriptor {
	t.Helper()
	idx := ocispec.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: children,
	}
	body, err := json.Marshal(idx)
	require.NoError(t, err)
	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageIndex,
		Digest:    digest.FromBytes(body),
		Size:      int64(len(body)),
	}
	require.NoError(t, repo.Push(ctx, desc, bytes.NewReader(body)))
	return desc
}

// PushSinglePlatformImage creates a config blob, a random layer, and a manifest that references
// both. The config embeds arch so distinct architectures produce distinct config blobs.
func PushSinglePlatformImage(ctx context.Context, t *testing.T, repo *remote.Repository, arch string) ocispec.Descriptor {
	t.Helper()
	layer := PushBlob(ctx, t, repo, ocispec.MediaTypeImageLayer, RandomBytes(t, 64))
	configJSON := fmt.Sprintf(`{"architecture":%q}`, arch)
	config := PushBlob(ctx, t, repo, ocispec.MediaTypeImageConfig, []byte(configJSON))
	return PushManifest(ctx, t, repo, config, []ocispec.Descriptor{layer})
}

// PushImage pushes a single-manifest image and tags it; returns the manifest digest.
func PushImage(ctx context.Context, t *testing.T, repoRef, tag string) string {
	t.Helper()
	repo := NewRepo(t, repoRef)
	desc := PushSinglePlatformImage(ctx, t, repo, "amd64")
	require.NoError(t, repo.Tag(ctx, desc, tag))
	return desc.Digest.String()
}

// PushMultiArchIndex pushes a flat multi-arch OCI image index with one single-platform manifest
// per entry in platforms. Returns the index digest.
func PushMultiArchIndex(ctx context.Context, t *testing.T, repoRef, tag string, platforms []ocispec.Platform) string {
	t.Helper()
	repo := NewRepo(t, repoRef)
	children := make([]ocispec.Descriptor, 0, len(platforms))
	for _, platform := range platforms {
		desc := PushSinglePlatformImage(ctx, t, repo, platform.Architecture)
		desc.Platform = &platform
		children = append(children, desc)
	}
	idx := PushIndex(ctx, t, repo, children)
	require.NoError(t, repo.Tag(ctx, idx, tag))
	return idx.Digest.String()
}

// PushNestedIndex pushes an OCI image index whose only child is itself an image index containing
// one single-platform manifest per entry in platforms. Returns the outer index digest.
func PushNestedIndex(ctx context.Context, t *testing.T, repoRef, tag string, platforms []ocispec.Platform) string {
	t.Helper()
	repo := NewRepo(t, repoRef)
	inner := make([]ocispec.Descriptor, 0, len(platforms))
	for _, platform := range platforms {
		desc := PushSinglePlatformImage(ctx, t, repo, platform.Architecture)
		desc.Platform = &platform
		inner = append(inner, desc)
	}
	innerIdx := PushIndex(ctx, t, repo, inner)
	outerIdx := PushIndex(ctx, t, repo, []ocispec.Descriptor{innerIdx})
	require.NoError(t, repo.Tag(ctx, outerIdx, tag))
	return outerIdx.Digest.String()
}
