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

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/internal/ioutil"
	"github.com/oras-project/oras-go/v3/registry/remote/auth"
)

// UnsafePush pushes a manifest whose digest or size may be unknown, bounded by
// [Registry.MaxMetadataBytes] (4 MiB by default). See
// [github.com/oras-project/oras-go/v3/content.UnsafePusher] for verification and
// error semantics. Content is fully buffered and verified before any PUT.
// Client-side referrers indexing copies it again, so peak memory per call is a
// small multiple of the size limit.
//
// When both digest and size are known, existing content is checked before reading;
// otherwise, it is checked after verification. A duplicate of the same size returns
// the expected or computed descriptor and errdef.ErrAlreadyExists without a PUT.
// When the digest is unknown, manifest policy is evaluated on the computed
// descriptor before checking for duplicates or pushing. Referrers are indexed as
// with Push; an indexing failure may leave the manifest stored and returns an
// empty descriptor and an error.
func (s *manifestStore) UnsafePush(ctx context.Context, expected ocispec.Descriptor, r io.Reader) (ocispec.Descriptor, error) {
	limit := s.repo.maxMetadataBytes()
	if limit <= 0 {
		limit = defaultMaxMetadataBytes
	}
	if err := ioutil.ValidateUnsafePush(expected, limit); err != nil {
		return ocispec.Descriptor{}, err
	}
	if err := checkDigestAlgorithm(digestSourceDescriptor, expected.Digest); err != nil {
		return ocispec.Descriptor{}, err
	}
	if !isManifest(s.repo.manifestMediaTypes(), expected) {
		return ocispec.Descriptor{}, fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, errdef.ErrInvalidMediaType)
	}
	// Validation deliberately precedes context and policy checks, since a policy
	// verifier may call back into the registry even for invalid input.
	if err := ctx.Err(); err != nil {
		return ocispec.Descriptor{}, err
	}
	if expected.Digest != "" {
		if err := s.repo.checkWritePolicy(ctx, expected.Digest.String()); err != nil {
			return ocispec.Descriptor{}, err
		}
	} else if err := s.repo.checkPolicy(ctx, ""); err != nil {
		return ocispec.Descriptor{}, err
	}
	ctx = auth.AppendRepositoryScope(ctx, s.repo.reference(), auth.ActionPull, auth.ActionPush)
	known := expected.Digest != "" && expected.Size >= 0
	if known {
		desc, err := s.resolve(ctx, expected.Digest.String())
		if err != nil && !errors.Is(err, errdef.ErrNotFound) {
			return ocispec.Descriptor{}, err
		}
		if err == nil && desc.Size == expected.Size {
			return expected, fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, errdef.ErrAlreadyExists)
		}
	}

	var buf bytes.Buffer
	desc, err := ioutil.CopyUnsafe(&buf, ioutil.NewContextReader(ctx, r), nil, expected, limit)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	if expected.Digest == "" {
		if err := s.repo.checkWritePolicy(ctx, desc.Digest.String()); err != nil {
			return ocispec.Descriptor{}, err
		}
	}
	if !known {
		existing, err := s.resolve(ctx, desc.Digest.String())
		if err != nil && !errors.Is(err, errdef.ErrNotFound) {
			return ocispec.Descriptor{}, err
		}
		if err == nil && existing.Size == desc.Size {
			return desc, fmt.Errorf("%s: %s: %w", desc.Digest, desc.MediaType, errdef.ErrAlreadyExists)
		}
	}
	if err := s.pushWithIndexing(ctx, desc, bytes.NewReader(buf.Bytes()), desc.Digest.String()); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("%s: %s: push failed: %w", desc.Digest, desc.MediaType, err)
	}
	return desc, nil
}
