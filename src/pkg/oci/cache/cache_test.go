// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/content"
	"github.com/oras-project/oras-go/v3/content/memory"
	"github.com/oras-project/oras-go/v3/content/oci"
	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/registry"
	"github.com/oras-project/oras-go/v3/registry/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		for _, fetchType := range []string{"digest", "reference"} {
			t.Run(tt.name+"/"+fetchType, func(t *testing.T) {
				t.Parallel()

				ctx := t.Context()
				data := bytes.Repeat([]byte("source content"), 8192)
				descriptor := content.NewDescriptorFromBytes("application/octet-stream", data)
				source := referenceStore{memory.New()}
				require.NoError(t, source.Push(ctx, descriptor, bytes.NewReader(data)))
				require.NoError(t, source.Tag(ctx, descriptor, "latest"))
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
				cached := New(source, cache)
				var reader io.ReadCloser
				var err error
				if fetchType == "reference" {
					fetcher, ok := cached.(registry.ReferenceFetcher)
					require.True(t, ok)
					_, reader, err = fetcher.FetchReference(ctx, "latest")
				} else {
					reader, err = cached.Fetch(ctx, descriptor)
				}
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
}

func TestFetchSharedCachePopulationRace(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	data := bytes.Repeat([]byte("shared layer content"), 8192)
	descriptor := content.NewDescriptorFromBytes("application/octet-stream", data)
	source := memory.New()
	require.NoError(t, source.Push(ctx, descriptor, bytes.NewReader(data)))
	cacheDir := filepath.Join(t.TempDir(), "images")
	firstCache, err := oci.NewStorage(cacheDir)
	require.NoError(t, err)
	secondCache, err := oci.NewStorage(cacheDir)
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

func TestFetchReferenceRefreshesTagAndUsesCachedContent(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	bodies := [][]byte{
		[]byte(`{"schemaVersion":2,"manifests":[]}`),
		[]byte(`{"schemaVersion":2,"manifests":[],"annotations":{"version":"2"}}`),
	}
	var current atomic.Int32
	var rejectBody atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := bodies[current.Load()]
		descriptor := content.NewDescriptorFromBytes(ocispec.MediaTypeImageIndex, body)
		if r.Method != http.MethodHead && rejectBody.Load() {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", descriptor.MediaType)
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.Header().Set("Docker-Content-Digest", descriptor.Digest.String())
		if r.Method != http.MethodHead {
			_, err := w.Write(body)
			assert.NoError(t, err)
		}
	}))
	t.Cleanup(server.Close)
	source, err := remote.NewRepository(strings.TrimPrefix(server.URL, "http://") + "/images")
	require.NoError(t, err)
	source.Registry.PlainHTTP = true
	storage, err := oci.NewStorage(t.TempDir())
	require.NoError(t, err)
	fetcher, ok := New(source, storage).(registry.ReferenceFetcher)
	require.True(t, ok)
	fetch := func() ocispec.Descriptor {
		t.Helper()
		descriptor, reader, err := fetcher.FetchReference(ctx, "latest")
		require.NoError(t, err)
		got, readErr := content.ReadAll(reader, descriptor)
		closeErr := reader.Close()
		require.NoError(t, readErr)
		require.NoError(t, closeErr)
		require.Equal(t, bodies[current.Load()], got)
		return descriptor
	}

	first := fetch()
	rejectBody.Store(true)
	require.Equal(t, first, fetch(), "cached content must remain readable when the registry refuses body downloads")

	current.Store(1)
	rejectBody.Store(false)
	second := fetch()
	require.NotEqual(t, first.Digest, second.Digest)
	rejectBody.Store(true)
	require.Equal(t, second, fetch())
	for i, descriptor := range []ocispec.Descriptor{first, second} {
		got, err := content.FetchAll(ctx, storage, descriptor)
		require.NoError(t, err)
		require.Equal(t, bodies[i], got)
	}
}

func TestFetchReferenceInterruptedReadDoesNotPopulateCache(t *testing.T) {
	t.Parallel()

	for _, stop := range []string{"close", "cancel"} {
		t.Run(stop, func(t *testing.T) {
			t.Parallel()

			data := []byte(`{"schemaVersion":2,"manifests":[]}`)
			descriptor := content.NewDescriptorFromBytes(ocispec.MediaTypeImageIndex, data)
			var complete atomic.Bool
			disconnected := make(chan struct{}, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", descriptor.MediaType)
				w.Header().Set("Content-Length", strconv.Itoa(len(data)))
				w.Header().Set("Docker-Content-Digest", descriptor.Digest.String())
				if r.Method == http.MethodHead {
					return
				}
				if complete.Load() {
					_, err := w.Write(data)
					assert.NoError(t, err)
					return
				}
				_, err := w.Write(data[:8])
				assert.NoError(t, err)
				assert.NoError(t, http.NewResponseController(w).Flush())
				<-r.Context().Done()
				disconnected <- struct{}{}
			}))
			t.Cleanup(server.Close)
			source, err := remote.NewRepository(strings.TrimPrefix(server.URL, "http://") + "/images")
			require.NoError(t, err)
			source.Registry.PlainHTTP = true
			storage, err := oci.NewStorage(t.TempDir())
			require.NoError(t, err)
			fetcher, ok := New(source, storage).(registry.ReferenceFetcher)
			require.True(t, ok)
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)
			_, reader, err := fetcher.FetchReference(ctx, "latest")
			require.NoError(t, err)
			t.Cleanup(func() { require.ErrorIs(t, reader.Close(), io.ErrUnexpectedEOF) })
			prefix := make([]byte, 8)
			_, err = io.ReadFull(reader, prefix)
			require.NoError(t, err)
			require.Equal(t, data[:8], prefix)
			if stop == "cancel" {
				cancel()
				_, err = io.ReadAll(reader)
				require.ErrorIs(t, err, context.Canceled)
			}
			require.ErrorIs(t, reader.Close(), io.ErrUnexpectedEOF)
			select {
			case <-disconnected:
			case <-time.After(5 * time.Second):
				t.Fatal("interrupted fetch did not close its source response")
			}
			exists, err := storage.Exists(t.Context(), descriptor)
			require.NoError(t, err)
			require.False(t, exists, "an interrupted read must not publish partial content to the cache")

			complete.Store(true)
			_, retried, err := fetcher.FetchReference(t.Context(), "latest")
			require.NoError(t, err)
			got, readErr := content.ReadAll(retried, descriptor)
			closeErr := retried.Close()
			require.NoError(t, readErr)
			require.NoError(t, closeErr)
			require.Equal(t, data, got)
			cached, err := content.FetchAll(t.Context(), storage, descriptor)
			require.NoError(t, err)
			require.Equal(t, data, cached)
		})
	}
}

type referenceStore struct {
	*memory.Store
}

func (s referenceStore) FetchReference(ctx context.Context, reference string) (ocispec.Descriptor, io.ReadCloser, error) {
	descriptor, err := s.Resolve(ctx, reference)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	reader, err := s.Fetch(ctx, descriptor)
	return descriptor, reader, err
}

type pushStorage struct {
	content.Storage
	push func(context.Context, ocispec.Descriptor, io.Reader) error
}

func (s pushStorage) Push(ctx context.Context, descriptor ocispec.Descriptor, reader io.Reader) error {
	return s.push(ctx, descriptor, reader)
}
