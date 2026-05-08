// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2/content/oci"
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

			seen := make(map[string]bool)
			for _, img := range images {
				seen[img.Image.Reference] = true
			}
			for _, ref := range tc.requestedImages {
				require.True(t, seen[ref], "expected pulled image for %s", ref)
			}

			idx, err := getIndexFromOCILayout(dstDir)
			require.NoError(t, err)

			// Verify manifests are annotated
			for _, descs := range idx.Manifests {
				imageName, ok := descs.Annotations[ocispec.AnnotationRefName]
				require.True(t, ok)
				require.Contains(t, tc.requestedImages, imageName)
			}

			// Verify every manifest's layers landed on disk by re-reading from the layout.
			for _, m := range idx.Manifests {
				if !IsManifest(m.MediaType) {
					continue
				}
				manifestPath := filepath.Join(dstDir, "blobs", "sha256", m.Digest.Hex())
				body, err := os.ReadFile(manifestPath)
				require.NoError(t, err)
				var manifest ocispec.Manifest
				require.NoError(t, json.Unmarshal(body, &manifest))
				for _, layer := range manifest.Layers {
					require.FileExists(t, filepath.Join(dstDir, "blobs", "sha256", layer.Digest.Hex()))
				}
			}
		})
	}
}

func TestUnpackImageIndexes(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	upstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)

	platforms := []ocispec.Platform{
		{OS: "linux", Architecture: "amd64"},
		{OS: "linux", Architecture: "arm64"},
	}
	multiArchDigest := testutil.PushMultiArchIndex(ctx, t, upstream+"/fixtures/multi", "v1", platforms)
	nestedDigest := testutil.PushNestedIndex(ctx, t, upstream+"/fixtures/nested", "v1", platforms)

	multiArchDigestRef := fmt.Sprintf("%s/fixtures/multi@%s", upstream, multiArchDigest)
	nestedDigestRef := fmt.Sprintf("%s/fixtures/nested@%s", upstream, nestedDigest)
	multiArchTagRef := fmt.Sprintf("%s/fixtures/multi:v1", upstream)

	testCases := []struct {
		name    string
		pullRef string
		// retagAs, when non-empty, swaps the source layout's ref annotation from pullRef to this
		// value so Unpack sees a tag-style ref over an existing multi-arch index.
		retagAs     string
		unpackRef   string
		expectIndex bool
	}{
		{
			name:        "multi-arch index by digest preserves index",
			pullRef:     multiArchDigestRef,
			unpackRef:   multiArchDigestRef,
			expectIndex: true,
		},
		{
			name:        "nested index by digest preserves nested structure",
			pullRef:     nestedDigestRef,
			unpackRef:   nestedDigestRef,
			expectIndex: true,
		},
		{
			name:      "multi-arch index by tag filters to platform",
			pullRef:   multiArchDigestRef,
			retagAs:   multiArchTagRef,
			unpackRef: multiArchTagRef,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pullRefInfo, err := transform.ParseImageRef(tc.pullRef)
			require.NoError(t, err)

			layoutDir := t.TempDir()
			_, err = Pull(ctx, []transform.Image{pullRefInfo}, layoutDir, PullOptions{
				Arch:           "amd64",
				CacheDirectory: t.TempDir(),
				PlainHTTP:      true,
			})
			require.NoError(t, err)

			if tc.retagAs != "" {
				store, err := oci.NewWithContext(ctx, layoutDir)
				require.NoError(t, err)
				desc, err := store.Resolve(ctx, tc.pullRef)
				require.NoError(t, err)
				require.NoError(t, store.Untag(ctx, tc.pullRef))
				require.NoError(t, store.Tag(ctx, desc, tc.retagAs))
			}

			tarFile := filepath.Join(t.TempDir(), "images.tar")
			require.NoError(t, archive.Compress(ctx, []string{layoutDir}, tarFile, archive.CompressOpts{}))

			dstDir := t.TempDir()
			unpacked, err := Unpack(ctx, v1alpha1.ImageArchive{
				Path:   tarFile,
				Images: []string{tc.unpackRef},
			}, dstDir, "amd64")
			require.NoError(t, err)
			require.Len(t, unpacked, 1)
			require.Equal(t, tc.unpackRef, unpacked[0].Image.Reference)

			dstIdx, err := getIndexFromOCILayout(dstDir)
			require.NoError(t, err)
			var top *ocispec.Descriptor
			for i := range dstIdx.Manifests {
				if dstIdx.Manifests[i].Annotations[ocispec.AnnotationRefName] == tc.unpackRef {
					top = &dstIdx.Manifests[i]
					break
				}
			}
			require.NotNil(t, top, "no manifest tagged with ref %s in %v", tc.unpackRef, dstIdx.Manifests)

			if tc.expectIndex {
				require.True(t, IsIndex(top.MediaType), "expected preserved index, got %s", top.MediaType)
				preserved := requireIndexBlobs(t, dstDir, top.Digest.String())
				require.NotEmpty(t, preserved.Manifests)
				return
			}

			require.True(t, IsManifest(top.MediaType), "expected platform-filtered manifest, got %s", top.MediaType)
			manifest := requireManifestBlobs(t, dstDir, top.Digest.String())
			cfgBytes, err := os.ReadFile(filepath.Join(dstDir, "blobs", "sha256", manifest.Config.Digest.Hex()))
			require.NoError(t, err)
			var cfg ocispec.Image
			require.NoError(t, json.Unmarshal(cfgBytes, &cfg))
			require.Equal(t, "amd64", cfg.Architecture)
		})
	}
}
