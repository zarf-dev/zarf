// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import (
	"bytes"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
)

func TestFetchFile(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	store := memory.New()
	want := []byte("kind: ZarfPackageConfig\n")
	desc := content.NewDescriptorFromBytes("application/vnd.zarf.layer.v1.blob", want)
	desc.Annotations = map[string]string{ocispec.AnnotationTitle: "zarf.yaml"}
	require.NoError(t, store.Push(ctx, desc, bytes.NewReader(want)))
	manifest := &Manifest{Manifest: ocispec.Manifest{Layers: []ocispec.Descriptor{desc}}}

	got, err := FetchFile(ctx, store, manifest, "zarf.yaml")
	require.NoError(t, err)
	require.Equal(t, want, got)

	_, err = FetchFile(ctx, store, manifest, "missing.yaml")
	require.EqualError(t, err, "unable to find missing.yaml in the manifest")
}
