// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import (
	"testing"

	godigest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
)

func TestManifestLocatePrefersMountPath(t *testing.T) {
	t.Parallel()

	mountPath := "sboms/images/aW1hZ2U6bG9uZ2hvcm4.json"
	titleOnly := ocispec.Descriptor{
		Digest:      godigest.FromString("title only"),
		Size:        1,
		Annotations: map[string]string{ocispec.AnnotationTitle: mountPath},
	}
	mountedResource := ocispec.Descriptor{
		Digest: godigest.FromString("mounted resource"),
		Size:   2,
		Annotations: map[string]string{
			ocispec.AnnotationTitle:     "image:longhornio/longhorn-ui:v1.11.2",
			ResourceMountPathAnnotation: mountPath,
		},
	}
	manifest := &Manifest{Manifest: ocispec.Manifest{Layers: []ocispec.Descriptor{titleOnly, mountedResource}}}

	require.Equal(t, mountedResource, manifest.Locate(mountPath))
	require.Equal(t, mountedResource, manifest.Locate("image:longhornio/longhorn-ui:v1.11.2"))
}
