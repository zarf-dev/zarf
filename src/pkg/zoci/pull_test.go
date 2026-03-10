// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci_test contains tests for interacting with Zarf packages stored in OCI registries.
package zoci_test

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/packager"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
	_ "modernc.org/sqlite"
	"oras.land/oras-go/v2/registry"
)

func createRegistry(t *testing.T, ctx context.Context) registry.Reference { //nolint:revive
	dstPort, err := helpers.GetAvailablePort()
	require.NoError(t, err)
	dstRegistryURL := testutil.SetupInMemoryRegistry(ctx, t, dstPort)
	return registry.Reference{
		Registry:   dstRegistryURL,
		Repository: "my-namespace",
	}
}

// publishAndConnect creates a package from srcPath, publishes it to a fresh registry,
// and returns a connected Remote along with the loaded PackageLayout.
func publishAndConnect(ctx context.Context, t *testing.T, srcPath string) (*zoci.Remote, *layout.PackageLayout) {
	t.Helper()
	registryRef := createRegistry(t, ctx)
	tmpdir := t.TempDir()

	packagePath, err := packager.Create(ctx, srcPath, tmpdir, packager.CreateOptions{
		OCIConcurrency: 3,
		CachePath:      tmpdir,
	})
	require.NoError(t, err)

	pkgLayout, err := layout.LoadFromTar(ctx, packagePath, layout.PackageLayoutOptions{})
	require.NoError(t, err)

	packageRef, err := packager.PublishPackage(ctx, pkgLayout, registryRef, packager.PublishPackageOptions{
		RemoteOptions:  types.RemoteOptions{PlainHTTP: true},
		OCIConcurrency: 3,
	})
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(pkgLayout.Pkg.Metadata.Name) }) //nolint:errcheck

	cacheModifier, err := zoci.GetOCICacheModifier(ctx, tmpdir)
	require.NoError(t, err)

	platform := oci.PlatformForArch(pkgLayout.Pkg.Build.Architecture)
	remote, err := zoci.NewRemote(ctx, packageRef.String(), platform, append([]oci.Modifier{oci.WithPlainHTTP(true)}, cacheModifier)...)
	require.NoError(t, err)

	return remote, pkgLayout
}

func TestAllLayersRespectsRequestedComponents(t *testing.T) {
	ctx := testutil.TestContext(t)
	remote, pkgLayout := publishAndConnect(ctx, t, "testdata/multi-component")

	alpineOnly := []v1alpha1.ZarfComponent{{Name: "alpine"}}
	bothComponents := pkgLayout.Pkg.Components

	allLayersFull, err := remote.AssembleLayers(ctx, bothComponents, zoci.GetAllLayerTypes()...)
	require.NoError(t, err)
	require.Len(t, allLayersFull, 4)

	allLayersSubset, err := remote.AssembleLayers(ctx, alpineOnly, zoci.GetAllLayerTypes()...)
	require.NoError(t, err)
	require.Len(t, allLayersSubset, 3)
}

func TestAssembleLayers(t *testing.T) {
	ctx := testutil.TestContext(t)

	remote, pkgLayout := publishAndConnect(ctx, t, "testdata/basic")
	components := pkgLayout.Pkg.Components

	layers, err := remote.AssembleLayers(ctx, components, zoci.GetAllLayerTypes()...)
	require.NoError(t, err)
	require.Len(t, layers, 10)

	nonDeterministicLayers := []string{"zarf.yaml", "checksums.txt"}

	// get sbom layers
	sbomInspectLayers, err := remote.AssembleLayers(ctx, components, zoci.SbomLayers)
	require.NoError(t, err)
	require.Len(t, sbomInspectLayers, 3)

	// get image layers
	expectedImageLayers := []string{
		"sha256:da324ac903c3287a9ab7f12d10fea0177251ca5d1aae156b293f042a722c414d",
		"sha256:18f0797eab35a4597c1e9624aa4f15fd91f6254e5538c1e0d193b2a95dd4acc6",
		"sha256:1c4eef651f65e2f7daee7ee785882ac164b02b78fb74503052a26dc061c90474",
		"sha256:aded1e1a5b3705116fa0a92ba074a5e0b0031647d9c315983ccba2ee5428ec8b",
		"sha256:f18232174bc91741fdf3da96d85011092101a032a93a388b79e99e69c2d5c870",
	}
	imageInspectLayers, err := remote.AssembleLayers(ctx, components, zoci.ImageLayers)
	require.NoError(t, err)
	require.Len(t, imageInspectLayers, 7)
	for _, layer := range imageInspectLayers {
		if !slices.Contains(nonDeterministicLayers, layer.Annotations["org.opencontainers.image.title"]) {
			t.Logf("Layer: %s, Title: %s", layer.Digest.String(), layer.Annotations["org.opencontainers.image.title"])
			require.Contains(t, expectedImageLayers, layer.Digest.String())
		}
	}

	// get component layers
	componentLayers, err := remote.AssembleLayers(ctx, components, zoci.ComponentLayers)
	require.NoError(t, err)
	require.Len(t, componentLayers, 3)

	// get documentation layers
	docLayers, err := remote.AssembleLayers(ctx, components, zoci.DocLayers)
	require.NoError(t, err)
	// 2 metadata layers (zarf.yaml, checksums.txt) + 1 documentation.tar
	require.Len(t, docLayers, 3)
}
