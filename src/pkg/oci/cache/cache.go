// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cache creates cache-backed ORAS read targets.
package cache

import (
	"context"
	"errors"
	"io"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3"
	"github.com/oras-project/oras-go/v3/content"
	orascache "github.com/oras-project/oras-go/v3/content/cache"
	"github.com/oras-project/oras-go/v3/errdef"
)

// New wraps source with caching and tolerates concurrent population of storage.
func New(source oras.ReadOnlyTarget, storage content.Storage) oras.ReadOnlyTarget {
	return orascache.CacheReadOnlyTarget(source, duplicateTolerantStorage{storage})
}

type duplicateTolerantStorage struct {
	content.Storage
}

func (s duplicateTolerantStorage) Push(ctx context.Context, desc ocispec.Descriptor, reader io.Reader) error {
	err := s.Storage.Push(ctx, desc, reader)
	if errors.Is(err, errdef.ErrAlreadyExists) {
		// Another reader populated this blob. Consume the pipe so its producer
		// can keep streaming to the caller without blocking or losing bytes.
		_, err = io.Copy(io.Discard, reader)
	}
	return err
}
