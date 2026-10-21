// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci

import (
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/oci"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

func TestGranularResourceLayersSelectsKind(t *testing.T) {
	t.Parallel()

	root := &oci.Manifest{Manifest: ocispec.Manifest{Layers: []ocispec.Descriptor{
		{Annotations: map[string]string{layout.ResourceKindAnnotation: layout.ResourceKindSBOM}},
		{Annotations: map[string]string{layout.ResourceKindAnnotation: layout.ResourceKindDocumentation}},
		{Annotations: map[string]string{layout.ResourceKindAnnotation: layout.ResourceKindSBOM}},
	}}}

	layers := granularResourceLayers(root, layout.ResourceKindSBOM)
	require.Len(t, layers, 2)
	for _, layer := range layers {
		require.Equal(t, layout.ResourceKindSBOM, layer.Annotations[layout.ResourceKindAnnotation])
	}
}
