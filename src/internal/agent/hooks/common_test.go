// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	"context"
	"fmt"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
)

func populateLocalRegistry(t *testing.T, ctx context.Context, localUrl string, artifact transform.Image, copyOpts oras.CopyOptions) {
	localReg, err := remote.NewRegistry(localUrl)
	require.NoError(t, err)

	localReg.PlainHTTP = true

	remoteReg, err := remote.NewRegistry(artifact.Host)
	require.NoError(t, err)

	src, err := remoteReg.Repository(ctx, artifact.Path)
	require.NoError(t, err)

	dst, err := localReg.Repository(ctx, artifact.Path)
	require.NoError(t, err)

	_, err = oras.Copy(ctx, src, artifact.Tag, dst, artifact.Tag, copyOpts)
	require.NoError(t, err)

	hashedTag, err := transform.ImageTransformHost(localUrl, fmt.Sprintf("%s/%s:%s", artifact.Host, artifact.Path, artifact.Tag))
	require.NoError(t, err)

	_, err = oras.Copy(ctx, src, artifact.Tag, dst, hashedTag, copyOpts)
	require.NoError(t, err)
}

func setupRegistry(t *testing.T, ctx context.Context, port int, artifacts []transform.Image, copyOpts oras.CopyOptions) (string, error) {
	localUrl := testutil.SetupInMemoryRegistry(ctx, t, port)

	localReg, err := remote.NewRegistry(localUrl)
	localReg.PlainHTTP = true
	if err != nil {
		return "", err
	}

	for _, art := range artifacts {
		populateLocalRegistry(t, ctx, localUrl, art, copyOpts)
	}

	return localUrl, nil
}

type mediaTypeTest struct {
	name     string
	image    string
	expected string
	artifact []transform.Image
	Opts     oras.CopyOptions
}

func TestConfigMediaTypes(t *testing.T) {
	t.Parallel()
	port, err := helpers.GetAvailablePort()
	require.NoError(t, err)

	linuxAmd64Opts := oras.DefaultCopyOptions
	linuxAmd64Opts.WithTargetPlatform(&v1.Platform{
		Architecture: "amd64",
		OS:           "linux",
	})

	tests := []mediaTypeTest{
		{
			// https://oci.dag.dev/?image=ghcr.io%2Fstefanprodan%2Fmanifests%2Fpodinfo%3A6.9.0
			name:     "flux manifest",
			expected: "application/vnd.cncf.flux.config.v1+json",
			image:    fmt.Sprintf("localhost:%d/stefanprodan/manifests/podinfo:6.9.0-zarf-2823281104", port),
			Opts:     oras.DefaultCopyOptions,
			artifact: []transform.Image{
				{
					Host: "ghcr.io",
					Path: "stefanprodan/manifests/podinfo",
					Tag:  "6.9.0",
				},
			},
		},
		{
			// https://oci.dag.dev/?image=ghcr.io%2Fstefanprodan%2Fcharts%2Fpodinfo%3A6.9.0
			name:     "helm chart manifest",
			expected: "application/vnd.cncf.helm.config.v1+json",
			image:    fmt.Sprintf("localhost:%d/stefanprodan/charts/podinfo:6.9.0", port),
			Opts:     oras.DefaultCopyOptions,
			artifact: []transform.Image{
				{
					Host: "ghcr.io",
					Path: "stefanprodan/charts/podinfo",
					Tag:  "6.9.0",
				},
			},
		},
		{
			//
			name:     "docker image manifest",
			expected: "application/vnd.oci.image.config.v1+json",
			image:    fmt.Sprintf("localhost:%d/zarf-dev/images/hello-world:latest", port),
			Opts:     linuxAmd64Opts,
			artifact: []transform.Image{
				{
					Host: "ghcr.io",
					Path: "zarf-dev/images/hello-world",
					Tag:  "latest",
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)
			url, err := setupRegistry(t, ctx, port, tt.artifact, tt.Opts)
			require.NoError(t, err)

			s := &state.State{RegistryInfo: state.RegistryInfo{Address: url}}
			mediaType, err := getManifestConfigMediaType(ctx, s, tt.image)
			require.NoError(t, err)
			require.Equal(t, tt.expected, mediaType)
		})
	}
}
