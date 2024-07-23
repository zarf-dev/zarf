// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

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
