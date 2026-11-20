// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
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

func TestPushPackageManifestReadAfterWrite(t *testing.T) {
	ctx := testutil.TestContext(t)
	address := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	repo := testutil.NewRepo(t, address+"/multi-component:0.0.1")
	proxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: address})

	// Return a 404 on the first read of a manifest after it is uploaded by
	// digest. Publishing uses the local descriptor to update the version index.
	var unreadable sync.Map
	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.Request.Method == http.MethodPut && resp.StatusCode == http.StatusCreated &&
			strings.Contains(resp.Request.URL.Path, "/manifests/sha256:") {
			unreadable.Store(resp.Request.URL.Path, struct{}{})
		}
		return nil
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			if _, pending := unreadable.LoadAndDelete(r.URL.Path); pending {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	destination := repo.Reference
	destination.Registry = strings.TrimPrefix(server.URL, "http://")

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

		remote, err := zoci.NewRemote(ctx, destination.String(), oci.PlatformForArch(arch), zoci.RemoteClientOptions{
			RemoteOptions: types.RemoteOptions{PlainHTTP: true},
		})
		require.NoError(t, err)

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
