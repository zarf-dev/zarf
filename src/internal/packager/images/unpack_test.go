// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"errors"
	"path/filepath"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestUnpackMultipleImages(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		srcDir         string
		expectedImages int
		imageRefs      []string
		expectErr      error
	}{
		{
			name:           "single image",
			srcDir:         "testdata/my-image",
			expectedImages: 1,
			imageRefs: []string{
				"docker.io/library/linux:latest",
			},
		},
		{
			name:   "non-existent",
			srcDir: "testdata/my-image",
			imageRefs: []string{
				"docker.io/library/non-existent-image:linux",
			},
			expectErr: errors.New("could not find image docker.io/library/non-existent-image:linux"),
		},
		{
			name:           "oras OCI layout with multiple images",
			srcDir:         "testdata/oras-oci-layout/images",
			expectedImages: 6,
			imageRefs: []string{
				"docker.io/library/hello-world@sha256:03b62250a3cb1abd125271d393fc08bf0cc713391eda6b57c02d1ef85efcc25c",
				"ghcr.io/zarf-dev/images/hello-world:latest",
				"ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig",
				"localhost:9999/local-test:1.0.0",
				"docker.io/library/local-test:1.0.0",
				"ghcr.io/stefanprodan/charts/podinfo:6.4.0",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)

			// Create a tar from the source directory
			tarFile := filepath.Join(t.TempDir(), "images.tar")
			err := archive.Compress(ctx, []string{tc.srcDir}, tarFile, archive.CompressOpts{})
			require.NoError(t, err)
			dstDir := t.TempDir()
			imageTar := v1alpha1.ImageTar{
				Path:   tarFile,
				Images: tc.imageRefs,
			}

			// Run
			images, err := Unpack(ctx, imageTar, dstDir)
			if tc.expectErr != nil {
				require.ErrorContains(t, err, tc.expectErr.Error())
				return
			}
			require.NoError(t, err)

			// Verify the correct amount of images were found
			require.Len(t, images, tc.expectedImages)
			imageMap := make(map[string]ImageWithManifest)
			for _, img := range images {
				imageMap[img.Image.Reference] = img
			}
			for _, ref := range tc.imageRefs {
				imgRef, err := transform.ParseImageRef(ref)
				require.NoError(t, err)
				img, found := imageMap[imgRef.Reference]
				require.True(t, found, "expected to find image %s", ref)
				require.NotEmpty(t, img.Manifest.Config.Digest)
			}

			// Verify the OCI layout was created properly
			idx, err := getIndexFromOCILayout(dstDir)
			require.NoError(t, err)
			require.Len(t, idx.Manifests, tc.expectedImages)
			for _, descs := range idx.Manifests {
				imageName, ok := descs.Annotations[ocispec.AnnotationRefName]
				require.True(t, ok)
				require.Contains(t, tc.imageRefs, imageName)
			}

			// Verify all images have the required blobs
			for _, img := range images {
				// Verify all layer blobs exist
				for _, layer := range img.Manifest.Layers {
					layerBlobPath := filepath.Join(dstDir, "blobs", "sha256", layer.Digest.Hex())
					require.FileExists(t, layerBlobPath)
				}
			}
		})
	}
}
