// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package archive provides read-only access to content stored in an OCI
// image layout directory.
package archive

import (
	"context"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"oras.land/oras-go/v2/content/oci"
)

// OCIStore represents a content store backed by an OCI image layout
// directory. It uses a root file path and an underlying OCI store source to
// manage content blobs.
type OCIStore struct {
	Root   string
	Source *oci.Store
}

// Info returns the size and digest of the content dgst identifies, or an
// error if the store cannot resolve it.
func (s *OCIStore) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	desc, err := s.Source.Resolve(ctx, dgst.String())
	if err != nil {
		return content.Info{}, err
	}
	return content.Info{
		Digest: desc.Digest,
		Size:   desc.Size,
	}, nil
}

// ReaderAt opens the blob desc identifies directly from the image layout
// directory and returns a reader over it. The caller owns the returned
// reader and must close it.
func (s *OCIStore) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	path := filepath.Join(s.Root, ocispec.ImageBlobsDir, desc.Digest.Algorithm().String(), desc.Digest.Encoded())
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		if cerr := f.Close(); cerr != nil {
			logger.From(ctx).Warn("failed to close blob reader", "path", path, "error", cerr)
		}
		return nil, err
	}
	return &fileReaderAt{File: f, size: fi.Size()}, nil
}

type fileReaderAt struct {
	*os.File
	size int64
}

func (r *fileReaderAt) Size() int64 {
	return r.size
}
