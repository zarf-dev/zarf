// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/transform"
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
			expectedErr: "%s resolved to an OCI image index which is not supported by Zarf, select a specific platform to use",
		},
		{
			name:        "docker manifest list",
			ref:         "defenseunicorns/zarf-game@sha256:0b694ca1c33afae97b7471488e07968599f1d2470c629f76af67145ca64428af",
			file:        "game-index.json",
			arch:        "arm64",
			expectedErr: "%s resolved to an OCI image index which is not supported by Zarf, select a specific platform to use",
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
			ctx := context.Background()
			refInfo, err := transform.ParseImageRef(tc.ref)
			require.NoError(t, err)
			repo, err := orasRemote.NewRepository(refInfo.Reference)
			require.NoError(t, err)
			_, b, err := oras.FetchBytes(ctx, repo, refInfo.Reference, oras.DefaultFetchBytesOptions)
			require.NoError(t, err)
			var idx ocispec.Index
			err = json.Unmarshal(b, &idx)
			require.NoError(t, err)
			tmp := t.TempDir()
			cfg := PullConfig{
				Arch:                 tc.arch,
				DestinationDirectory: tmp,
				ImageList:            []transform.Image{refInfo},
				CacheDirectory:       tmp,
			}
			_, err = Pull(ctx, cfg)
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
		name      string
		ref       string
		arch      string
		expectErr bool
	}{
		{
			name: "pull an image",
			ref:  "ghcr.io/zarf-dev/zarf/agent:v0.32.6@sha256:b3fabdc7d4ecd0f396016ef78da19002c39e3ace352ea0ae4baa2ce9d5958376",
			arch: "arm64",
		},
		{
			name:      "error when pulling an image that doesn't exist",
			ref:       "ghcr.io/zarf-dev/zarf/imagethatdoesntexist:v1.1.1",
			expectErr: true,
		},
		{
			name: "pull an image signature",
			ref:  "ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig",
			arch: "doesnt-matter",
		},
		{
			name: "pull a Helm OCI object",
			ref:  "ghcr.io/stefanprodan/manifests/podinfo:6.4.0",
			arch: "doesnt-matter",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ref, err := transform.ParseImageRef(tc.ref)
			require.NoError(t, err)
			destDir := t.TempDir()
			cacheDir := t.TempDir()
			pullConfig := PullConfig{
				DestinationDirectory: destDir,
				CacheDirectory:       cacheDir,
				Arch:                 tc.arch,
				ImageList: []transform.Image{
					ref,
				},
			}

			imageManifests, err := Pull(context.Background(), pullConfig)
			if tc.expectErr {
				require.Error(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)

			// Make sure all the layers of the image are pulled in
			for _, manifest := range imageManifests {
				for _, layer := range manifest.Layers {
					require.FileExists(t, filepath.Join(destDir, fmt.Sprintf("blobs/sha256/%s", layer.Digest.Hex())))
					require.FileExists(t, filepath.Join(cacheDir, fmt.Sprintf("blobs/sha256/%s", layer.Digest.Hex())))
				}
			}
		})
	}
}

func TestPullRegistryOverrides(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name      string
		ref       string
		arch      string
		expectErr bool
	}{
		{
			name: "pull an image",
			ref:  "ghcr.io/stefanprodan/podinfo:6.4.0",
			arch: "amd64",
		},
		{
			name:      "error when pulling an image that doesn't exist",
			ref:       "ghcr.io/zarf-dev/zarf/imagethatdoesntexist:v1.1.1",
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ref, err := transform.ParseImageRef(tc.ref)
			require.NoError(t, err)
			destDir := t.TempDir()
			cacheDir := t.TempDir()
			pullConfig := PullConfig{
				DestinationDirectory: destDir,
				CacheDirectory:       cacheDir,
				Arch:                 tc.arch,
				RegistryOverrides: map[string]string{
					"ghcr.io": "docker.io",
				},
				ImageList: []transform.Image{
					ref,
				},
			}

			imageManifests, err := Pull(context.Background(), pullConfig)
			if tc.expectErr {
				require.Error(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)

			idx, err := getIndexFromOCILayout(filepath.Join(destDir))
			require.NoError(t, err)
			expectedAnnotations := map[string]string{
				ocispec.AnnotationRefName:       tc.ref,
				ocispec.AnnotationBaseImageName: tc.ref,
			}
			require.ElementsMatch(t, idx.Manifests[0].Annotations, expectedAnnotations)

			// Make sure all the layers of the image are pulled in
			for _, manifest := range imageManifests {
				for _, layer := range manifest.Layers {
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
	ref, err := transform.ParseImageRef("ghcr.io/fluxcd/image-automation-controller@sha256:48a89734dc82c3a2d4138554b3ad4acf93230f770b3a582f7f48be38436d031c")
	require.NoError(t, err)
	destDir := t.TempDir()
	cacheDir := t.TempDir()
	invalidContent := []byte("this mimics a corrupted file")
	// This is the sha of a layer of the image.
	// we intentionally put junk data into the cache with this layer to test that it will get cleaned up.
	correctLayerSha := "d94c8059c3cffb9278601bf9f8be070d50c84796401a4c5106eb8a4042445bbc"
	require.NoError(t, err)
	invalidLayerPath := filepath.Join(cacheDir, fmt.Sprintf("sha256:%s", correctLayerSha))
	err = os.WriteFile(invalidLayerPath, invalidContent, 0777)
	require.NoError(t, err)

	pullConfig := PullConfig{
		DestinationDirectory: destDir,
		CacheDirectory:       cacheDir,
		ImageList: []transform.Image{
			ref,
		},
	}
	_, err = Pull(context.Background(), pullConfig)
	require.NoError(t, err)

	// If the correct
	pulledLayerPath := filepath.Join(destDir, "blobs", "sha256", correctLayerSha)
	pulledLayer, err := os.ReadFile(pulledLayerPath)
	require.NoError(t, err)
	pulledLayerSha := sha256.Sum256(pulledLayer)
	require.Equal(t, correctLayerSha, fmt.Sprintf("%x", pulledLayerSha))
}
