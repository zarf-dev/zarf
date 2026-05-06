// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2/registry/remote"
)

// pushDockerManifestList pushes a Docker-mediaType manifest list to exercise isIndex's docker path.
func pushDockerManifestList(ctx context.Context, t *testing.T, repo *remote.Repository, children []ocispec.Descriptor) ocispec.Descriptor {
	t.Helper()
	list := struct {
		specs.Versioned
		MediaType string               `json:"mediaType"`
		Manifests []ocispec.Descriptor `json:"manifests"`
	}{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: DockerMediaTypeManifestList,
		Manifests: children,
	}
	body, err := json.Marshal(list)
	require.NoError(t, err)
	desc := ocispec.Descriptor{
		MediaType: DockerMediaTypeManifestList,
		Digest:    digest.FromBytes(body),
		Size:      int64(len(body)),
	}
	require.NoError(t, repo.Push(ctx, desc, bytes.NewReader(body)))
	return desc
}

func TestCheckForIndex(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	upstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)

	platforms := []ocispec.Platform{
		{OS: "linux", Architecture: "amd64"},
		{OS: "linux", Architecture: "arm64"},
	}

	ociRepo := testutil.NewRepo(t, upstream+"/fixtures/idx")
	ociChildren := make([]ocispec.Descriptor, 0, len(platforms))
	for _, p := range platforms {
		desc := testutil.PushSinglePlatformImage(ctx, t, ociRepo, p.Architecture)
		desc.Platform = &p
		ociChildren = append(ociChildren, desc)
	}
	ociIdx := testutil.PushIndex(ctx, t, ociRepo, ociChildren)
	require.NoError(t, ociRepo.Tag(ctx, ociIdx, "v1"))

	dockerRepo := testutil.NewRepo(t, upstream+"/fixtures/docker-list")
	dockerChildren := make([]ocispec.Descriptor, 0, len(platforms))
	for _, p := range platforms {
		desc := testutil.PushSinglePlatformImage(ctx, t, dockerRepo, p.Architecture)
		desc.Platform = &p
		dockerChildren = append(dockerChildren, desc)
	}
	dockerList := pushDockerManifestList(ctx, t, dockerRepo, dockerChildren)
	require.NoError(t, dockerRepo.Tag(ctx, dockerList, "v1"))

	manifestDigest := testutil.PushImage(ctx, t, upstream+"/fixtures/img", "v1")

	testCases := []struct {
		name            string
		ref             string
		expectedDigests []string
		expectedErr     string
	}{
		{
			name:        "oci index sha",
			ref:         fmt.Sprintf("%s/fixtures/idx@%s", upstream, ociIdx.Digest),
			expectedErr: "%s resolved to an OCI image index which is not supported by Zarf, select a specific platform to use",
			expectedDigests: []string{
				ociChildren[0].Digest.String(),
				ociChildren[1].Digest.String(),
			},
		},
		{
			name:        "docker manifest list",
			ref:         fmt.Sprintf("%s/fixtures/docker-list@%s", upstream, dockerList.Digest),
			expectedErr: "%s resolved to an OCI image index which is not supported by Zarf, select a specific platform to use",
			expectedDigests: []string{
				dockerChildren[0].Digest.String(),
				dockerChildren[1].Digest.String(),
			},
		},
		{
			name: "image manifest by tag",
			ref:  fmt.Sprintf("%s/fixtures/img:v1", upstream),
		},
		{
			name: "image manifest by digest",
			ref:  fmt.Sprintf("%s/fixtures/img@%s", upstream, manifestDigest),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			refInfo, err := transform.ParseImageRef(tc.ref)
			require.NoError(t, err)

			cacheDir := t.TempDir()
			dstDir := t.TempDir()
			opts := PullOptions{
				Arch:           "amd64",
				CacheDirectory: cacheDir,
				PlainHTTP:      true,
			}
			_, err = Pull(ctx, []transform.Image{refInfo}, dstDir, opts)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, fmt.Sprintf(tc.expectedErr, refInfo.Reference))
				for _, d := range tc.expectedDigests {
					require.ErrorContains(t, err, d)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestPull(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	upstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)

	testutil.PushImage(ctx, t, upstream+"/fixtures/container", "0.0.1")
	testutil.PushImage(ctx, t, upstream+"/fixtures/sig", "v1.sig")
	testutil.PushImage(ctx, t, upstream+"/fixtures/helm", "6.4.0")
	shaDigest := testutil.PushImage(ctx, t, upstream+"/fixtures/sha-pinned", "ignored")

	overrideUpstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	testutil.PushImage(ctx, t, overrideUpstream+"/library/podinfo", "6.4.0")

	testCases := []struct {
		name              string
		refs              []string
		registryOverrides []RegistryOverride
		expectErr         bool
	}{
		{
			name: "pull a container image, a cosign-style signature, a chart-style image, and a sha'd image",
			refs: []string{
				fmt.Sprintf("%s/fixtures/container:0.0.1", upstream),
				fmt.Sprintf("%s/fixtures/sig:v1.sig", upstream),
				fmt.Sprintf("%s/fixtures/helm:6.4.0", upstream),
				fmt.Sprintf("%s/fixtures/sha-pinned@%s", upstream, shaDigest),
			},
		},
		{
			name: "error when pulling an image that doesn't exist",
			refs: []string{
				fmt.Sprintf("%s/fixtures/missing:does-not-exist", upstream),
			},
			expectErr: true,
		},
		{
			name: "test registry overrides",
			refs: []string{
				"fake.example/library/podinfo:6.4.0",
			},
			registryOverrides: []RegistryOverride{
				{
					Source:   "fake.example",
					Override: overrideUpstream,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
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
				RegistryOverrides: tc.registryOverrides,
				Arch:              "amd64",
				PlainHTTP:         true,
			}

			imageManifests, err := Pull(ctx, images, destDir, opts)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

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

			for _, imageWithManifest := range imageManifests {
				for _, layer := range imageWithManifest.Manifest.Layers {
					require.FileExists(t, filepath.Join(destDir, fmt.Sprintf("blobs/sha256/%s", layer.Digest.Hex())))
					require.FileExists(t, filepath.Join(cacheDir, fmt.Sprintf("blobs/sha256/%s", layer.Digest.Hex())))
				}
			}
		})
	}
}

func TestPullInvalidCache(t *testing.T) {
	// pulling an image with an invalid layer in the cache should still pull the image
	t.Parallel()
	ctx := testutil.TestContext(t)
	upstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)

	repo := testutil.NewRepo(t, upstream+"/fixtures/cache")
	layer := testutil.PushBlob(ctx, t, repo, ocispec.MediaTypeImageLayer, testutil.RandomBytes(t, 128))
	config := testutil.PushBlob(ctx, t, repo, ocispec.MediaTypeImageConfig, []byte(`{"architecture":"amd64"}`))
	manifest := testutil.PushManifest(ctx, t, repo, config, []ocispec.Descriptor{layer})
	require.NoError(t, repo.Tag(ctx, manifest, "v1"))

	ref, err := transform.ParseImageRef(fmt.Sprintf("%s/fixtures/cache@%s", upstream, manifest.Digest))
	require.NoError(t, err)

	destDir := t.TempDir()
	cacheDir := t.TempDir()
	require.NoError(t, os.MkdirAll(cacheDir, 0o777))

	correctLayerSha := layer.Digest.Hex()
	invalidLayerPath := filepath.Join(cacheDir, fmt.Sprintf("sha256:%s", correctLayerSha))
	require.NoError(t, os.WriteFile(invalidLayerPath, []byte("this mimics a corrupted file"), 0o777))

	_, err = Pull(ctx, []transform.Image{ref}, destDir, PullOptions{
		CacheDirectory: cacheDir,
		Arch:           "amd64",
		PlainHTTP:      true,
	})
	require.NoError(t, err)

	pulledLayerPath := filepath.Join(destDir, "blobs", "sha256", correctLayerSha)
	pulledLayer, err := os.ReadFile(pulledLayerPath)
	require.NoError(t, err)
	pulledLayerSha := sha256.Sum256(pulledLayer)
	require.Equal(t, correctLayerSha, fmt.Sprintf("%x", pulledLayerSha))
}
