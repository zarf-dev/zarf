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
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2"
	orasRemote "oras.land/oras-go/v2/registry/remote"
)

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

			// Make sure all the layers of the image are pulled in
			for _, manifest := range idx.Manifests {
				manifestPath := filepath.Join(destDir, "blobs", "sha256", manifest.Digest.Hex())
				mb, err := os.ReadFile(manifestPath)
				require.NoError(t, err)
				var m ocispec.Manifest
				require.NoError(t, json.Unmarshal(mb, &m))
				for _, layer := range m.Layers {
					require.FileExists(t, filepath.Join(destDir, fmt.Sprintf("blobs/sha256/%s", layer.Digest.Hex())))
					require.FileExists(t, filepath.Join(cacheDir, fmt.Sprintf("blobs/sha256/%s", layer.Digest.Hex())))
				}
			}
		})
	}
}

func TestPullMultiArchIndex(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	// Index digest for ghcr.io/zarf-dev/zarf/agent:v0.32.6 (multi-platform manifest list).
	ref, err := transform.ParseImageRef("ghcr.io/zarf-dev/zarf/agent:v0.32.6@sha256:05a82656df5466ce17c3e364c16792ae21ce68438bfe06eeab309d0520c16b48")
	require.NoError(t, err)

	destDir := t.TempDir()
	cacheDir := t.TempDir()
	opts := PullOptions{
		Arch:           v1alpha1.MultiArch,
		CacheDirectory: cacheDir,
	}
	_, err = Pull(ctx, []transform.Image{ref}, destDir, opts)
	require.NoError(t, err)

	// The top-level index.json lists every manifest oras.Copy walked; the root is the one
	// tagged with our image reference.
	idx, err := getIndexFromOCILayout(destDir)
	require.NoError(t, err)
	var rootDigest string
	for _, m := range idx.Manifests {
		if m.Annotations[ocispec.AnnotationRefName] == ref.Reference {
			rootDigest = m.Digest.String()
			require.Equal(t, ocispec.MediaTypeImageIndex, m.MediaType)
			break
		}
	}
	require.Equal(t, ref.Digest, rootDigest, "root index digest must match requested digest")

	// The pulled blob at that digest should itself be an OCI index with >1 platform manifest.
	digestHex := rootDigest[len("sha256:"):]
	blobPath := filepath.Join(destDir, "blobs", "sha256", digestHex)
	require.FileExists(t, blobPath)
	b, err := os.ReadFile(blobPath)
	require.NoError(t, err)
	var pulledIdx ocispec.Index
	require.NoError(t, json.Unmarshal(b, &pulledIdx))
	require.Greater(t, len(pulledIdx.Manifests), 1, "expected multiple platform manifests in pulled index")

	// Every referenced manifest blob must be present locally (full graph was copied).
	for _, m := range pulledIdx.Manifests {
		require.FileExists(t, filepath.Join(destDir, "blobs", "sha256", m.Digest.Hex()))
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
	require.True(t, pulled[0].IsContainerImage, "single-arch container image must be flagged as container")

	manifestBlob := filepath.Join(destDir, "blobs", "sha256", digest[len("sha256:"):])
	require.FileExists(t, manifestBlob)
	mb, err := os.ReadFile(manifestBlob)
	require.NoError(t, err)
	var m ocispec.Manifest
	require.NoError(t, json.Unmarshal(mb, &m))
	require.FileExists(t, filepath.Join(destDir, "blobs", "sha256", m.Config.Digest.Hex()))
	for _, layer := range m.Layers {
		require.FileExists(t, filepath.Join(destDir, "blobs", "sha256", layer.Digest.Hex()))
	}
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
	require.True(t, pulled[0].IsContainerImage, "multi-arch index of container images must be flagged as container")

	idxBlob := filepath.Join(destDir, "blobs", "sha256", digest[len("sha256:"):])
	require.FileExists(t, idxBlob)
	ib, err := os.ReadFile(idxBlob)
	require.NoError(t, err)
	var idx ocispec.Index
	require.NoError(t, json.Unmarshal(ib, &idx))
	require.Len(t, idx.Manifests, len(platforms))
	for _, child := range idx.Manifests {
		manifestPath := filepath.Join(destDir, "blobs", "sha256", child.Digest.Hex())
		require.FileExists(t, manifestPath)
		mb, err := os.ReadFile(manifestPath)
		require.NoError(t, err)
		var m ocispec.Manifest
		require.NoError(t, json.Unmarshal(mb, &m))
		require.FileExists(t, filepath.Join(destDir, "blobs", "sha256", m.Config.Digest.Hex()))
		for _, layer := range m.Layers {
			require.FileExists(t, filepath.Join(destDir, "blobs", "sha256", layer.Digest.Hex()))
		}
	}
}

// TestPullNestedIndex verifies Pull of an index whose only child is itself a multi-arch index.
// This exercises the recursive paths in both indexIsContainerImage (flag is true even though
// the direct children are indexes, not manifests) and oras.Copy (all nested leaf blobs are pulled).
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
	require.True(t, pulled[0].IsContainerImage, "nested index over container images must be flagged as container")

	outerBlob := filepath.Join(destDir, "blobs", "sha256", digest[len("sha256:"):])
	require.FileExists(t, outerBlob)
	ob, err := os.ReadFile(outerBlob)
	require.NoError(t, err)
	var outerIdx ocispec.Index
	require.NoError(t, json.Unmarshal(ob, &outerIdx))
	require.Len(t, outerIdx.Manifests, 1, "outer index wraps a single inner index")

	innerDesc := outerIdx.Manifests[0]
	innerBlob := filepath.Join(destDir, "blobs", "sha256", innerDesc.Digest.Hex())
	require.FileExists(t, innerBlob)
	ib, err := os.ReadFile(innerBlob)
	require.NoError(t, err)
	var innerIdx ocispec.Index
	require.NoError(t, json.Unmarshal(ib, &innerIdx))
	require.Len(t, innerIdx.Manifests, platforms)
	for _, child := range innerIdx.Manifests {
		manifestPath := filepath.Join(destDir, "blobs", "sha256", child.Digest.Hex())
		require.FileExists(t, manifestPath)
		mb, err := os.ReadFile(manifestPath)
		require.NoError(t, err)
		var m ocispec.Manifest
		require.NoError(t, json.Unmarshal(mb, &m))
		require.FileExists(t, filepath.Join(destDir, "blobs", "sha256", m.Config.Digest.Hex()))
		for _, layer := range m.Layers {
			require.FileExists(t, filepath.Join(destDir, "blobs", "sha256", layer.Digest.Hex()))
		}
	}
}

// TestPullNonContainerImage pushes a single manifest whose only layer has a non-image media type
// (mimicking a Helm chart or similar OCI artifact). IsContainerImage must be false so assemble.go
// does not hand it to syft for SBOM scanning.
func TestPullNonContainerImage(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	upstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	repoRef := upstream + "/fixtures/artifact"
	repo := testutil.NewRepo(t, repoRef)

	helmLayer := testutil.PushBlob(ctx, t, repo, "application/vnd.cncf.helm.chart.content.v1.tar+gzip", testutil.RandomBytes(t, 64))
	config := testutil.PushBlob(ctx, t, repo, ocispec.MediaTypeImageConfig, []byte(`{}`))
	manifestDesc := testutil.PushManifest(ctx, t, repo, config, []ocispec.Descriptor{helmLayer})
	require.NoError(t, repo.Tag(ctx, manifestDesc, "test"))

	ref, err := transform.ParseImageRef(fmt.Sprintf("%s:test@%s", repoRef, manifestDesc.Digest.String()))
	require.NoError(t, err)

	destDir := t.TempDir()
	pulled, err := Pull(ctx, []transform.Image{ref}, destDir, PullOptions{
		Arch:           "amd64",
		CacheDirectory: t.TempDir(),
		PlainHTTP:      true,
	})
	require.NoError(t, err)
	require.Len(t, pulled, 1)
	require.False(t, pulled[0].IsContainerImage, "Helm-chart-style artifact must not be flagged as a container image")
}

// TestPullIndexNonContainerChildren covers an index that references only non-container artifacts
func TestPullIndexNonContainerChildren(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	upstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	repoRef := upstream + "/fixtures/artifact-index"
	repo := testutil.NewRepo(t, repoRef)

	pushHelmManifest := func(arch string) ocispec.Descriptor {
		layer := testutil.PushBlob(ctx, t, repo, "application/vnd.cncf.helm.chart.content.v1.tar+gzip", testutil.RandomBytes(t, 64))
		config := testutil.PushBlob(ctx, t, repo, ocispec.MediaTypeImageConfig, fmt.Appendf(nil, `{"architecture":%q}`, arch))
		desc := testutil.PushManifest(ctx, t, repo, config, []ocispec.Descriptor{layer})
		desc.Platform = &ocispec.Platform{OS: "linux", Architecture: arch}
		return desc
	}
	// real charts wouldn't have architecture, adding for the sake of tests
	children := []ocispec.Descriptor{pushHelmManifest("amd64"), pushHelmManifest("arm64")}
	idxDesc := testutil.PushIndex(ctx, t, repo, children)
	require.NoError(t, repo.Tag(ctx, idxDesc, "test"))

	ref, err := transform.ParseImageRef(fmt.Sprintf("%s:test@%s", repoRef, idxDesc.Digest.String()))
	require.NoError(t, err)

	destDir := t.TempDir()
	pulled, err := Pull(ctx, []transform.Image{ref}, destDir, PullOptions{
		Arch:           v1alpha1.MultiArch,
		CacheDirectory: t.TempDir(),
		PlainHTTP:      true,
	})
	require.NoError(t, err)
	require.Len(t, pulled, 1)
	require.False(t, pulled[0].IsContainerImage, "index of non-container artifacts must not be flagged as container")
}

func TestIndexIsContainerImageRecursive(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	upstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	repoRef := upstream + "/fixtures/recursive"
	repo := testutil.NewRepo(t, repoRef)

	imageManifest := testutil.PushSinglePlatformImage(ctx, t, repo, "amd64")

	helmLayer := testutil.PushBlob(ctx, t, repo, "application/vnd.cncf.helm.chart.content.v1.tar+gzip", testutil.RandomBytes(t, 64))
	helmConfig := testutil.PushBlob(ctx, t, repo, ocispec.MediaTypeImageConfig, []byte(`{}`))
	helmManifest := testutil.PushManifest(ctx, t, repo, helmConfig, []ocispec.Descriptor{helmLayer})

	flatContainer := ocispec.Index{Manifests: []ocispec.Descriptor{imageManifest}}
	flatHelm := ocispec.Index{Manifests: []ocispec.Descriptor{helmManifest}}

	innerContainerDesc := testutil.PushIndex(ctx, t, repo, []ocispec.Descriptor{imageManifest})
	nested := ocispec.Index{Manifests: []ocispec.Descriptor{innerContainerDesc}}

	ok, err := indexIsContainerImage(ctx, repo, repoRef, flatContainer)
	require.NoError(t, err)
	require.True(t, ok, "index with a container image child must be flagged")

	ok, err = indexIsContainerImage(ctx, repo, repoRef, flatHelm)
	require.NoError(t, err)
	require.False(t, ok, "index with only helm chart children must not be flagged")

	ok, err = indexIsContainerImage(ctx, repo, repoRef, nested)
	require.NoError(t, err)
	require.True(t, ok, "outer index wrapping a container-image inner index must be flagged (recursion)")
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
