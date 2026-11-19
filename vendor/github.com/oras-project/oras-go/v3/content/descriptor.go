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
	"fmt"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/internal/descriptor"
)

// NewDescriptorFromBytes returns a descriptor, given the content and media type.
// If no media type is specified, "application/octet-stream" will be used.
func NewDescriptorFromBytes(mediaType string, content []byte) ocispec.Descriptor {
	if mediaType == "" {
		mediaType = descriptor.DefaultMediaType
	}
	return ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    digest.FromBytes(content),
		Size:      int64(len(content)),
	}
}

// NewDescriptorFromBytesWithAlgorithm returns a descriptor like
// [NewDescriptorFromBytes], with the digest computed using alg.
// If alg is empty, [digest.Canonical] is used.
// It returns [errdef.ErrUnsupported] if alg is not one of SHA-256, SHA-384 and
// SHA-512, or is not available, which requires its hash package to be linked
// into the binary.
func NewDescriptorFromBytesWithAlgorithm(mediaType string, content []byte, alg digest.Algorithm) (ocispec.Descriptor, error) {
	if alg == "" {
		alg = digest.Canonical
	}
	if !descriptor.IsSupportedAlgorithm(alg) || !alg.Available() {
		return ocispec.Descriptor{}, fmt.Errorf("digest algorithm %q: %w", alg, errdef.ErrUnsupported)
	}
	if mediaType == "" {
		mediaType = descriptor.DefaultMediaType
	}
	return ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    alg.FromBytes(content),
		Size:      int64(len(content)),
	}, nil
}

// Equal returns true if two descriptors point to the same content.
func Equal(a, b ocispec.Descriptor) bool {
	return a.Size == b.Size && a.Digest == b.Digest && a.MediaType == b.MediaType
}
