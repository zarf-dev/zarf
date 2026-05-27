// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package images

import (
	"context"
	"encoding/json"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
)

func fetchAll(ctx context.Context, t *testing.T, fetcher content.Fetcher, desc ocispec.Descriptor) []byte {
	t.Helper()
	b, err := content.FetchAll(ctx, fetcher, desc)
	require.NoError(t, err)
	return b
}

// pushImageWithLayer pushes a single-platform image referencing the given layer, tags the manifest
// with linux/arch, and returns its descriptor plus the naive (non-deduplicated) total of every
// blob it references.
func pushImageWithLayer(ctx context.Context, t *testing.T, repo *remote.Repository, arch string, layer ocispec.Descriptor) (ocispec.Descriptor, int64) {
	t.Helper()
	desc := testutil.PushSinglePlatformImageWithLayer(ctx, t, repo, arch, layer)
	desc.Platform = &ocispec.Platform{OS: "linux", Architecture: arch}
	config := manifestConfig(ctx, t, repo, desc)
	return desc, desc.Size + config.Size + layer.Size
}

// manifestConfig fetches a manifest and returns its config descriptor.
func manifestConfig(ctx context.Context, t *testing.T, fetcher content.Fetcher, manDesc ocispec.Descriptor) ocispec.Descriptor {
	t.Helper()
	b := fetchAll(ctx, t, fetcher, manDesc)
	var m ocispec.Manifest
	require.NoError(t, json.Unmarshal(b, &m))
	return m.Config
}

func TestInspectIndex(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	upstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)

	t.Run("flat oci index sums every blob and reports each leaf platform", func(t *testing.T) {
		t.Parallel()
		repo := testutil.NewRepo(t, upstream+"/inspect/flat")
		layerA := testutil.PushBlob(ctx, t, repo, ocispec.MediaTypeImageLayer, testutil.RandomBytes(t, 128))
		layerB := testutil.PushBlob(ctx, t, repo, ocispec.MediaTypeImageLayer, testutil.RandomBytes(t, 256))
		manA, sizeA := pushImageWithLayer(ctx, t, repo, "amd64", layerA)
		manB, sizeB := pushImageWithLayer(ctx, t, repo, "arm64", layerB)
		idx := testutil.PushIndex(ctx, t, repo, []ocispec.Descriptor{manA, manB})

		size, platforms, err := inspectIndex(ctx, repo, idx, fetchAll(ctx, t, repo, idx))
		require.NoError(t, err)
		require.Equal(t, idx.Size+sizeA+sizeB, size)
		require.ElementsMatch(t, []string{"amd64", "arm64"}, platforms)
	})

	t.Run("nested oci index recurses into inner index", func(t *testing.T) {
		t.Parallel()
		repo := testutil.NewRepo(t, upstream+"/inspect/nested")
		layerA := testutil.PushBlob(ctx, t, repo, ocispec.MediaTypeImageLayer, testutil.RandomBytes(t, 128))
		layerB := testutil.PushBlob(ctx, t, repo, ocispec.MediaTypeImageLayer, testutil.RandomBytes(t, 256))
		manA, sizeA := pushImageWithLayer(ctx, t, repo, "amd64", layerA)
		manB, sizeB := pushImageWithLayer(ctx, t, repo, "arm64", layerB)
		innerIdx := testutil.PushIndex(ctx, t, repo, []ocispec.Descriptor{manA, manB})
		outerIdx := testutil.PushIndex(ctx, t, repo, []ocispec.Descriptor{innerIdx})

		size, platforms, err := inspectIndex(ctx, repo, outerIdx, fetchAll(ctx, t, repo, outerIdx))
		require.NoError(t, err)
		require.Equal(t, outerIdx.Size+innerIdx.Size+sizeA+sizeB, size)
		require.ElementsMatch(t, []string{"amd64", "arm64"}, platforms)
	})

	t.Run("docker manifest list is treated as an index", func(t *testing.T) {
		t.Parallel()
		repo := testutil.NewRepo(t, upstream+"/inspect/docker")
		layerA := testutil.PushBlob(ctx, t, repo, ocispec.MediaTypeImageLayer, testutil.RandomBytes(t, 128))
		manA, sizeA := pushImageWithLayer(ctx, t, repo, "amd64", layerA)
		list := pushDockerManifestList(ctx, t, repo, []ocispec.Descriptor{manA})

		size, platforms, err := inspectIndex(ctx, repo, list, fetchAll(ctx, t, repo, list))
		require.NoError(t, err)
		require.Equal(t, list.Size+sizeA, size)
		require.ElementsMatch(t, []string{"amd64"}, platforms)
	})

	t.Run("dedups a layer referenced by sibling manifests", func(t *testing.T) {
		t.Parallel()
		repo := testutil.NewRepo(t, upstream+"/inspect/dedup-layer")
		shared := testutil.PushBlob(ctx, t, repo, ocispec.MediaTypeImageLayer, testutil.RandomBytes(t, 512))
		manA, _ := pushImageWithLayer(ctx, t, repo, "amd64", shared)
		manB, _ := pushImageWithLayer(ctx, t, repo, "arm64", shared)
		idx := testutil.PushIndex(ctx, t, repo, []ocispec.Descriptor{manA, manB})

		size, _, err := inspectIndex(ctx, repo, idx, fetchAll(ctx, t, repo, idx))
		require.NoError(t, err)

		configA := manifestConfig(ctx, t, repo, manA)
		configB := manifestConfig(ctx, t, repo, manB)
		expected := idx.Size + manA.Size + configA.Size + shared.Size + manB.Size + configB.Size
		require.Equal(t, expected, size)
	})

	t.Run("dedups a manifest listed twice in an index", func(t *testing.T) {
		t.Parallel()
		repo := testutil.NewRepo(t, upstream+"/inspect/dup-manifest")
		layer := testutil.PushBlob(ctx, t, repo, ocispec.MediaTypeImageLayer, testutil.RandomBytes(t, 256))
		man, manSize := pushImageWithLayer(ctx, t, repo, "amd64", layer)
		idx := testutil.PushIndex(ctx, t, repo, []ocispec.Descriptor{man, man})

		size, platforms, err := inspectIndex(ctx, repo, idx, fetchAll(ctx, t, repo, idx))
		require.NoError(t, err)
		require.Equal(t, idx.Size+manSize, size)
		require.ElementsMatch(t, []string{"amd64"}, platforms)
	})
}
