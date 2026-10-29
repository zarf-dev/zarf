// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import (
	"bytes"
	"context"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
)

// storeWithFiles pushes each path/body pair as a titled layer and returns the store and a manifest over them.
func storeWithFiles(ctx context.Context, t *testing.T, files map[string][]byte) (*memory.Store, *Manifest) {
	t.Helper()
	store := memory.New()
	manifest := &Manifest{}
	for path, body := range files {
		desc := content.NewDescriptorFromBytes("application/vnd.zarf.layer.v1.blob", body)
		desc.Annotations = map[string]string{ocispec.AnnotationTitle: path}
		require.NoError(t, store.Push(ctx, desc, bytes.NewReader(body)))
		manifest.Layers = append(manifest.Layers, desc)
	}
	return store, manifest
}

func TestFetchFile(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	want := []byte("kind: ZarfPackageConfig\n")
	store, manifest := storeWithFiles(ctx, t, map[string][]byte{"zarf.yaml": want})

	got, err := FetchFile(ctx, store, manifest, "zarf.yaml")
	require.NoError(t, err)
	require.Equal(t, want, got)

	_, err = FetchFile(ctx, store, manifest, "missing.yaml")
	require.EqualError(t, err, "unable to find missing.yaml in the manifest")
}

func TestFetchJSONFile(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	store, manifest := storeWithFiles(ctx, t, map[string][]byte{
		"images/index.json": []byte(`{"schemaVersion":2,"manifests":[{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:deadbeef","size":42}]}`),
		"truncated.json":    []byte(`{"schemaVersion":`),
	})

	// the pointer generic is the shape every in-tree caller uses
	index, err := FetchJSONFile[*ocispec.Index](ctx, store, manifest, "images/index.json")
	require.NoError(t, err)
	require.Equal(t, 2, index.SchemaVersion)
	require.Len(t, index.Manifests, 1)
	require.Equal(t, "sha256:deadbeef", index.Manifests[0].Digest.String())

	_, err = FetchJSONFile[*ocispec.Index](ctx, store, manifest, "missing.json")
	require.EqualError(t, err, "unable to find missing.json in the manifest")

	_, err = FetchJSONFile[*ocispec.Index](ctx, store, manifest, "truncated.json")
	require.ErrorContains(t, err, "unexpected end of JSON input")
}
