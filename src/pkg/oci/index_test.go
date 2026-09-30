// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry"
	remote "oras.land/oras-go/v2/registry/remote"
)

func TestUpdateIndexWithDescriptor(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	ref := registry.Reference{
		Registry:   testutil.SetupInMemoryRegistryDynamic(ctx, t),
		Repository: "components",
		Reference:  "example",
	}
	remote, err := zoci.NewRemoteWithOptions(ctx, ref.String(), ocispec.Platform{}, zoci.RemoteClientOptions{
		RemoteOptions: types.RemoteOptions{PlainHTTP: true},
	})
	require.NoError(t, err)

	amd64First := pushManifest(ctx, t, remote, "amd64-first")
	indexDescriptor, err := UpdateIndexWithDescriptor(ctx, remote.Repo(), ref.Reference, ocispec.Platform{Architecture: "amd64"}, amd64First)
	require.NoError(t, err)
	require.Equal(t, ocispec.MediaTypeImageIndex, indexDescriptor.MediaType)
	_, err = remote.Repo().Resolve(ctx, ref.Reference)
	require.ErrorContains(t, err, "not found")
	tagDescriptor(ctx, t, remote, indexDescriptor, ref.Reference)
	require.Equal(t, indexDescriptor, resolveDescriptor(ctx, t, remote, ref.Reference))
	requireIndexManifests(t, readIndex(ctx, t, remote, ref.Reference), map[string]ocispec.Descriptor{
		"amd64": amd64First,
	})

	arm64 := pushManifest(ctx, t, remote, "arm64")
	previousIndex := indexDescriptor
	indexDescriptor, err = UpdateIndexWithDescriptor(ctx, remote.Repo(), ref.Reference, ocispec.Platform{Architecture: "arm64"}, arm64)
	require.NoError(t, err)
	require.Equal(t, previousIndex, resolveDescriptor(ctx, t, remote, ref.Reference))
	tagDescriptor(ctx, t, remote, indexDescriptor, ref.Reference)
	require.Equal(t, indexDescriptor, resolveDescriptor(ctx, t, remote, ref.Reference))
	requireIndexManifests(t, readIndex(ctx, t, remote, ref.Reference), map[string]ocispec.Descriptor{
		"amd64": amd64First,
		"arm64": arm64,
	})

	amd64Replacement := pushManifest(ctx, t, remote, "amd64-replacement")
	previousIndex = indexDescriptor
	indexDescriptor, err = UpdateIndexWithDescriptor(ctx, remote.Repo(), ref.Reference, ocispec.Platform{Architecture: "amd64"}, amd64Replacement)
	require.NoError(t, err)
	require.Equal(t, previousIndex, resolveDescriptor(ctx, t, remote, ref.Reference))
	tagDescriptor(ctx, t, remote, indexDescriptor, ref.Reference)
	require.Equal(t, indexDescriptor, resolveDescriptor(ctx, t, remote, ref.Reference))
	requireIndexManifests(t, readIndex(ctx, t, remote, ref.Reference), map[string]ocispec.Descriptor{
		"amd64": amd64Replacement,
		"arm64": arm64,
	})
}

func TestFetchIndexRejectsMismatchedDescriptor(t *testing.T) {
	t.Parallel()

	expected := []byte(`{"schemaVersion":2,"manifests":[]}`)
	received := []byte(`{"schemaVersion":3,"manifests":[]}`)
	descriptor := content.NewDescriptorFromBytes(ocispec.MediaTypeImageIndex, expected)
	repo := &remote.Repository{
		Client: staticResponseClient{
			body:   received,
			digest: descriptor.Digest.String(),
		},
		Reference: registry.Reference{
			Registry:   "example.com",
			Repository: "components",
		},
		PlainHTTP: true,
	}

	_, err := fetchIndex(context.Background(), repo, "example")
	require.ErrorIs(t, err, content.ErrMismatchedDigest)
}

type staticResponseClient struct {
	body   []byte
	digest string
}

func (c staticResponseClient) Do(request *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode:    http.StatusOK,
		ContentLength: int64(len(c.body)),
		Header: http.Header{
			"Content-Type":          []string{ocispec.MediaTypeImageIndex},
			"Docker-Content-Digest": []string{c.digest},
		},
		Body:    io.NopCloser(bytes.NewReader(c.body)),
		Request: request,
	}, nil
}

func pushManifest(ctx context.Context, t *testing.T, remote *zoci.Remote, contents string) ocispec.Descriptor {
	t.Helper()

	store := memory.New()
	config := content.NewDescriptorFromBytes("application/vnd.zarf.component.test.config.v1+json", []byte(contents))
	require.NoError(t, store.Push(ctx, config, bytes.NewReader([]byte(contents))))
	manifest, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, "", oras.PackManifestOptions{
		ConfigDescriptor: &config,
	})
	require.NoError(t, err)
	require.NoError(t, store.Tag(ctx, manifest, manifest.Digest.String()))
	_, err = oras.Copy(ctx, store, manifest.Digest.String(), remote.Repo(), "", remote.GetDefaultCopyOpts())
	require.NoError(t, err)
	return manifest
}
func tagDescriptor(ctx context.Context, t *testing.T, remote *zoci.Remote, descriptor ocispec.Descriptor, tag string) {
	t.Helper()
	require.NoError(t, remote.Repo().Tag(ctx, descriptor, tag))
}

func resolveDescriptor(ctx context.Context, t *testing.T, remote *zoci.Remote, tag string) ocispec.Descriptor {
	t.Helper()

	descriptor, err := remote.Repo().Resolve(ctx, tag)
	require.NoError(t, err)
	return descriptor
}

func readIndex(ctx context.Context, t *testing.T, remote *zoci.Remote, tag string) ocispec.Index {
	t.Helper()

	descriptor, reader, err := remote.Repo().FetchReference(ctx, tag)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reader.Close()) })
	require.Equal(t, ocispec.MediaTypeImageIndex, descriptor.MediaType)
	indexBytes, err := io.ReadAll(reader)
	require.NoError(t, err)

	var index ocispec.Index
	require.NoError(t, json.Unmarshal(indexBytes, &index))
	return index
}

func requireIndexManifests(t *testing.T, index ocispec.Index, expected map[string]ocispec.Descriptor) {
	t.Helper()

	actual := make(map[string]ocispec.Descriptor, len(index.Manifests))
	for _, manifest := range index.Manifests {
		require.NotNil(t, manifest.Platform)
		actual[manifest.Platform.Architecture] = manifest
	}
	require.Len(t, actual, len(expected))
	for architecture, descriptor := range expected {
		actualDescriptor, found := actual[architecture]
		require.Truef(t, found, "missing platform %q", architecture)
		require.Equal(t, descriptor.Digest, actualDescriptor.Digest)
		require.Equal(t, descriptor.Size, actualDescriptor.Size)
	}
}
