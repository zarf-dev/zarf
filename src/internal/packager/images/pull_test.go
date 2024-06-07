// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/stretchr/testify/require"
)

func TestPull(t *testing.T) {
	t.Run("pulling a cosign image is successful and doesn't add anything to the cache", func(t *testing.T) {

		ref, err := transform.ParseImageRef("ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig")
		require.NoError(t, err)
		tmpDestDir := t.TempDir()
		tmpCacheDir := t.TempDir()
		destDir := filepath.Join(tmpDestDir, "images")
		pullConfig := PullConfig{
			DestinationDirectory: destDir,
			CacheDirectory:       tmpCacheDir,
			ImageList: []transform.Image{
				ref,
			},
		}

		_, err = Pull(context.Background(), pullConfig)
		require.NoError(t, err)
		require.FileExists(t, filepath.Join(destDir, "blobs/sha256/3e84ea487b4c52a3299cf2996f70e7e1721236a0998da33a0e30107108486b3e"))

		files, err := filepath.Glob(filepath.Join(tmpCacheDir, "*"))
		require.NoError(t, err)
		require.Empty(t, files)
	})
}
