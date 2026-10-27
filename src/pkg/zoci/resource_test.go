// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/internal/checksum"
	"github.com/zarf-dev/zarf/src/pkg/oci"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
)

func TestAttachmentResourceLayers(t *testing.T) {
	t.Parallel()

	attachmentDescriptor := func(title string) ocispec.Descriptor {
		return ocispec.Descriptor{
			MediaType: layout.AttachmentLayerMediaTypeBlob,
			Annotations: map[string]string{
				ocispec.AnnotationTitle:       title,
				layout.ResourceKindAnnotation: layout.ResourceKindAttachment,
			},
		}
	}
	attachments := map[string]string{"readme": "README.md", "license": "LICENSE"}
	readme := attachmentDescriptor("attachments/README.md")
	license := attachmentDescriptor("attachments/LICENSE")

	tests := []struct {
		name          string
		attachments   map[string]string
		keys          []string
		layers        []ocispec.Descriptor
		expected      []ocispec.Descriptor
		errorContains string
	}{
		{
			name:        "all attachments",
			attachments: attachments,
			layers:      []ocispec.Descriptor{readme, license},
			expected:    []ocispec.Descriptor{readme, license},
		},
		{
			name:        "single requested key",
			attachments: attachments,
			keys:        []string{"readme"},
			layers:      []ocispec.Descriptor{readme, license},
			expected:    []ocispec.Descriptor{readme},
		},
		{
			name:        "colliding source basenames",
			attachments: map[string]string{"first": "docs/README.md", "second": "other/README.md"},
			keys:        []string{"second"},
			layers: []ocispec.Descriptor{
				attachmentDescriptor("attachments/first-README.md"),
				attachmentDescriptor("attachments/second-README.md"),
			},
			expected: []ocispec.Descriptor{attachmentDescriptor("attachments/second-README.md")},
		},
		{
			name:          "unknown key",
			attachments:   attachments,
			keys:          []string{"missing"},
			layers:        []ocispec.Descriptor{readme, license},
			errorContains: "attachment key missing not found in package",
		},
		{
			name:          "missing title",
			attachments:   attachments,
			layers:        []ocispec.Descriptor{readme},
			errorContains: `attachment resource "attachments/LICENSE" is missing from OCI manifest`,
		},
		{
			name:          "duplicate title",
			attachments:   map[string]string{"readme": "README.md"},
			layers:        []ocispec.Descriptor{readme, readme},
			errorContains: `multiple OCI descriptors represent attachment resource "attachments/README.md"`,
		},
		{
			name:        "invalid kind",
			attachments: map[string]string{"readme": "README.md"},
			layers: []ocispec.Descriptor{{
				MediaType: layout.AttachmentLayerMediaTypeBlob,
				Annotations: map[string]string{
					ocispec.AnnotationTitle:       "attachments/README.md",
					layout.ResourceKindAnnotation: layout.ResourceKindSBOM,
				},
			}},
			errorContains: `attachment resource "attachments/README.md" has an invalid OCI kind`,
		},
		{
			name:        "invalid media type",
			attachments: map[string]string{"readme": "README.md"},
			layers: []ocispec.Descriptor{{
				MediaType: layout.ZarfLayerMediaTypeBlob,
				Annotations: map[string]string{
					ocispec.AnnotationTitle:       "attachments/README.md",
					layout.ResourceKindAnnotation: layout.ResourceKindAttachment,
				},
			}},
			errorContains: `attachment resource "attachments/README.md" has an invalid OCI media type`,
		},
		{
			name:        "undeclared attachment kind",
			attachments: map[string]string{"readme": "README.md"},
			layers: []ocispec.Descriptor{
				readme,
				attachmentDescriptor("attachments/EXTRA.md"),
			},
			errorContains: `attachment resource "attachments/EXTRA.md" is not declared by the package`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := &oci.Manifest{Manifest: ocispec.Manifest{Layers: tt.layers}}
			layers, err := attachmentResourceLayers(root, tt.attachments, tt.keys)
			if tt.errorContains != "" {
				require.ErrorContains(t, err, tt.errorContains)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, layers)
		})
	}
}

func TestChecksumVerifiedSBOMLayersSelectsRequestedKeys(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	componentResourcePath := layout.ComponentSBOMResourcePath("metrics")
	componentResource := content.NewDescriptorFromBytes(layout.SBOMMediaTypeSyftJSON, []byte("component metrics sbom"))
	componentResource.Annotations = map[string]string{
		ocispec.AnnotationTitle:       componentResourcePath,
		layout.ResourceKindAnnotation: layout.ResourceKindSBOM,
	}
	imageResourcePath := layout.ImageSBOMResourcePath("metrics")
	imageResource := content.NewDescriptorFromBytes(layout.SBOMMediaTypeSyftJSON, []byte("image metrics sbom"))
	imageResource.Annotations = map[string]string{
		ocispec.AnnotationTitle:       imageResourcePath,
		layout.ResourceKindAnnotation: layout.ResourceKindSBOM,
	}
	imageReference := "ghcr.io/acme/api:v1.4.2"
	imageReferenceResourcePath := layout.ImageSBOMResourcePath(imageReference)
	imageReferenceResource := content.NewDescriptorFromBytes(layout.SBOMMediaTypeSyftJSON, []byte("api sbom"))
	imageReferenceResource.Annotations = map[string]string{
		ocispec.AnnotationTitle:       imageReferenceResourcePath,
		layout.ResourceKindAnnotation: layout.ResourceKindSBOM,
	}
	unselectedResourcePath := layout.ComponentSBOMResourcePath("logs")
	unselectedResource := content.NewDescriptorFromBytes(layout.SBOMMediaTypeSyftJSON, []byte("logs sbom"))
	unselectedResource.Annotations = map[string]string{
		ocispec.AnnotationTitle:       unselectedResourcePath,
		layout.ResourceKindAnnotation: layout.ResourceKindSBOM,
	}
	checksums := []byte(fmt.Sprintf("%s %s\n%s %s\n%s %s\n%s %s\n",
		componentResource.Digest.Encoded(), componentResourcePath,
		imageResource.Digest.Encoded(), imageResourcePath,
		imageReferenceResource.Digest.Encoded(), imageReferenceResourcePath,
		unselectedResource.Digest.Encoded(), unselectedResourcePath,
	))
	aggregateChecksum, err := checksum.GetSHA256Hash(bytes.NewReader(checksums))
	require.NoError(t, err)

	checksumDesc := content.NewDescriptorFromBytes(layout.ZarfLayerMediaTypeBlob, checksums)
	checksumDesc.Annotations = map[string]string{ocispec.AnnotationTitle: layout.Checksums}
	zarfYAML := []byte(fmt.Sprintf(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: package
build:
  aggregateChecksum: %s
`, aggregateChecksum))
	zarfYAMLDesc := content.NewDescriptorFromBytes(layout.ZarfLayerMediaTypeBlob, zarfYAML)
	zarfYAMLDesc.Annotations = map[string]string{ocispec.AnnotationTitle: layout.ZarfYAML}
	store := memory.New()
	require.NoError(t, store.Push(ctx, zarfYAMLDesc, bytes.NewReader(zarfYAML)))
	require.NoError(t, store.Push(ctx, checksumDesc, bytes.NewReader(checksums)))

	root := &oci.Manifest{Manifest: ocispec.Manifest{
		Layers: []ocispec.Descriptor{
			zarfYAMLDesc,
			checksumDesc,
			componentResource,
			imageResource,
			imageReferenceResource,
			unselectedResource,
		},
	}}
	selection := LayerSelection{SBOMs: &SBOMSelection{Keys: []string{"metrics"}}}
	layers, err := AssembleLayers(ctx, root, store, nil, selection)
	require.NoError(t, err)
	require.Equal(t, []ocispec.Descriptor{zarfYAMLDesc, checksumDesc, componentResource, imageResource}, layers)

	imageLayers, err := AssembleLayers(ctx, root, store, nil, LayerSelection{
		SBOMs: &SBOMSelection{Keys: []string{imageReference}},
	})
	require.NoError(t, err)
	require.Equal(t, []ocispec.Descriptor{zarfYAMLDesc, checksumDesc, imageReferenceResource}, imageLayers)

	unselectedResource.Annotations = map[string]string{
		ocispec.AnnotationTitle: unselectedResourcePath,
	}
	root.Layers[5] = unselectedResource
	_, err = AssembleLayers(ctx, root, store, nil, selection)
	require.ErrorContains(t, err, `SBOM resource "`+unselectedResourcePath+`" has an invalid OCI kind`)

	root.Layers[5] = content.NewDescriptorFromBytes(layout.SBOMMediaTypeSyftJSON, []byte("logs sbom"))
	root.Layers[5].Annotations = map[string]string{
		ocispec.AnnotationTitle:       unselectedResourcePath,
		layout.ResourceKindAnnotation: layout.ResourceKindSBOM,
	}
	_, err = AssembleLayers(ctx, root, store, nil, LayerSelection{
		SBOMs: &SBOMSelection{Keys: []string{"missing"}},
	})
	require.ErrorContains(t, err, `SBOM key "missing" not found in package`)
}

func TestAssembleLayersSelectsLegacyResourceArchives(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	zarfYAML := []byte(`kind: ZarfPackageConfig
metadata:
  name: package
documentation:
  readme: README.md
`)
	zarfYAMLDesc := content.NewDescriptorFromBytes(layout.ZarfLayerMediaTypeBlob, zarfYAML)
	zarfYAMLDesc.Annotations = map[string]string{ocispec.AnnotationTitle: layout.ZarfYAML}
	documentationDesc := content.NewDescriptorFromBytes(layout.ZarfLayerMediaTypeBlob, []byte("documentation"))
	documentationDesc.Annotations = map[string]string{ocispec.AnnotationTitle: layout.DocumentationTar}
	sbomDesc := content.NewDescriptorFromBytes(layout.ZarfLayerMediaTypeBlob, []byte("sboms"))
	sbomDesc.Annotations = map[string]string{ocispec.AnnotationTitle: layout.SBOMTar}

	store := memory.New()
	require.NoError(t, store.Push(ctx, zarfYAMLDesc, bytes.NewReader(zarfYAML)))
	root := &oci.Manifest{Manifest: ocispec.Manifest{
		Layers: []ocispec.Descriptor{zarfYAMLDesc, documentationDesc, sbomDesc},
	}}

	attachmentLayers, err := AssembleLayers(ctx, root, store, nil, LayerSelection{
		Attachments: &AttachmentSelection{},
	})
	require.NoError(t, err)
	require.Equal(t, []ocispec.Descriptor{zarfYAMLDesc, documentationDesc}, attachmentLayers)

	sbomLayers, err := AssembleLayers(ctx, root, store, nil, LayerSelection{SBOMs: &SBOMSelection{}})
	require.NoError(t, err)
	require.Equal(t, []ocispec.Descriptor{zarfYAMLDesc, sbomDesc}, sbomLayers)
}
