// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

func TestCheckForIndex(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		ref         string
		file        string
		expectedErr string
	}{
		{
			name:        "index sha",
			ref:         "ghcr.io/zarf-dev/zarf/agent:v0.32.6@sha256:05a82656df5466ce17c3e364c16792ae21ce68438bfe06eeab309d0520c16b48",
			file:        "agent-index.json",
			expectedErr: "%s resolved to an OCI image index which is not supported by Zarf, select a specific platform to use",
		},
		{
			name:        "docker manifest list",
			ref:         "defenseunicorns/zarf-game@sha256:0b694ca1c33afae97b7471488e07968599f1d2470c629f76af67145ca64428af",
			file:        "game-index.json",
			expectedErr: "%s resolved to an OCI image index which is not supported by Zarf, select a specific platform to use",
		},
		{
			name:        "image manifest",
			ref:         "ghcr.io/zarf-dev/zarf/agent:v0.32.6",
			file:        "agent-manifest.json",
			expectedErr: "",
		},
		{
			name:        "image manifest sha'd",
			ref:         "ghcr.io/zarf-dev/zarf/agent:v0.32.6@sha256:b3fabdc7d4ecd0f396016ef78da19002c39e3ace352ea0ae4baa2ce9d5958376",
			file:        "agent-manifest.json",
			expectedErr: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			refInfo, err := transform.ParseImageRef(tc.ref)
			require.NoError(t, err)
			file := filepath.Join("testdata", tc.file)
			manifest, err := os.ReadFile(file)
			require.NoError(t, err)
			var idx v1.IndexManifest
			err = json.Unmarshal(manifest, &idx)
			require.NoError(t, err)
			tmp := t.TempDir()
			cfg := PullConfig{
				Arch:                 "arm64",
				DestinationDirectory: tmp,
				ImageList:            []transform.Image{refInfo},
				CacheDirectory:       t.TempDir(),
			}
			_, err = Pull(context.Background(), cfg)
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
			arch:      "amd64",
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ref, err := transform.ParseImageRef(tc.ref)
			require.NoError(t, err)
			destDir := t.TempDir()
			pullConfig := PullConfig{
				DestinationDirectory: destDir,
				Arch:                 tc.arch,
				ImageList: []transform.Image{
					ref,
				},
			}

			_, err = Pull(context.Background(), pullConfig)
			if tc.expectErr {
				require.Error(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)

			// // Make sure all the layers of the image are pulled in
			// for _, desc := range descs {
			// 	digestHash, err := desc.
			// 	require.NoError(t, err)
			// 	digest, _ := strings.CutPrefix(digestHash.String(), "sha256:")
			// 	require.FileExists(t, filepath.Join(destDir, fmt.Sprintf("blobs/sha256/%s", digest)))
			// }
		})
	}

	// t.Run("pulling a cosign image is successful and doesn't add anything to the cache", func(t *testing.T) {
	// 	t.Parallel()
	// 	ref, err := transform.ParseImageRef("ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig")
	// 	require.NoError(t, err)
	// 	destDir := t.TempDir()
	// 	cacheDir := t.TempDir()
	// 	pullConfig := PullConfig{
	// 		DestinationDirectory: destDir,
	// 		CacheDirectory:       cacheDir,
	// 		ImageList: []transform.Image{
	// 			ref,
	// 		},
	// 	}

	// 	_, err = Pull(context.Background(), pullConfig)
	// 	require.NoError(t, err)
	// 	require.FileExists(t, filepath.Join(destDir, "blobs/sha256/3e84ea487b4c52a3299cf2996f70e7e1721236a0998da33a0e30107108486b3e"))

	// 	dir, err := os.ReadDir(cacheDir)
	// 	require.NoError(t, err)
	// 	require.Empty(t, dir)
	// })

	// t.Run("pulling an image with an invalid layer in the cache should still pull the image", func(t *testing.T) {
	// 	t.Parallel()
	// 	ref, err := transform.ParseImageRef("ghcr.io/fluxcd/image-automation-controller@sha256:48a89734dc82c3a2d4138554b3ad4acf93230f770b3a582f7f48be38436d031c")
	// 	require.NoError(t, err)
	// 	destDir := t.TempDir()
	// 	cacheDir := t.TempDir()
	// 	invalidContent := []byte("this mimics a corrupted file")
	// 	// This is the sha of a layer of the image. Crane will make a file using this sha in the cache
	// 	// we intentionally put junk data into the cache with this layer to test that it will get cleaned up.
	// 	correctLayerSha := "d94c8059c3cffb9278601bf9f8be070d50c84796401a4c5106eb8a4042445bbc"
	// 	hash, err := v1.NewHash(fmt.Sprintf("sha256:%s", correctLayerSha))
	// 	require.NoError(t, err)
	// 	invalidLayerPath := layerCachePath(cacheDir, hash)
	// 	err = os.WriteFile(invalidLayerPath, invalidContent, 0777)
	// 	require.NoError(t, err)

	// 	pullConfig := PullConfig{
	// 		DestinationDirectory: destDir,
	// 		CacheDirectory:       cacheDir,
	// 		ImageList: []transform.Image{
	// 			ref,
	// 		},
	// 	}
	// 	_, err = Pull(context.Background(), pullConfig)
	// 	require.NoError(t, err)

	// 	// Verify the cache layer has the correct sha
	// 	nowValidCachedLayer, err := os.ReadFile(invalidLayerPath)
	// 	cachedLayerSha := sha256.Sum256(nowValidCachedLayer)
	// 	require.Equal(t, correctLayerSha, fmt.Sprintf("%x", cachedLayerSha))
	// 	require.NoError(t, err)
	// 	// Verify the pulled layer hsa the correct sha
	// 	pulledLayerPath := filepath.Join(destDir, "blobs", "sha256", hash.Hex)
	// 	pulledLayer, err := os.ReadFile(pulledLayerPath)
	// 	require.NoError(t, err)
	// 	pulledLayerSha := sha256.Sum256(pulledLayer)
	// 	require.Equal(t, correctLayerSha, fmt.Sprintf("%x", pulledLayerSha))
	// })
}
