// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/oci"
	"github.com/zarf-dev/zarf/src/pkg/packager"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
	_ "modernc.org/sqlite"
)

func TestPushPackage(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	pkg := buildVirtualPackage(ctx, t)

	pkgLayout, err := layout.LoadFromTar(ctx, pkg.packagePath, layout.PackageLayoutOptions{Filter: filters.Empty()})
	require.NoError(t, err)

	pkgDefinition := pkgLayout.Definition()
	platform := oci.PlatformForArch(pkgDefinition.Build.Architecture)
	remote, err := zoci.NewRemote(ctx, pkg.registryAddr+"/"+pkgDefinition.Metadata.Name+":"+pkgDefinition.Metadata.Version, platform, zoci.RemoteClientOptions{
		RemoteOptions: types.RemoteOptions{PlainHTTP: true},
	})
	require.NoError(t, err)

	desc, err := remote.PushPackage(ctx, pkgLayout, zoci.PublishOptions{
		OCIConcurrency: 3,
		Retries:        1,
	})
	require.NoError(t, err)
	require.NotEmpty(t, desc.Digest.String())
	require.Positive(t, desc.Size)

	fetchedRoot, err := remote.FetchRoot(ctx)
	require.NoError(t, err)
	require.Equal(t, layout.ZarfConfigMediaType, fetchedRoot.Config.MediaType)

	configBytes, err := remote.FetchLayer(ctx, fetchedRoot.Config)
	require.NoError(t, err)
	var configPkg v1alpha1.ZarfPackage
	require.NoError(t, json.Unmarshal(configBytes, &configPkg))
	require.Equal(t, pkgLayout.Definition().Metadata.Name, configPkg.Metadata.Name)
	require.Equal(t, pkgLayout.Definition().Metadata.Version, configPkg.Metadata.Version)
}

// FIXME: consider removing this test
func TestPushPackageManifestReadAfterWrite(t *testing.T) {
	ctx := testutil.TestContext(t)
	address := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	repo := testutil.NewRepo(t, address+"/multi-component:0.0.1")
	expected := map[string]string{}
	for _, arch := range []string{"amd64", "arm64"} {
		packagePath, err := packager.Create(ctx, "testdata/multi-component", t.TempDir(), packager.CreateOptions{
			Architecture: arch,
			CachePath:    t.TempDir(),
			SkipSBOM:     true,
		})
		require.NoError(t, err)
		pkgLayout, err := layout.LoadFromTar(ctx, packagePath, layout.PackageLayoutOptions{})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })
		expected[arch] = pkgLayout.Digest()

		remote, err := zoci.NewRemote(ctx, repo.Reference.String(), oci.PlatformForArch(arch), zoci.RemoteClientOptions{
			RemoteOptions: types.RemoteOptions{PlainHTTP: true},
		})
		require.NoError(t, err)
		client := remote.Repo().Client
		var unreadable atomic.Bool
		remote.Repo().Client = registryClientFunc(func(req *http.Request) (*http.Response, error) {
			// After accepting the manifest, fail its first digest read with 404.
			isManifest := strings.HasSuffix(req.URL.Path, "/manifests/"+pkgLayout.Digest())
			if isManifest && (req.Method == http.MethodGet || req.Method == http.MethodHead) && unreadable.Swap(false) {
				return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody, Request: req}, nil
			}
			resp, err := client.Do(req)
			if err == nil && isManifest && req.Method == http.MethodPut && resp.StatusCode == http.StatusCreated {
				unreadable.Store(true)
			}
			return resp, err
		})

		desc, err := remote.PushPackage(ctx, pkgLayout, zoci.PublishOptions{Retries: 1})
		require.NoError(t, err, "publishing %s must tolerate an unreadable newly uploaded manifest", arch)
		require.Equal(t, pkgLayout.Digest(), desc.Digest.String())
		manifest, err := oci.FetchUnmarshal[oci.Manifest](ctx, repo, json.Unmarshal, desc)
		require.NoError(t, err)
		localManifest, err := pkgLayout.Manifest()
		require.NoError(t, err)
		require.Equal(t, *localManifest, manifest)
	}

	desc, err := repo.Resolve(ctx, repo.Reference.Reference)
	require.NoError(t, err)
	index, err := oci.FetchUnmarshal[ocispec.Index](ctx, repo, json.Unmarshal, desc)
	require.NoError(t, err)
	require.Len(t, index.Manifests, len(expected))
	actual := map[string]string{}
	for _, manifest := range index.Manifests {
		require.NotNil(t, manifest.Platform)
		actual[manifest.Platform.Architecture] = manifest.Digest.String()
	}
	require.Equal(t, expected, actual)
}

type registryClientFunc func(*http.Request) (*http.Response, error)

func (f registryClientFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }
