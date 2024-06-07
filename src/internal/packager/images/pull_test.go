// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/transform"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory" // used for docker test registry[]
	"github.com/stretchr/testify/require"
)

func TestPull(t *testing.T) {
	t.Run("pulling a cosign image with the layer already cached does not result in error", func(t *testing.T) {

		ref, err := transform.ParseImageRef("ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig")
		tmpDir := t.TempDir()
		require.NoError(t, err)
		PullConfig := PullConfig{
			DestinationDirectory: filepath.Join(tmpDir, "images"),
			CacheDirectory:       filepath.Join("testdata", "cache"),
			ImageList: []transform.Image{
				ref,
			},
		}

		_, err = Pull(context.Background(), PullConfig)
		require.NoError(t, err)
	})
}
