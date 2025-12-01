// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"fmt"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry"
	orasRemote "oras.land/oras-go/v2/registry/remote"
)

func TestPush(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name            string
		SourceDirectory string
		imageNames      []string
		expectErr       bool
		namespace       string
	}{
		{
			name: "push local images oras",
			// This OCI format directory was created by building the package at src/test/packages/39-crane-to-oras with the ORAS implementation
			SourceDirectory: "testdata/oras-oci-layout/images",
			imageNames: []string{
				"local-test:1.0.0",
				"localhost:9999/local-test:1.0.0",
				"ghcr.io/zarf-dev/images/hello-world:latest",
				"ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig",
				"ghcr.io/stefanprodan/charts/podinfo:6.4.0",
				"hello-world@sha256:03b62250a3cb1abd125271d393fc08bf0cc713391eda6b57c02d1ef85efcc25c",
			},
		},
		{
			name: "push local images crane",
			// This OCI format directory was created by building the package at src/test/packages/39-crane-to-oras with the Crane implementation (Zarf v0.49.1)
			SourceDirectory: "testdata/crane-oci-layout/images",
			imageNames: []string{
				"local-test:1.0.0",
				"localhost:9999/local-test:1.0.0",
				"ghcr.io/zarf-dev/images/hello-world:latest",
				"ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig",
				"ghcr.io/stefanprodan/charts/podinfo:6.4.0",
				"hello-world@sha256:03b62250a3cb1abd125271d393fc08bf0cc713391eda6b57c02d1ef85efcc25c",
			},
		},
		{
			name:            "push image to namespace",
			SourceDirectory: "testdata/oras-oci-layout/images",
			imageNames: []string{
				"local-test:1.0.0",
			},
			namespace: "my-namespace",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Push overwrites the index, this code sets it back, this means we can't run these tests in parallel
			idx, err := getIndexFromOCILayout(tc.SourceDirectory)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, saveIndexToOCILayout(tc.SourceDirectory, idx))
			}()
			ctx := testutil.TestContext(t)
			// setup in memory registry
			port, err := helpers.GetAvailablePort()
			require.NoError(t, err)
			address := testutil.SetupInMemoryRegistry(ctx, t, port)
			if tc.namespace != "" {
				address = fmt.Sprintf("%s/%s", address, tc.namespace)
			}
			imageList := []transform.Image{}
			regInfo := state.RegistryInfo{
				Address: address,
			}
			require.NoError(t, err)

			for _, name := range tc.imageNames {
				ref, err := transform.ParseImageRef(name)
				require.NoError(t, err)
				imageList = append(imageList, ref)
			}

			// push images to registry
			opts := PushOptions{
				PlainHTTP: true,
				Arch:      "amd64",
			}
			err = Push(ctx, imageList, tc.SourceDirectory, regInfo, opts)

			if tc.expectErr {
				require.Error(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)

			// Verify all images are in the repo
			for _, image := range tc.imageNames {
				checksumRef, err := transform.ImageTransformHost(address, image)
				require.NoError(t, err)
				verifyImageExists(ctx, t, checksumRef)
				ref, err := transform.ImageTransformHostWithoutChecksum(address, image)
				require.NoError(t, err)
				verifyImageExists(ctx, t, ref)
			}
		})
	}
}

func verifyImageExists(ctx context.Context, t *testing.T, ref string) {
	repo := &orasRemote.Repository{}
	var err error
	repo.Reference, err = registry.ParseReference(ref)
	require.NoError(t, err)
	repo.PlainHTTP = true
	_, err = oras.Resolve(ctx, repo, ref, oras.DefaultResolveOptions)
	require.NoError(t, err)
}
