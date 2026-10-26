// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import (
	"testing"

	godigest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
)

func TestManifestLocate(t *testing.T) {
	t.Parallel()

	descriptor := ocispec.Descriptor{
		Digest:      godigest.FromString("resource"),
		Size:        1,
		Annotations: map[string]string{ocispec.AnnotationTitle: "sboms/images/resource.json"},
	}
	digestMatch := ocispec.Descriptor{
		Digest:      godigest.FromString("digest-match"),
		Size:        2,
		Annotations: map[string]string{ocispec.AnnotationTitle: "sboms/images/digest-match.json"},
	}
	titleBeforeDigest := ocispec.Descriptor{
		Digest:      godigest.FromString("title-before-digest"),
		Size:        3,
		Annotations: map[string]string{ocispec.AnnotationTitle: digestMatch.Digest.Encoded()},
	}
	manifest := &Manifest{Manifest: ocispec.Manifest{Layers: []ocispec.Descriptor{descriptor, titleBeforeDigest, digestMatch}}}

	require.Equal(t, descriptor, manifest.Locate("sboms/images/resource.json"))
	require.Equal(t, digestMatch, manifest.Locate(digestMatch.Digest.Encoded()))
	require.True(t, IsEmptyDescriptor(manifest.Locate("missing")))
}
