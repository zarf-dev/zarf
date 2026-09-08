// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cache creates cache-backed ORAS read targets.
package cache

import (
	"context"
	"io"
	"sync"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry"
)

type target struct {
	oras.ReadOnlyTarget
	cache content.Storage
}

// New generates a new target storage with caching.
func New(source oras.ReadOnlyTarget, cache content.Storage) oras.ReadOnlyTarget {
	t := &target{
		ReadOnlyTarget: source,
		cache:          cache,
	}
	if fetcher, ok := source.(registry.ReferenceFetcher); ok {
		return &referenceTarget{target: t, ReferenceFetcher: fetcher}
	}
	return t
}

// Fetch fetches the content identified by the descriptor.
func (target *target) Fetch(ctx context.Context, descriptor ocispec.Descriptor) (io.ReadCloser, error) {
	reader, err := target.cache.Fetch(ctx, descriptor)
	if err == nil {
		return reader, nil
	}
	reader, err = target.ReadOnlyTarget.Fetch(ctx, descriptor)
	if err != nil {
		return nil, err
	}
	return target.cacheReader(ctx, reader, descriptor), nil
}

func (target *target) cacheReader(ctx context.Context, reader io.ReadCloser, descriptor ocispec.Descriptor) io.ReadCloser {
	pipedReader, pipedWriter := io.Pipe()
	var waitGroup sync.WaitGroup
	var pushErr error
	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		pushErr = target.cache.Push(ctx, descriptor, pipedReader)
		if pushErr != nil {
			_ = pipedReader.CloseWithError(pushErr)
		}
	}()
	return &cacheReadCloser{
		Reader: io.TeeReader(reader, pipedWriter),
		close: func() error {
			readerErr := reader.Close()
			if err := pipedWriter.Close(); err != nil {
				return err
			}
			waitGroup.Wait()
			if pushErr != nil {
				return pushErr
			}
			return readerErr
		}}
}

type cacheReadCloser struct {
	io.Reader
	close func() error
}

func (reader *cacheReadCloser) Close() error { return reader.close() }

// Exists returns true if the described content exists.
func (target *target) Exists(ctx context.Context, descriptor ocispec.Descriptor) (bool, error) {
	exists, err := target.cache.Exists(ctx, descriptor)
	if err == nil && exists {
		return true, nil
	}
	return target.ReadOnlyTarget.Exists(ctx, descriptor)
}

type referenceTarget struct {
	*target
	registry.ReferenceFetcher
}

// FetchReference fetches the content identified by the reference from the
// remote and cache the fetched content.
// Cached content will only be read via Fetch, FetchReference will always fetch
// From origin.
func (target *referenceTarget) FetchReference(ctx context.Context, reference string) (ocispec.Descriptor, io.ReadCloser, error) {
	descriptor, reader, err := target.ReferenceFetcher.FetchReference(ctx, reference)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	exists, err := target.cache.Exists(ctx, descriptor)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	if !exists {
		return descriptor, target.cacheReader(ctx, reader, descriptor), nil
	}
	if err := reader.Close(); err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	reader, err = target.cache.Fetch(ctx, descriptor)
	return descriptor, reader, err
}
