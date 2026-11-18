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

package content

import (
	"context"
	"io"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Fetcher fetches content.
type Fetcher interface {
	// Fetch fetches the content identified by the descriptor.
	Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error)
}

// Pusher pushes content.
type Pusher interface {
	// Push pushes the content, matching the expected descriptor.
	// Reader is preferred to Writer so that the suitable buffer size can be
	// chosen by the underlying implementation. Furthermore, the implementation
	// can also do reflection on the Reader for more advanced I/O optimization.
	Push(ctx context.Context, expected ocispec.Descriptor, content io.Reader) error
}

// UnsafePusher pushes content whose descriptor is not fully known in advance.
// It is "unsafe" because, when the digest is not supplied, the integrity of the
// pushed content against what the caller intended cannot be guaranteed.
//
// An empty digest is computed using SHA-256. A supplied digest must be valid
// and use a supported, available algorithm. A digest mismatch returns
// ErrMismatchedDigest. Recalculation preserves all descriptor fields other than
// Digest and Size.
//
// Supplied sizes, including zero, must match exactly. Short content returns
// io.ErrUnexpectedEOF, and extra content with a known size returns ErrTrailingData.
// Implementations enforce a size limit for both known and unknown sizes.
// Content exceeding the configured limit returns errdef.ErrSizeExceedsLimit;
// a known size exceeding the limit is rejected before reading. Implementations
// read at most one byte beyond the known size or limit to detect excess content.
//
// If both digest and size are known, existing content is checked before reading.
// Otherwise, content is fully read and verified before checking for duplicates.
// On ErrAlreadyExists, the returned descriptor is the expected or computed
// descriptor; the existing content is not re-verified.
//
// On other errors, the returned descriptor is empty. As with Push, if graph
// indexing or tagging fails after content is stored, the content remains stored
// and an error is returned. Validation occurs before reading content. A canceled
// context is checked before starting and before each read of content; a read
// already blocked in the reader is not interrupted.
type UnsafePusher interface {
	// UnsafePush pushes the content, matching the specified field in the expected descriptor.
	//   - The Digest of the expected descriptor is recalculated if empty
	//   - The Size of the expected descriptor is recalculated if less than zero
	// The recalculated descriptor is returned upon a successful push.
	UnsafePush(ctx context.Context, expected ocispec.Descriptor, content io.Reader) (ocispec.Descriptor, error)
}

// Storage represents a content-addressable storage (CAS) where contents are
// accessed via Descriptors.
// The storage is designed to handle blobs of large sizes.
type Storage interface {
	ReadOnlyStorage
	Pusher
}

// ReadOnlyStorage represents a read-only Storage.
type ReadOnlyStorage interface {
	Fetcher

	// Exists returns true if the described content exists.
	Exists(ctx context.Context, target ocispec.Descriptor) (bool, error)
}

// Deleter removes content.
// Deleter is an extension of Storage.
type Deleter interface {
	// Delete removes the content identified by the descriptor.
	Delete(ctx context.Context, target ocispec.Descriptor) error
}

// FetchAll safely fetches the content described by the descriptor.
// The fetched content is verified against the size and the digest.
func FetchAll(ctx context.Context, fetcher Fetcher, desc ocispec.Descriptor) ([]byte, error) {
	rc, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return ReadAll(rc, desc)
}

// FetcherFunc is the basic Fetch method defined in Fetcher.
type FetcherFunc func(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error)

// Fetch performs Fetch operation by the FetcherFunc.
func (fn FetcherFunc) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	return fn(ctx, target)
}
