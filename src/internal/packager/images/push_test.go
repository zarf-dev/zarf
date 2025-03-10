// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
)

func TestPush(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name       string
		cfg        PushConfig
		imageNames []string
		expectErr  bool
	}{
		{
			name: "push local images",
			cfg: PushConfig{
				SourceDirectory: "testdata/oras-oci-layout/images",
				PlainHTTP:       true,
				Arch:            "amd64",
			},
			imageNames: []string{
				"ghcr.io/local/small:1.0.0",
				"cgr.dev/chainguard/static:latest",
				"ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig",
				"ghcr.io/stefanprodan/charts/podinfo:6.4.0",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)
			port, err := helpers.GetAvailablePort()
			require.NoError(t, err)
			address := testutil.SetupInMemoryRegistry(ctx, t, port)
			imageList := []transform.Image{}
			regInfo := types.RegistryInfo{
				Address: address,
			}
			require.NoError(t, err)
			tc.cfg.RegInfo = regInfo
			for _, name := range tc.imageNames {
				ref, err := transform.ParseImageRef(name)
				require.NoError(t, err)
				imageList = append(imageList, ref)
			}
			tc.cfg.ImageList = imageList

			err = Push(ctx, tc.cfg)
			if tc.expectErr {
				require.Error(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
