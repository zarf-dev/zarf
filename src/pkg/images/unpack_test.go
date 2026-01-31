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
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestGetRefFromManifest(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		desc     ocispec.Descriptor
		expected string
	}{
		{
			name: "io.containerd.image.name present",
			desc: ocispec.Descriptor{
				Digest: "sha256:b20377b80653db287c2047b8effbd2458d045ee9c43098cf57d769fd6fc1a110",
				Annotations: map[string]string{
					"io.containerd.image.name":                    "docker.io/library/nginx:perl",
					"org.opencontainers.image.ref.name":           "perl",
					"containerd.io/distribution.source.docker.io": "library/nginx",
				},
			},
			expected: "docker.io/library/nginx:perl",
		},
		{
			name: "only containerd.io/distribution.source.docker.io present",
			desc: ocispec.Descriptor{
				Digest: "sha256:b20377b80653db287c2047b8effbd2458d045ee9c43098cf57d769fd6fc1a110",
				Annotations: map[string]string{
					"containerd.io/distribution.source.docker.io": "library/nginx",
				},
			},
			expected: "library/nginx@sha256:b20377b80653db287c2047b8effbd2458d045ee9c43098cf57d769fd6fc1a110",
		},
		{
			name: "org.opencontainers.image.ref.name present",
			desc: ocispec.Descriptor{
				Digest: "sha256:b20377b80653db287c2047b8effbd2458d045ee9c43098cf57d769fd6fc1a110",
				Annotations: map[string]string{
					"org.opencontainers.image.ref.name": "registry.com/podman-or-oras:0.0.1",
				},
			},
			expected: "registry.com/podman-or-oras:0.0.1",
		},
		{
			name:     "no annotations",
			desc:     ocispec.Descriptor{},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := getRefFromManifest(tc.desc)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestFindImagesInManifests(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		manifests      []ocispec.Descriptor
		expectedImages []string
		expectErr      error
	}{
		{
			name: "single manifest descriptor, single image",
			manifests: []ocispec.Descriptor{
				{
					Annotations: map[string]string{
						dockerRefAnnotation: "docker.io/library/hello-world:linux",
					},
				},
			},
			expectedImages: []string{"docker.io/library/hello-world:linux"},
		},
		{
			name: "multiple manifest descriptors, blank image name",
			manifests: []ocispec.Descriptor{
				{
					Annotations: map[string]string{
						dockerRefAnnotation: "docker.io/library/hello-world@sha256:03b62250a3cb1abd125271d393fc08bf0cc713391eda6b57c02d1ef85efcc25c",
					},
				},
				{
					Annotations: map[string]string{
						dockerRefAnnotation: "localhost:9999/local-test:1.0.0",
					},
				},
				{
					Annotations: nil,
				},
			},
			expectedImages: []string{
				"docker.io/library/hello-world@sha256:03b62250a3cb1abd125271d393fc08bf0cc713391eda6b57c02d1ef85efcc25c",
				"localhost:9999/local-test:1.0.0",
			},
		},
		{
			name: "invalid image name",
			manifests: []ocispec.Descriptor{
				{
					Annotations: map[string]string{
						dockerRefAnnotation: "localhost:9999/local-test@hello-world:1.0.0",
					},
				},
			},
			expectedImages: []string{},
			expectErr:      errors.New("failed to parse image reference localhost:9999/local-test@hello-world:1.0.0"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			images, err := FindImagesInManifests(tc.manifests)
			if tc.expectErr != nil {
				require.ErrorContains(t, err, tc.expectErr.Error())
				return
			}
			require.NoError(t, err)

			for _, img := range tc.expectedImages {
				require.Contains(t, images, img)
			}
		})
	}
}

func TestGetManifestsFromArchive(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		srcDir        string
		expectedImage string
		expectErr     error
	}{
		{
			name:          "single archive",
			srcDir:        filepath.Join("testdata", "docker-graph-driver-image-store"),
			expectedImage: "docker.io/library/hello-world:linux",
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
			manifests, err := GetManifestsFromArchive(ctx, tarFile)
			if tc.expectErr != nil {
				require.ErrorContains(t, err, tc.expectErr.Error())
			}
			require.NoError(t, err)

			for _, manifest := range manifests {
				require.Equal(t, tc.expectedImage, manifest.Annotations[dockerRefAnnotation])
			}
		})
	}

}

func TestUnpackMultipleImages(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		srcDir          string
		requestedImages []string
		expectErr       error
	}{
		{
			name:   "single image, docker image store",
			srcDir: filepath.Join("testdata", "docker-graph-driver-image-store"),
			requestedImages: []string{
				"docker.io/library/hello-world:linux",
			},
		},
		{
			name:   "single image, docker containerd store",
			srcDir: filepath.Join("testdata", "docker-containerd-image-store"),
			requestedImages: []string{
				"docker.io/library/hello-world:linux",
			},
		},
		{
			name:   "pull several images, including non-container images",
			srcDir: filepath.Join("testdata", "oras-oci-layout", "images"),
			requestedImages: []string{
				"docker.io/library/hello-world@sha256:03b62250a3cb1abd125271d393fc08bf0cc713391eda6b57c02d1ef85efcc25c",
				"ghcr.io/zarf-dev/images/hello-world:latest",
				"ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig",
				"localhost:9999/local-test:1.0.0",
				"docker.io/library/local-test:1.0.0",
				"ghcr.io/stefanprodan/charts/podinfo:6.4.0",
			},
		},
		{
			name:   "non-existent image",
			srcDir: filepath.Join("testdata", "docker-graph-driver-image-store"),
			requestedImages: []string{
				"docker.io/library/hello-world:linux",
				"docker.io/library/non-existent-image:linux",
			},
			expectErr: errors.New("could not find image docker.io/library/non-existent-image:linux"),
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
			imageArchives := v1alpha1.ImageArchive{
				Path:   tarFile,
				Images: tc.requestedImages,
			}

			// Run
			images, err := Unpack(ctx, imageArchives, dstDir, "amd64")
			if tc.expectErr != nil {
				require.ErrorContains(t, err, tc.expectErr.Error())
				return
			}
			require.NoError(t, err)

			imageMap := make(map[string]ImageWithManifest)
			for _, img := range images {
				imageMap[img.Image.Reference] = img
			}

			for _, ref := range tc.requestedImages {
				img, found := imageMap[ref]
				require.True(t, found)
				require.NotEmpty(t, img.Manifest.Config.Digest)
			}

			idx, err := getIndexFromOCILayout(dstDir)
			require.NoError(t, err)

			// Verify manifests are annotated
			for _, descs := range idx.Manifests {
				imageName, ok := descs.Annotations[ocispec.AnnotationRefName]
				require.True(t, ok)
				require.Contains(t, tc.requestedImages, imageName)
			}

			// Verify all the required layers exist in the oci layout
			for _, img := range images {
				for _, layer := range img.Manifest.Layers {
					layerBlobPath := filepath.Join(dstDir, "blobs", "sha256", layer.Digest.Hex())
					require.FileExists(t, layerBlobPath)
				}
			}
		})
	}
}
