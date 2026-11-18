/*
Copyright The ORAS Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/content"
	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/internal/ioutil"
	"github.com/oras-project/oras-go/v3/registry/remote/auth"
)

// defaultUnsafePushChunkSize is used when chunked upload is not configured.
// Each chunk is buffered in memory.
const defaultUnsafePushChunkSize int64 = 8 << 20

// UnsafePush pushes a blob whose digest or size may be unknown, subject to
// [Repository.MaxUnsafePushBytes]. See [content.UnsafePusher] for verification
// and error semantics. Manifests are not supported here. Content is verified
// before anything is committed. A session opened by UnsafePush is canceled on
// failure; a monolithic push of already-verified small content behaves like Push.
// When the digest or size is unknown, existing content is pushed again and the
// registry deduplicates it, so ErrAlreadyExists is only returned by the HEAD check.
//
// Content no larger than the effective MaxChunkSize (8 MiB if chunking is not
// configured) is buffered in memory; larger content is pushed in chunks, so
// up to about twice the chunk size is held in memory. Once content has been
// read there is no monolithic fallback: the push fails if the registry does not
// support chunked uploads or advertises an OCI-Chunk-Min-Length above 32 MiB.
func (s *blobStore) UnsafePush(ctx context.Context, expected ocispec.Descriptor, r io.Reader) (ocispec.Descriptor, error) {
	if err := s.repo.checkPolicy(ctx, ""); err != nil {
		return ocispec.Descriptor{}, err
	}
	if err := ctx.Err(); err != nil {
		return ocispec.Descriptor{}, err
	}
	limit := s.repo.maxUnsafePushBytes()
	if err := ioutil.ValidateUnsafePush(expected, limit); err != nil {
		return ocispec.Descriptor{}, err
	}
	if err := checkDigestAlgorithm(digestSourceDescriptor, expected.Digest); err != nil {
		return ocispec.Descriptor{}, err
	}
	ctx = auth.AppendRepositoryScope(ctx, s.repo.reference(), auth.ActionPull, auth.ActionPush)
	if expected.Digest != "" && expected.Size >= 0 {
		desc, err := s.resolve(ctx, expected.Digest.String())
		if err != nil && !errors.Is(err, errdef.ErrNotFound) {
			return ocispec.Descriptor{}, err
		}
		if err == nil && desc.Size == expected.Size {
			return expected, fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, errdef.ErrAlreadyExists)
		}
	}

	chunk := s.repo.maxChunkSize()
	if chunk <= 0 {
		chunk = defaultUnsafePushChunkSize
	}
	bound := limit
	if expected.Size >= 0 {
		bound = expected.Size
	}
	peek := min(chunk, bound)
	headLimit := peek
	if headLimit < math.MaxInt64 {
		headLimit++
	}
	source := &unsafePushReader{r: ioutil.NewContextReader(ctx, r), expected: expected}
	head, err := io.ReadAll(io.LimitReader(source, headLimit))
	if err != nil {
		// source already names the descriptor in its error.
		return ocispec.Descriptor{}, err
	}
	if int64(len(head)) <= peek || peek == bound {
		desc, err := ioutil.CopyUnsafe(io.Discard, bytes.NewReader(head), nil, expected, limit)
		if err != nil {
			return ocispec.Descriptor{}, err
		}
		if err := s.push(ctx, desc, bytes.NewReader(head)); err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("%s: %s: push failed: %w", desc.Digest, desc.MediaType, err)
		}
		return desc, nil
	}

	up, err := s.openUploadSession(ctx, chunk)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("%s: %s: open upload session failed: %w", expected.Digest, expected.MediaType, err)
	}
	if up.chunk > chunk && up.chunk > maxChunkMinLength {
		s.cancelUpload(ctx, up.location)
		return ocispec.Descriptor{}, fmt.Errorf("%s: %s: advertised minimum chunk size %d exceeds limit %d", expected.Digest, expected.MediaType, up.chunk, maxChunkMinLength)
	}
	alg := digest.Canonical
	if expected.Digest != "" {
		alg = expected.Digest.Algorithm()
	}
	up.digester = alg.Digester()
	src := io.LimitReader(io.MultiReader(bytes.NewReader(head), source), bound)
	if err := s.uploadChunks(ctx, up, src); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("%s: %s: upload failed: %w", expected.Digest, expected.MediaType, err)
	}
	if source.err != nil {
		s.cancelUpload(ctx, up.location)
		return ocispec.Descriptor{}, source.err
	}
	n := up.offset
	if expected.Size >= 0 && n < expected.Size {
		s.cancelUpload(ctx, up.location)
		return ocispec.Descriptor{}, fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, io.ErrUnexpectedEOF)
	}
	if n == bound {
		var peek [1]byte
		if _, err := io.ReadFull(source, peek[:]); err == nil {
			s.cancelUpload(ctx, up.location)
			if expected.Size >= 0 {
				return ocispec.Descriptor{}, fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, content.ErrTrailingData)
			}
			return ocispec.Descriptor{}, fmt.Errorf("%s: %s: content size exceeds unsafe push size limit %d: %w", expected.Digest, expected.MediaType, limit, errdef.ErrSizeExceedsLimit)
		} else if err != io.EOF {
			s.cancelUpload(ctx, up.location)
			return ocispec.Descriptor{}, err
		}
	}
	actual := up.digester.Digest()
	if expected.Digest != "" && expected.Digest != actual {
		s.cancelUpload(ctx, up.location)
		return ocispec.Descriptor{}, fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, content.ErrMismatchedDigest)
	}
	if err := s.closeUploadSession(ctx, up, actual); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("%s: %s: close upload session failed: %w", expected.Digest, expected.MediaType, err)
	}
	expected.Digest = actual
	expected.Size = n
	return expected, nil
}

// unsafePushReader preserves source errors that io.ReadFull may otherwise
// discard when they accompany the bytes that complete a chunk.
type unsafePushReader struct {
	// r supplies the unverified content.
	r io.Reader
	// expected supplies descriptor context for read errors.
	expected ocispec.Descriptor
	// err is the first non-EOF source error, wrapped and retained across reads.
	err error
}

func (r *unsafePushReader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	n, err := r.r.Read(p)
	if err != nil && err != io.EOF {
		r.err = fmt.Errorf("%s: %s: read failed: %w", r.expected.Digest, r.expected.MediaType, err)
		return n, r.err
	}
	return n, err
}
