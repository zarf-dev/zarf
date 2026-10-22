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
	manifest := &Manifest{Manifest: ocispec.Manifest{Layers: []ocispec.Descriptor{descriptor}}}

	require.Equal(t, descriptor, manifest.Locate("sboms/images/resource.json"))
	require.Equal(t, descriptor, manifest.Locate(descriptor.Digest.Encoded()))
	require.True(t, IsEmptyDescriptor(manifest.Locate("missing")))
}
