// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/errdef"
)

func TestFetchCachePushError(t *testing.T) {
	t.Parallel()

	pushFailure := errors.New("cache write failed")
	for _, tt := range []struct {
		name        string
		pushErr     error
		consume     int64
		expectedErr error
	}{
		{name: "already exists", pushErr: errdef.ErrAlreadyExists},
		{name: "wrapped already exists", pushErr: fmt.Errorf("blob: %w", errdef.ErrAlreadyExists)},
		{name: "already exists after partial read", pushErr: errdef.ErrAlreadyExists, consume: 7},
		{name: "other error", pushErr: pushFailure, expectedErr: pushFailure},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			data := bytes.Repeat([]byte("source content"), 8192)
			descriptor := content.NewDescriptorFromBytes("application/octet-stream", data)
			source := memory.New()
			require.NoError(t, source.Push(ctx, descriptor, bytes.NewReader(data)))
			cache := pushStorage{
				Storage: memory.New(),
				push: func(_ context.Context, _ ocispec.Descriptor, reader io.Reader) error {
					if tt.consume > 0 {
						if _, err := io.CopyN(io.Discard, reader, tt.consume); err != nil {
							return err
						}
					}
					return tt.pushErr
				},
			}
			reader, err := New(source, cache).Fetch(ctx, descriptor)
			require.NoError(t, err)
			got, readErr := io.ReadAll(reader)
			closeErr := reader.Close()
			require.ErrorIs(t, readErr, tt.expectedErr)
			require.ErrorIs(t, closeErr, tt.expectedErr)
			if tt.expectedErr == nil {
				require.Equal(t, data, got)
			}
		})
	}
}

func TestFetchSharedCachePopulationRace(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	data := bytes.Repeat([]byte("shared layer content"), 8192)
	descriptor := content.NewDescriptorFromBytes("application/octet-stream", data)
	source := memory.New()
	require.NoError(t, source.Push(ctx, descriptor, bytes.NewReader(data)))
	cacheDir := filepath.Join(t.TempDir(), "images")
	firstCache, err := oci.New(cacheDir)
	require.NoError(t, err)
	secondCache, err := oci.New(cacheDir)
	require.NoError(t, err)

	// Hold the second push until the first reader has published the blob.
	// Both independent stores must miss the cache before either is populated.
	published := make(chan struct{})
	pushResult := make(chan error, 1)
	secondTarget := New(source, pushStorage{
		Storage: secondCache,
		push: func(ctx context.Context, descriptor ocispec.Descriptor, reader io.Reader) error {
			select {
			case <-published:
			case <-ctx.Done():
				return ctx.Err()
			}
			err := secondCache.Push(ctx, descriptor, reader)
			pushResult <- err
			return err
		},
	})
	firstReader, err := New(source, firstCache).Fetch(ctx, descriptor)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, firstReader.Close()) })
	secondReader, err := secondTarget.Fetch(ctx, descriptor)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, secondReader.Close()) })
	defer close(published)

	got, err := io.ReadAll(firstReader)
	closeErr := firstReader.Close()
	require.NoError(t, err)
	require.NoError(t, closeErr)
	require.Equal(t, data, got)
	published <- struct{}{}

	// Populate a separate package layout from the losing reader, as a pull does.
	packageLayout, err := oci.NewStorage(t.TempDir())
	require.NoError(t, err)
	err = packageLayout.Push(ctx, descriptor, secondReader)
	closeErr = secondReader.Close()
	require.NoError(t, err)
	require.NoError(t, closeErr)
	require.ErrorIs(t, <-pushResult, errdef.ErrAlreadyExists)

	for _, storage := range []content.Storage{firstCache, secondCache, packageLayout} {
		reader, err := storage.Fetch(ctx, descriptor)
		require.NoError(t, err)
		got, err := io.ReadAll(reader)
		closeErr := reader.Close()
		require.NoError(t, err)
		require.NoError(t, closeErr)
		require.Equal(t, data, got)
	}
}

type pushStorage struct {
	content.Storage
	push func(context.Context, ocispec.Descriptor, io.Reader) error
}

func (s pushStorage) Push(ctx context.Context, descriptor ocispec.Descriptor, reader io.Reader) error {
	return s.push(ctx, descriptor, reader)
}
