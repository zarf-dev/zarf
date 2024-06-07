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

// func setupInMemoryRegistry(ctx context.Context, t *testing.T) string {
// 	port, err := freeport.GetFreePort()
// 	require.NoError(t, err)
// 	config := &configuration.Configuration{}
// 	config.HTTP.Addr = fmt.Sprintf(":%d", port)
// 	config.HTTP.Secret = "Fake secret so we don't get warning"
// 	config.Log.AccessLog.Disabled = true
// 	config.HTTP.DrainTimeout = 10 * time.Second
// 	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}

// 	ref, err := registry.NewRegistry(ctx, config)
// 	require.NoError(t, err)

// 	go ref.ListenAndServe()
// 	return fmt.Sprintf("localhost:%d", port)
// }

func TestPull(t *testing.T) {
	t.Run("pulling a cosign image with a full cache does not result in error", func(t *testing.T) {

		// registryUrl := setupInMemoryRegistry(t, context.Background())
		// img, err := crane.Load(filepath.Join("testdata", "image.tar"), []crane.Option{}...)
		// require.NoError(t, err)
		// imgEndpoint := fmt.Sprintf("%s/%s", registryUrl, "cosign-image:sha256-test")
		// crane.Push(img, imgEndpoint)
		ref, err := transform.ParseImageRef("ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig")
		// ref, err := transform.ParseImageRef(imgEndpoint)

		require.NoError(t, err)
		PullConfig := PullConfig{
			DestinationDirectory: filepath.Join("testdata", "images"),
			CacheDirectory:       filepath.Join("testdata", "cache"),
			ImageList: []transform.Image{
				ref,
			},
		}

		_, err = Pull(context.Background(), PullConfig)
		require.NoError(t, err)
	})
}
