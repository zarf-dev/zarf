// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

// CachePath returns the root of the local OCI cache configured for this remote.
func (r *Remote) CachePath() (string, error) {
	if r.cachePath == "" {
		return "", fmt.Errorf("remote has no configured OCI cache")
	}
	return r.cachePath, nil
}

// CachedLayerPath fetches a layer through the remote's cache and returns its
// content-addressed path in the local OCI layout.
func (r *Remote) CachedLayerPath(ctx context.Context, descriptor ocispec.Descriptor) (string, error) {
	if _, err := r.CachePath(); err != nil {
		return "", err
	}
	cachedPath, err := cacheBlobPath(r.cachePath, descriptor)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(cachedPath); err == nil {
		return cachedPath, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	source, err := r.Fetch(ctx, descriptor)
	if err != nil {
		return "", err
	}
	reader := content.NewVerifyReader(source, descriptor)
	_, copyErr := io.Copy(io.Discard, reader)
	if copyErr == nil {
		copyErr = reader.Verify()
	}
	if closeErr := source.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return "", copyErr
	}
	if _, err := os.Stat(cachedPath); err != nil {
		return "", fmt.Errorf("cached OCI layer is unavailable: %w", err)
	}
	return cachedPath, nil
}

func cacheBlobPath(cachePath string, descriptor ocispec.Descriptor) (string, error) {
	if err := descriptor.Digest.Validate(); err != nil {
		return "", fmt.Errorf("invalid OCI descriptor digest: %w", err)
	}
	return filepath.Join(cachePath, ImageCacheDirectory, ocispec.ImageBlobsDir, descriptor.Digest.Algorithm().String(), descriptor.Digest.Encoded()), nil
}
