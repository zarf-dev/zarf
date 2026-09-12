// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package archive

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"testing"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/oci"

	"github.com/zarf-dev/zarf/src/test/testutil"
)

// newTestStore creates an OCIStore backed by a fresh OCI store and
// pushes data as a single blob, returning the store and its descriptor.
func newTestStore(ctx context.Context, t *testing.T, data []byte) (*OCIStore, ocispec.Descriptor) {
	t.Helper()

	root := t.TempDir()
	src, err := oci.New(root)
	require.NoError(t, err)

	desc := content.NewDescriptorFromBytes(ocispec.MediaTypeImageLayer, data)
	require.NoError(t, src.Push(ctx, desc, bytes.NewReader(data)))

	return &OCIStore{Root: root, Source: src}, desc
}

func TestOCIStoreInfo(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	data := []byte("hello world")
	store, desc := newTestStore(ctx, t, data)

	testCases := []struct {
		name      string
		dgst      digest.Digest
		expectErr error
	}{
		{
			name: "existing blob",
			dgst: desc.Digest,
		},
		{
			name:      "missing digest",
			dgst:      digest.FromString("does-not-exist"),
			expectErr: errors.New("not found"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			info, err := store.Info(ctx, tc.dgst)
			if tc.expectErr != nil {
				require.ErrorContains(t, err, tc.expectErr.Error())
				return
			}
			require.NoError(t, err)
			require.Equal(t, desc.Digest, info.Digest)
			require.Equal(t, desc.Size, info.Size)
		})
	}
}

func TestOCIStoreReaderAt(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	data := []byte("the quick brown fox jumps over the lazy dog")
	store, desc := newTestStore(ctx, t, data)

	missing := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayer,
		Digest:    digest.FromString("missing"),
		Size:      4,
	}

	testCases := []struct {
		name      string
		desc      ocispec.Descriptor
		offset    int64
		want      []byte
		expectErr error
	}{
		{
			name: "read full blob from start",
			desc: desc,
			want: data,
		},
		{
			name:   "read partial blob at offset",
			desc:   desc,
			offset: 4,
			want:   data[4:8],
		},
		{
			name:      "missing blob",
			desc:      missing,
			expectErr: fs.ErrNotExist,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ra, err := store.ReaderAt(ctx, tc.desc)
			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, ra.Close()) })

			require.Equal(t, desc.Size, ra.Size())

			got := make([]byte, len(tc.want))
			n, err := ra.ReadAt(got, tc.offset)
			require.NoError(t, err)
			require.Equal(t, len(tc.want), n)
			require.Equal(t, tc.want, got)
		})
	}
}
