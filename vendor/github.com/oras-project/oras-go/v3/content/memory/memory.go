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

// Package memory provides implementation of a memory backed content store.
package memory

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/content"
	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/internal/cas"
	"github.com/oras-project/oras-go/v3/internal/graph"
	"github.com/oras-project/oras-go/v3/internal/ioutil"
	"github.com/oras-project/oras-go/v3/internal/resolver"
)

// Store represents a memory based store, which implements `oras.Target`.
type Store struct {
	// MaxUnsafePushBytes is the maximum number of bytes UnsafePush accepts for
	// both known and unknown sizes. If <= 0, the default is used. Set it before
	// using the store concurrently. Peak memory during an UnsafePush can be a
	// small multiple of the limit, as content is buffered and then stored as a copy.
	//   - Default value: 256 MiB.
	MaxUnsafePushBytes int64

	storage  content.Storage
	resolver content.TagResolver
	graph    *graph.Memory
}

const defaultMaxUnsafePushBytes int64 = 256 << 20

// Match content.ReadAll's bound on eager allocation from an untrusted size.
const maxInitialUnsafePushBufferSize = 32 << 20

// New creates a new memory based store.
func New() *Store {
	return &Store{
		storage:  cas.NewMemory(),
		resolver: resolver.NewMemory(),
		graph:    graph.NewMemory(),
	}
}

// Fetch fetches the content identified by the descriptor.
func (s *Store) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	return s.storage.Fetch(ctx, target)
}

// Push pushes the content, matching the expected descriptor.
func (s *Store) Push(ctx context.Context, expected ocispec.Descriptor, reader io.Reader) error {
	if err := s.storage.Push(ctx, expected, reader); err != nil {
		return err
	}

	// index predecessors.
	// there is no data consistency issue as long as deletion is not implemented
	// for the memory store.
	return s.graph.Index(ctx, s.storage, expected)
}

// UnsafePush pushes content whose digest or size may be unknown, subject to
// MaxUnsafePushBytes. See content.UnsafePusher for verification and error semantics.
func (s *Store) UnsafePush(ctx context.Context, expected ocispec.Descriptor, r io.Reader) (ocispec.Descriptor, error) {
	if err := ctx.Err(); err != nil {
		return ocispec.Descriptor{}, err
	}
	limit := s.MaxUnsafePushBytes
	if limit <= 0 {
		limit = defaultMaxUnsafePushBytes
	}
	if err := ioutil.ValidateUnsafePush(expected, limit); err != nil {
		return ocispec.Descriptor{}, err
	}
	if expected.Digest != "" && expected.Size >= 0 {
		exists, err := s.storage.Exists(ctx, expected)
		if err != nil {
			return ocispec.Descriptor{}, err
		}
		if exists {
			return expected, fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, errdef.ErrAlreadyExists)
		}
	}
	var buf bytes.Buffer
	if expected.Size >= 0 && expected.Size <= math.MaxInt {
		buf.Grow(int(min(expected.Size, maxInitialUnsafePushBufferSize)))
	}
	desc, err := ioutil.CopyUnsafe(&buf, ioutil.NewContextReader(ctx, r), nil, expected, limit)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	if err := s.storage.Push(ctx, desc, bytes.NewReader(buf.Bytes())); err != nil {
		if errors.Is(err, errdef.ErrAlreadyExists) {
			return desc, err
		}
		return ocispec.Descriptor{}, err
	}
	if err := s.graph.Index(ctx, s.storage, desc); err != nil {
		return ocispec.Descriptor{}, err
	}
	return desc, nil
}

// Exists returns true if the described content exists.
func (s *Store) Exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	return s.storage.Exists(ctx, target)
}

// Resolve resolves a reference to a descriptor.
func (s *Store) Resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	return s.resolver.Resolve(ctx, reference)
}

// Tag tags a descriptor with a reference string.
// Returns ErrNotFound if the tagged content does not exist.
func (s *Store) Tag(ctx context.Context, desc ocispec.Descriptor, reference string) error {
	exists, err := s.storage.Exists(ctx, desc)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%s: %s: %w", desc.Digest, desc.MediaType, errdef.ErrNotFound)
	}
	return s.resolver.Tag(ctx, desc, reference)
}

// Predecessors returns the nodes directly pointing to the current node.
// Predecessors returns nil without error if the node does not exists in the
// store.
// Like other operations, calling Predecessors() is go-routine safe. However,
// it does not necessarily correspond to any consistent snapshot of the stored
// contents.
func (s *Store) Predecessors(ctx context.Context, node ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	return s.graph.Predecessors(ctx, node)
}
