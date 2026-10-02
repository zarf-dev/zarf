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

func TestGranularResourceLayersSelectsKeys(t *testing.T) {
	t.Parallel()

	root := &oci.Manifest{Manifest: ocispec.Manifest{Layers: []ocispec.Descriptor{
		{Annotations: map[string]string{layout.ResourceKindAnnotation: layout.ResourceKindSBOM, layout.ResourceKeyAnnotation: "component:metrics"}},
		{Annotations: map[string]string{layout.ResourceKindAnnotation: layout.ResourceKindSBOM, layout.ResourceKeyAnnotation: "component:logging"}},
	}}}

	selection := LayerSelection{
		Types: []LayerType{SbomLayers},
		ResourceKeys: map[LayerType][]string{
			SbomLayers: {"component:metrics"},
		},
	}
	layers, err := granularResourceLayers(root, layout.ResourceKindSBOM, selection.ResourceKeys[SbomLayers])
	require.NoError(t, err)
	require.Len(t, layers, 1)
	require.Equal(t, "component:metrics", layers[0].Annotations[layout.ResourceKeyAnnotation])
	require.ErrorContains(t, func() error {
		_, err := granularResourceLayers(root, layout.ResourceKindSBOM, []string{"component:missing"})
		return err
	}(), "component:missing")
}
