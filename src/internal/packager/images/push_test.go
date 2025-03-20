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
		name            string
		SourceDirectory string
		imageNames      []string
		expectErr       bool
	}{
		{
			name:            "push local images oras",
			// This OCI format directory was created by building the package at src/test/packages/39-crane-to-oras with the ORAS implementation
			SourceDirectory: "testdata/oras-oci-layout/images",
			imageNames: []string{
				"local-test:1.0.0",
				"ghcr.io/zarf-dev/images/hello-world:latest",
				"ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig",
				"ghcr.io/stefanprodan/charts/podinfo:6.4.0",
				"hello-world@sha256:03b62250a3cb1abd125271d393fc08bf0cc713391eda6b57c02d1ef85efcc25c",
			},
		},
		{
			name:            "push local images crane",
			// This OCI format directory was created by building the package at src/test/packages/39-crane-to-oras with the Crane implementation (Zarf v0.49.1)
			SourceDirectory: "testdata/crane-oci-layout/images",
			imageNames: []string{
				"local-test:1.0.0",
				"ghcr.io/zarf-dev/images/hello-world:latest",
				"ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig",
				"ghcr.io/stefanprodan/charts/podinfo:6.4.0",
				"hello-world@sha256:03b62250a3cb1abd125271d393fc08bf0cc713391eda6b57c02d1ef85efcc25c",
			},
		},
		{
			name:            "push local images crane",
			SourceDirectory: "testdata/oras-oci-layout/images",
			imageNames: []string{
				"this-image-does-not-exist:1.0.0",
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)
			// setup in memory registry
			port, err := helpers.GetAvailablePort()
			require.NoError(t, err)
			address := testutil.SetupInMemoryRegistry(ctx, t, port)
			imageList := []transform.Image{}
			regInfo := types.RegistryInfo{
				Address: address,
			}
			require.NoError(t, err)

			for _, name := range tc.imageNames {
				ref, err := transform.ParseImageRef(name)
				require.NoError(t, err)
				imageList = append(imageList, ref)
			}

			// push images to registry
			cfg := PushConfig{
				SourceDirectory: tc.SourceDirectory,
				RegInfo:         regInfo,
				PlainHTTP:       true,
				Arch:            "amd64",
				ImageList:       imageList,
			}
			err = Push(ctx, cfg)

			if tc.expectErr {
				require.Error(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
