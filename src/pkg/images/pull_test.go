// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2"
	orasRemote "oras.land/oras-go/v2/registry/remote"
)

// requireManifestBlobs asserts the manifest blob at digest is on disk in destDir along with its
// config and every layer. Returns the parsed manifest so callers can make test-specific assertions.
func requireManifestBlobs(t *testing.T, destDir, digest string) ocispec.Manifest {
	t.Helper()
	path := filepath.Join(destDir, "blobs", "sha256", strings.TrimPrefix(digest, "sha256:"))
	require.FileExists(t, path)
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	var m ocispec.Manifest
	require.NoError(t, json.Unmarshal(b, &m))
	require.FileExists(t, filepath.Join(destDir, "blobs", "sha256", m.Config.Digest.Hex()))
	for _, layer := range m.Layers {
		require.FileExists(t, filepath.Join(destDir, "blobs", "sha256", layer.Digest.Hex()))
	}
	return m
}

// requireIndexBlobs asserts the index blob at digest is on disk and every descendant (nested
// indexes + leaf manifests with their config/layers) is too. Returns the parsed top-level index.
func requireIndexBlobs(t *testing.T, destDir, digest string) ocispec.Index {
	t.Helper()
	path := filepath.Join(destDir, "blobs", "sha256", strings.TrimPrefix(digest, "sha256:"))
	require.FileExists(t, path)
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	var idx ocispec.Index
	require.NoError(t, json.Unmarshal(b, &idx))
	for _, child := range idx.Manifests {
		if IsIndex(string(child.MediaType)) {
			requireIndexBlobs(t, destDir, child.Digest.String())
			continue
		}
		requireManifestBlobs(t, destDir, child.Digest.String())
	}
	return idx
}

func TestCheckForIndex(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		ref         string
		file        string
		arch        string
		expectedErr string
	}{
		{
			name:        "index sha",
			ref:         "ghcr.io/zarf-dev/zarf/agent:v0.32.6@sha256:05a82656df5466ce17c3e364c16792ae21ce68438bfe06eeab309d0520c16b48",
			file:        "agent-index.json",
			arch:        "arm64",
			expectedErr: "%s resolved to an OCI image index. Either set metadata.architecture to \"multi\" to build a multi-arch package that preserves the full index, or pin the image to a platform-specific digest",
		},
		{
			name:        "docker manifest list",
			ref:         "defenseunicorns/zarf-game@sha256:0b694ca1c33afae97b7471488e07968599f1d2470c629f76af67145ca64428af",
			file:        "game-index.json",
			arch:        "arm64",
			expectedErr: "%s resolved to an OCI image index. Either set metadata.architecture to \"multi\" to build a multi-arch package that preserves the full index, or pin the image to a platform-specific digest",
		},
		{
			name:        "image manifest",
			ref:         "ghcr.io/zarf-dev/zarf/agent:v0.32.6",
			file:        "agent-manifest.json",
			arch:        "arm64",
			expectedErr: "",
		},
		{
			name:        "image manifest sha'd",
			ref:         "ghcr.io/zarf-dev/zarf/agent:v0.32.6@sha256:b3fabdc7d4ecd0f396016ef78da19002c39e3ace352ea0ae4baa2ce9d5958376",
			file:        "agent-manifest.json",
			arch:        "arm64",
			expectedErr: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)
			refInfo, err := transform.ParseImageRef(tc.ref)
			require.NoError(t, err)
			repo, err := orasRemote.NewRepository(refInfo.Reference)
			require.NoError(t, err)
			_, b, err := oras.FetchBytes(ctx, repo, refInfo.Reference, oras.DefaultFetchBytesOptions)
			require.NoError(t, err)
			var idx ocispec.Index
			err = json.Unmarshal(b, &idx)
			require.NoError(t, err)
			cacheDir := t.TempDir()
			dstDir := t.TempDir()
			opts := PullOptions{
				Arch:           tc.arch,
				CacheDirectory: cacheDir,
			}
			_, err = Pull(ctx, []transform.Image{refInfo}, dstDir, opts)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, fmt.Sprintf(tc.expectedErr, refInfo.Reference))
				// Ensure the error message contains the digest of the manifests the user can use
				for _, manifest := range idx.Manifests {
					require.ErrorContains(t, err, manifest.Digest.String())
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestPull(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name              string
		refs              []string
		RegistryOverrides []RegistryOverride
		arch              string
		expectErr         bool
	}{
		{
			name: "pull a container image, a cosign signature, a Helm chart, and a sha'd container image",
			refs: []string{
				"ghcr.io/zarf-dev/doom-game:0.0.1",
				"ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig",
				"ghcr.io/stefanprodan/manifests/podinfo:6.4.0",
				"ghcr.io/fluxcd/image-automation-controller@sha256:48a89734dc82c3a2d4138554b3ad4acf93230f770b3a582f7f48be38436d031c",
			},
			arch: "amd64",
		},
		{
			name: "error when pulling an image that doesn't exist",
			refs: []string{
				"ghcr.io/zarf-dev/zarf/imagethatdoesntexist:v1.1.1",
			},
			expectErr: true,
		},
		{
			name: "test registry overrides",
			refs: []string{
				"stefanprodan/podinfo:6.4.0",
			},
			arch: "amd64",
			RegistryOverrides: []RegistryOverride{
				{
					Source:   "docker.io",
					Override: "ghcr.io",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)
			var images []transform.Image
			for _, ref := range tc.refs {
				image, err := transform.ParseImageRef(ref)
				require.NoError(t, err)
				images = append(images, image)
			}

			destDir := t.TempDir()
			cacheDir := t.TempDir()
			opts := PullOptions{
				CacheDirectory:    cacheDir,
				RegistryOverrides: tc.RegistryOverrides,
				Arch:              tc.arch,
			}

			pulled, err := Pull(ctx, images, destDir, opts)
			if tc.expectErr {
				require.Error(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, pulled, len(images))

			idx, err := getIndexFromOCILayout(filepath.Join(destDir))
			require.NoError(t, err)
			var expectedImageAnnotations []map[string]string
			for _, ref := range images {
				expectedAnnotations := map[string]string{
					ocispec.AnnotationRefName:       ref.Reference,
					ocispec.AnnotationBaseImageName: ref.Reference,
				}
				expectedImageAnnotations = append(expectedImageAnnotations, expectedAnnotations)
			}
			var actualImageAnnotations []map[string]string
			for _, manifest := range idx.Manifests {
				actualImageAnnotations = append(actualImageAnnotations, manifest.Annotations)
			}
			require.ElementsMatch(t, expectedImageAnnotations, actualImageAnnotations)

			// Make sure all the layers of the image are pulled in (including the shared cache).
			for _, manifestDesc := range idx.Manifests {
				m := requireManifestBlobs(t, destDir, manifestDesc.Digest.String())
				for _, layer := range m.Layers {
					require.FileExists(t, filepath.Join(cacheDir, fmt.Sprintf("blobs/sha256/%s", layer.Digest.Hex())))
				}
			}
		})
	}
}

func TestPullSingleArchContainerImage(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	upstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	digest := testutil.PushImage(ctx, t, upstream+"/fixtures/single", "test")
	ref, err := transform.ParseImageRef(fmt.Sprintf("%s/fixtures/single:test@%s", upstream, digest))
	require.NoError(t, err)

	destDir := t.TempDir()
	pulled, err := Pull(ctx, []transform.Image{ref}, destDir, PullOptions{
		Arch:           "amd64",
		CacheDirectory: t.TempDir(),
		PlainHTTP:      true,
	})
	require.NoError(t, err)
	require.Len(t, pulled, 1)

	requireManifestBlobs(t, destDir, digest)
}

func TestPullMultiArchContainerImage(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	upstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	platforms := []ocispec.Platform{
		{OS: "linux", Architecture: "amd64"},
		{OS: "linux", Architecture: "arm64"},
	}
	digest := testutil.PushMultiArchIndex(ctx, t, upstream+"/fixtures/multi", "test", platforms)
	ref, err := transform.ParseImageRef(fmt.Sprintf("%s/fixtures/multi:test@%s", upstream, digest))
	require.NoError(t, err)

	destDir := t.TempDir()
	pulled, err := Pull(ctx, []transform.Image{ref}, destDir, PullOptions{
		Arch:           v1alpha1.MultiArch,
		CacheDirectory: t.TempDir(),
		PlainHTTP:      true,
	})
	require.NoError(t, err)
	require.Len(t, pulled, 1)

	idx := requireIndexBlobs(t, destDir, digest)
	require.Len(t, idx.Manifests, len(platforms))
}

func TestPullNestedIndex(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	upstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	const platforms = 2
	digest := testutil.PushNestedIndex(ctx, t, upstream+"/fixtures/nested", "test", platforms)
	ref, err := transform.ParseImageRef(fmt.Sprintf("%s/fixtures/nested:test@%s", upstream, digest))
	require.NoError(t, err)

	destDir := t.TempDir()
	pulled, err := Pull(ctx, []transform.Image{ref}, destDir, PullOptions{
		Arch:           v1alpha1.MultiArch,
		CacheDirectory: t.TempDir(),
		PlainHTTP:      true,
	})
	require.NoError(t, err)
	require.Len(t, pulled, 1)

	outerIdx := requireIndexBlobs(t, destDir, digest)
	require.Len(t, outerIdx.Manifests, 1, "outer index wraps a single inner index")
	innerIdx := requireIndexBlobs(t, destDir, outerIdx.Manifests[0].Digest.String())
	require.Len(t, innerIdx.Manifests, platforms)
}

func TestGetSizeOfIndexRecursive(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	upstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	repoRef := upstream + "/fixtures/size"
	repo := testutil.NewRepo(t, repoRef)

	amd64 := testutil.PushSinglePlatformImage(ctx, t, repo, "amd64")
	amd64.Platform = &ocispec.Platform{OS: "linux", Architecture: "amd64"}
	arm64 := testutil.PushSinglePlatformImage(ctx, t, repo, "arm64")
	arm64.Platform = &ocispec.Platform{OS: "linux", Architecture: "arm64"}
	innerDesc := testutil.PushIndex(ctx, t, repo, []ocispec.Descriptor{amd64, arm64})
	outerDesc := testutil.PushIndex(ctx, t, repo, []ocispec.Descriptor{innerDesc})

	_, outerBytes, err := oras.FetchBytes(ctx, repo, outerDesc.Digest.String(), oras.DefaultFetchBytesOptions)
	require.NoError(t, err)

	size, err := getSizeOfIndex(ctx, repo, outerDesc, outerBytes)
	require.NoError(t, err)

	expected := outerDesc.Size + innerDesc.Size
	for _, leafDesc := range []ocispec.Descriptor{amd64, arm64} {
		_, mb, err := oras.FetchBytes(ctx, repo, leafDesc.Digest.String(), oras.DefaultFetchBytesOptions)
		require.NoError(t, err)
		var m ocispec.Manifest
		require.NoError(t, json.Unmarshal(mb, &m))
		expected += leafDesc.Size + m.Config.Size
		for _, layer := range m.Layers {
			expected += layer.Size
		}
	}
	require.Equal(t, expected, size, "getSizeOfIndex must recurse and sum every nested leaf blob")
}

func TestPullMultiArchRejectsDaemonFallback(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	upstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	missingRef := fmt.Sprintf("%s/fixtures/missing:latest", upstream)
	ref, err := transform.ParseImageRef(missingRef)
	require.NoError(t, err)
	_, err = Pull(ctx, []transform.Image{ref}, t.TempDir(), PullOptions{
		Arch:           v1alpha1.MultiArch,
		CacheDirectory: t.TempDir(),
	})
	require.ErrorContains(t, err, "multi-arch packages cannot fall back to the docker daemon")
	require.ErrorContains(t, err, missingRef)
}

func TestPullInvalidCache(t *testing.T) {
	// pulling an image with an invalid layer in the cache should still pull the image
	t.Parallel()
	ctx := testutil.TestContext(t)
	ref, err := transform.ParseImageRef("ghcr.io/fluxcd/image-automation-controller@sha256:48a89734dc82c3a2d4138554b3ad4acf93230f770b3a582f7f48be38436d031c")
	require.NoError(t, err)
	destDir := t.TempDir()
	cacheDir := t.TempDir()
	require.NoError(t, os.MkdirAll(cacheDir, 0777))
	invalidContent := []byte("this mimics a corrupted file")
	// This is the sha of a layer of the image.
	// we intentionally put junk data into the cache with this layer to test that it will get cleaned up.
	correctLayerSha := "d94c8059c3cffb9278601bf9f8be070d50c84796401a4c5106eb8a4042445bbc"
	invalidLayerPath := filepath.Join(cacheDir, fmt.Sprintf("sha256:%s", correctLayerSha))
	err = os.WriteFile(invalidLayerPath, invalidContent, 0777)
	require.NoError(t, err)

	opts := PullOptions{
		CacheDirectory: cacheDir,
	}
	_, err = Pull(ctx, []transform.Image{ref}, destDir, opts)
	require.NoError(t, err)

	pulledLayerPath := filepath.Join(destDir, "blobs", "sha256", correctLayerSha)
	pulledLayer, err := os.ReadFile(pulledLayerPath)
	require.NoError(t, err)
	pulledLayerSha := sha256.Sum256(pulledLayer)
	require.Equal(t, correctLayerSha, fmt.Sprintf("%x", pulledLayerSha))
}
