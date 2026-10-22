// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci

import (
	"bytes"
	"context"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/internal/checksum"
	"github.com/zarf-dev/zarf/src/pkg/oci"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
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

func TestChecksumVerifiedSBOMLayersRejectsKindOmission(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	resourcePath := layout.SBOMResourcePath("component:metrics")
	resource := content.NewDescriptorFromBytes(layout.SBOMMediaTypeSyftJSON, []byte("sbom"))
	resource.Annotations = map[string]string{
		ocispec.AnnotationTitle:       resourcePath,
		layout.ResourceKindAnnotation: layout.ResourceKindSBOM,
	}
	checksums := []byte(resource.Digest.Encoded() + " " + resourcePath + "\n")
	aggregateChecksum, err := checksum.GetSHA256Hash(bytes.NewReader(checksums))
	require.NoError(t, err)
	checksumDesc := content.NewDescriptorFromBytes(layout.ZarfLayerMediaTypeBlob, checksums)
	checksumDesc.Annotations = map[string]string{ocispec.AnnotationTitle: layout.Checksums}
	store := memory.New()
	require.NoError(t, store.Push(ctx, checksumDesc, bytes.NewReader(checksums)))
	pkg := api.Package{Build: api.BuildData{AggregateChecksum: aggregateChecksum}}

	layers, err := checksumVerifiedSBOMLayers(ctx, &oci.Manifest{Manifest: ocispec.Manifest{
		Layers: []ocispec.Descriptor{checksumDesc, resource},
	}}, store, pkg)
	require.NoError(t, err)
	require.Equal(t, []ocispec.Descriptor{resource}, layers)

	resource.Annotations = map[string]string{
		ocispec.AnnotationTitle: resourcePath,
	}
	_, err = checksumVerifiedSBOMLayers(ctx, &oci.Manifest{Manifest: ocispec.Manifest{
		Layers: []ocispec.Descriptor{checksumDesc, resource},
	}}, store, pkg)
	require.ErrorContains(t, err, `SBOM resource "`+resourcePath+`" has an invalid OCI kind`)
}
