// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
)

func populateLocalRegistry(t *testing.T, ctx context.Context, localUrl string, artifact transform.Image) error {
	localReg, err := remote.NewRegistry(localUrl)
	if err != nil {
		return err
	}
	localReg.PlainHTTP = true

	remoteReg, err := remote.NewRegistry(artifact.Host)
	if err != nil {
		return err
	}

	src, err := remoteReg.Repository(ctx, artifact.Path)
	if err != nil {
		return err
	}
	dst, err := localReg.Repository(ctx, artifact.Path)
	if err != nil {
		return err
	}
	_, err = oras.Copy(ctx, src, artifact.Tag, dst, artifact.Tag, oras.DefaultCopyOptions)
	if err != nil {
		return err
	}

	hashedTag, err := transform.ImageTransformHost(localUrl, fmt.Sprintf("%s/%s:%s", artifact.Host, artifact.Path, artifact.Tag))
	if err != nil {
		return err
	}

	_, err = oras.Copy(ctx, src, artifact.Tag, dst, hashedTag, oras.DefaultCopyOptions)
	if err != nil {
		return err
	}

	return nil
}

func setupRegistry(t *testing.T, ctx context.Context) (string, error) {
	localUrl := testutil.SetupInMemoryRegistry(ctx, t, 5000)

	localReg, err := remote.NewRegistry(localUrl)
	localReg.PlainHTTP = true
	if err != nil {
		return "", err
	}
	var artifacts = []transform.Image{
		{
			Host: "ghcr.io",
			Path: "stefanprodan/charts/podinfo",
			Tag:  "6.9.0",
		},
		{
			Host: "ghcr.io",
			Path: "stefanprodan/manifests/podinfo",
			Tag:  "6.9.0",
		},
		{
			Host: "ghcr.io",
			Path: "stefanprodan/podinfo",
			Tag:  "6.9.0",
		},
	}

	for _, art := range artifacts {
		err := populateLocalRegistry(t, ctx, localUrl, art)
		if err != nil {
			return "", err
		}
	}

	return localUrl, nil
}

type mediaTypeTest struct {
	name     string
	image    string
	expected string
}

func TestConfigMediaTypes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	url, err := setupRegistry(t, ctx)
	if err != nil {
		panic(err)
	}

	tests := []mediaTypeTest{
		{
			// https://oci.dag.dev/?image=ghcr.io%2Fstefanprodan%2Fmanifests%2Fpodinfo%3A6.9.0
			name:     "flux manifest",
			expected: "application/vnd.cncf.flux.config.v1+json",
			image:    "localhost:5000/stefanprodan/manifests/podinfo:6.9.0-zarf-2823281104",
		},
		{
			// https://oci.dag.dev/?image=ghcr.io%2Fstefanprodan%2Fcharts%2Fpodinfo%3A6.9.0
			name:     "helm chart manifest",
			expected: "application/vnd.cncf.helm.config.v1+json",
			image:    "localhost:5000/stefanprodan/charts/podinfo:6.9.0",
		},
		{
			// docker images do not include a `.config.mediaType`
			// https://oci.dag.dev/?image=ghcr.io%2Fstefanprodan%2Fpodinfo%3A6.9.0
			name:     "docker image manifest",
			expected: "",
			image:    "localhost:5000/stefanprodan/podinfo:6.9.0-zarf-2985051089",
		},
	}

	s := &state.State{RegistryInfo: types.RegistryInfo{
		Address:      url,
		PushUsername: "",
		PushPassword: "",
		PullUsername: "",
		PullPassword: "",
	}}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mediaType, err := getManifestConfigMediaType(ctx, s, tt.image)
			require.NoError(t, err)
			require.Equal(t, tt.expected, mediaType)
		})
	}
}
