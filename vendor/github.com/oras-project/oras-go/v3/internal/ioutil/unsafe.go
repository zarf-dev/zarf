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

package ioutil

import (
	_ "crypto/sha256" // UnsafePush always supports computing the default digest.
	"errors"
	"fmt"
	"io"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/content"
	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/internal/descriptor"
)

// DefaultMaxUnsafePushBytes is the default size limit for UnsafePush in stores
// that do not keep content in memory.
const DefaultMaxUnsafePushBytes int64 = 4 << 30

// ValidateUnsafePush validates the descriptor and effective size limit before
// any content is read or temporary files are created. The limit must be nonnegative.
func ValidateUnsafePush(expected ocispec.Descriptor, limit int64) error {
	alg := digest.Canonical
	if expected.Digest != "" {
		if err := expected.Digest.Validate(); err != nil {
			// go-digest also rejects well-formed digests with unavailable algorithms.
			// Preserve both the validation failure and the unsupported classification.
			if errors.Is(err, digest.ErrDigestUnsupported) {
				return fmt.Errorf("%s: %s: %w: %w", expected.Digest, expected.MediaType, errdef.ErrInvalidDigest, errdef.ErrUnsupported)
			}
			return fmt.Errorf("%s: %s: %w: %v", expected.Digest, expected.MediaType, errdef.ErrInvalidDigest, err)
		}
		alg = expected.Digest.Algorithm()
	}
	if !descriptor.IsSupportedAlgorithm(alg) || !alg.Available() {
		return fmt.Errorf("%s: %s: digest algorithm %q: %w", expected.Digest, expected.MediaType, alg, errdef.ErrUnsupported)
	}
	if limit < 0 || expected.Size > limit {
		return unsafePushLimitError(expected, limit)
	}
	return nil
}

// CopyUnsafe copies and hashes content with a mandatory size limit. It writes at
// most the known size or limit, and reads at most one additional byte to detect
// excess content. Hashes include only bytes successfully written to dst.
func CopyUnsafe(dst io.Writer, src io.Reader, buf []byte, expected ocispec.Descriptor, limit int64) (ocispec.Descriptor, error) {
	if err := ValidateUnsafePush(expected, limit); err != nil {
		return ocispec.Descriptor{}, err
	}
	alg := digest.Canonical
	if expected.Digest != "" {
		alg = expected.Digest.Algorithm()
	}
	digester := alg.Digester()
	bound := limit
	if expected.Size >= 0 {
		bound = expected.Size
	}
	n, err := CopyWithBuffer(io.MultiWriter(dst, digester.Hash()), io.LimitReader(src, bound), buf)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("%s: %s: copy failed: %w", expected.Digest, expected.MediaType, err)
	}
	if expected.Size >= 0 && n < expected.Size {
		return ocispec.Descriptor{}, fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, io.ErrUnexpectedEOF)
	}
	// Probe separately instead of adding one to bound, which may be MaxInt64.
	if n == bound {
		var peek [1]byte
		if _, err := io.ReadFull(src, peek[:]); err == nil {
			if expected.Size >= 0 {
				return ocispec.Descriptor{}, fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, content.ErrTrailingData)
			}
			return ocispec.Descriptor{}, unsafePushLimitError(expected, limit)
		} else if err != io.EOF {
			return ocispec.Descriptor{}, fmt.Errorf("%s: %s: read failed: %w", expected.Digest, expected.MediaType, err)
		}
	}
	actual := digester.Digest()
	if expected.Digest != "" && expected.Digest != actual {
		return ocispec.Descriptor{}, fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, content.ErrMismatchedDigest)
	}
	expected.Digest = actual
	expected.Size = n
	return expected, nil
}

func unsafePushLimitError(expected ocispec.Descriptor, limit int64) error {
	return fmt.Errorf("%s: %s: content size exceeds unsafe push size limit %d: %w", expected.Digest, expected.MediaType, limit, errdef.ErrSizeExceedsLimit)
}
