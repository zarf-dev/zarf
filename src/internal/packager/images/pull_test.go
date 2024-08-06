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
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

func TestCheckForIndex(t *testing.T) {
	t.Parallel()
	testData := []struct {
		name        string
		ref         string
		file        string
		expectedErr string
	}{
		{
			name:        "index sha",
			ref:         "ghcr.io/zarf-dev/zarf/agent:v0.32.6@sha256:05a82656df5466ce17c3e364c16792ae21ce68438bfe06eeab309d0520c16b48",
			file:        "agent-index.json",
			expectedErr: pullIndexShaErr,
		},
		{
			name:        "docker manifest list",
			ref:         "defenseunicorns/zarf-game@sha256:0b694ca1c33afae97b7471488e07968599f1d2470c629f76af67145ca64428af",
			file:        "game-index.json",
			expectedErr: pullIndexShaErr,
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

	for _, tc := range testData {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			refInfo, err := transform.ParseImageRef(tc.ref)
			require.NoError(t, err)
			file := filepath.Join("testdata", tc.file)
			manifest, err := os.ReadFile(file)
			require.NoError(t, err)
			var idx v1.IndexManifest
			jsonErr := json.Unmarshal(manifest, &idx)
			require.NoError(t, jsonErr)
			desc := remote.Descriptor{
				Descriptor: v1.Descriptor{
					MediaType: idx.MediaType,
				},
				Manifest: manifest,
			}
			err = checkForIndex(refInfo, desc)
			if err != nil {
				require.ErrorContains(t, err, fmt.Errorf(tc.expectedErr, refInfo.Reference, "").Error())
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
	t.Run("pulling a cosign image is successful and doesn't add anything to the cache", func(t *testing.T) {
		ref, err := transform.ParseImageRef("ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig")
		require.NoError(t, err)
		destDir := t.TempDir()
		cacheDir := t.TempDir()
		pullConfig := PullConfig{
			DestinationDirectory: destDir,
			CacheDirectory:       cacheDir,
			ImageList: []transform.Image{
				ref,
			},
		}

		_, err = Pull(context.Background(), pullConfig)
		require.NoError(t, err)
		require.FileExists(t, filepath.Join(destDir, "blobs/sha256/3e84ea487b4c52a3299cf2996f70e7e1721236a0998da33a0e30107108486b3e"))

		dir, err := os.ReadDir(cacheDir)
		require.NoError(t, err)
		require.Empty(t, dir)
	})
}
