// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci_test contains tests for interacting with Zarf packages stored in OCI registries.
package zoci_test

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/packager"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
	_ "modernc.org/sqlite"
	"oras.land/oras-go/v2/registry"
)

func createRegistry(t *testing.T, ctx context.Context) registry.Reference { //nolint:revive
	// Setup destination registry
	dstPort, err := helpers.GetAvailablePort()
	require.NoError(t, err)
	dstRegistryURL := testutil.SetupInMemoryRegistry(ctx, t, dstPort)
	dstRegistryRef := registry.Reference{
		Registry:   dstRegistryURL,
		Repository: "my-namespace",
	}

	return dstRegistryRef
}

func TestAssembleLayers(t *testing.T) {
	tt := []struct {
		name string
		path string
		opts packager.PublishPackageOptions
	}{
		{
			name: "Assemble layers from a package",
			path: "testdata/basic",
			opts: packager.PublishPackageOptions{
				RemoteOptions: types.RemoteOptions{
					PlainHTTP: true,
				},
				OCIConcurrency: 3,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.TestContext(t)
			registryRef := createRegistry(t, ctx)
			tmpdir := t.TempDir()

			// create the package
			opt := packager.CreateOptions{
				OCIConcurrency: tc.opts.OCIConcurrency,
				CachePath:      tmpdir,
			}
			packagePath, err := packager.Create(ctx, tc.path, tmpdir, opt)
			require.NoError(t, err)
			// We want to pull the package and sure the content is the same as the local package
			layoutExpected, err := layout.LoadFromTar(ctx, packagePath, layout.PackageLayoutOptions{Filter: filters.Empty()})
			require.NoError(t, err)

			// Publish test package
			packageRef, err := packager.PublishPackage(ctx, layoutExpected, registryRef, tc.opts)
			require.NoError(t, err)

			// Publish creates a local oci manifest file using the package name, delete this to clean up test name
			defer os.Remove(layoutExpected.Pkg.Metadata.Name) //nolint:errcheck

			cacheModifier, err := zoci.GetOCICacheModifier(ctx, tmpdir)
			require.NoError(t, err)

			platform := oci.PlatformForArch(layoutExpected.Pkg.Build.Architecture)
			remote, err := zoci.NewRemote(ctx, packageRef.String(), platform, oci.WithPlainHTTP(tc.opts.PlainHTTP), cacheModifier)
			require.NoError(t, err)

			// get all layers
			layers, err := remote.AssembleLayers(ctx, layoutExpected.Pkg.Components, false, zoci.AllLayers)
			require.NoError(t, err)
			require.Len(t, layers, 10)

			nonDeterministicLayers := []string{"zarf.yaml", "checksums.txt"}

			// get sbom layers - it appears as though the sbom layers are not deterministic
			sbomInspectLayers, err := remote.AssembleLayers(ctx, layoutExpected.Pkg.Components, false, zoci.SbomLayers)
			require.NoError(t, err)
			require.Len(t, sbomInspectLayers, 3)

			// get image layers
			expectedImageLayers := []string{"sha256:da324ac903c3287a9ab7f12d10fea0177251ca5d1aae156b293f042a722c414d",
				"sha256:18f0797eab35a4597c1e9624aa4f15fd91f6254e5538c1e0d193b2a95dd4acc6",
				"sha256:1c4eef651f65e2f7daee7ee785882ac164b02b78fb74503052a26dc061c90474",
				"sha256:aded1e1a5b3705116fa0a92ba074a5e0b0031647d9c315983ccba2ee5428ec8b",
				"sha256:f18232174bc91741fdf3da96d85011092101a032a93a388b79e99e69c2d5c870"}
			imageInspectLayers, err := remote.AssembleLayers(ctx, layoutExpected.Pkg.Components, false, zoci.ImageLayers)
			require.NoError(t, err)
			require.Len(t, imageInspectLayers, 7)
			for _, layer := range imageInspectLayers {
				if !slices.Contains(nonDeterministicLayers, layer.Annotations["org.opencontainers.image.title"]) {
					t.Logf("Layer: %s, Title: %s", layer.Digest.String(), layer.Annotations["org.opencontainers.image.title"])
					require.Contains(t, expectedImageLayers, layer.Digest.String())
				}
			}

			// get component layers
			componentLayers, err := remote.AssembleLayers(ctx, layoutExpected.Pkg.Components, false, zoci.ComponentLayers)
			require.NoError(t, err)
			require.Len(t, componentLayers, 3)

			// get documentation layers
			docLayers, err := remote.AssembleLayers(ctx, layoutExpected.Pkg.Components, false, zoci.DocLayers)
			require.NoError(t, err)
			// 2 metadata layers (zarf.yaml, checksums.txt) + 1 documentation.tar
			require.Len(t, docLayers, 3)
		})
	}
}

func TestManifestConfigIsPackageConfig(t *testing.T) {
	ctx := testutil.TestContext(t)
	registryRef := createRegistry(t, ctx)
	tmpdir := t.TempDir()

	opt := packager.CreateOptions{
		CachePath: tmpdir,
	}
	packagePath, err := packager.Create(ctx, "testdata/basic", tmpdir, opt)
	require.NoError(t, err)

	pkgLayout, err := layout.LoadFromTar(ctx, packagePath, layout.PackageLayoutOptions{Filter: filters.Empty()})
	require.NoError(t, err)

	publishOpts := packager.PublishPackageOptions{
		RemoteOptions: types.RemoteOptions{
			PlainHTTP: true,
		},
		OCIConcurrency: 3,
	}
	packageRef, err := packager.PublishPackage(ctx, pkgLayout, registryRef, publishOpts)
	require.NoError(t, err)

	cacheModifier, err := zoci.GetOCICacheModifier(ctx, tmpdir)
	require.NoError(t, err)

	platform := oci.PlatformForArch(pkgLayout.Pkg.Build.Architecture)
	remote, err := zoci.NewRemote(ctx, packageRef.String(), platform, oci.WithPlainHTTP(true), cacheModifier)
	require.NoError(t, err)

	// Fetch the OCI manifest root
	root, err := remote.FetchRoot(ctx)
	require.NoError(t, err)

	// Verify the manifest config has the correct media type
	require.Equal(t, zoci.ZarfConfigMediaType, root.Config.MediaType)

	// Fetch and unmarshal the manifest config blob
	configBytes, err := remote.FetchLayer(ctx, root.Config)
	require.NoError(t, err)

	var configPkg v1alpha1.ZarfPackage
	err = json.Unmarshal(configBytes, &configPkg)
	require.NoError(t, err)

	// Verify the config contains the original package data
	require.Equal(t, pkgLayout.Pkg.Kind, configPkg.Kind)
	require.Equal(t, pkgLayout.Pkg.Metadata.Name, configPkg.Metadata.Name)
	require.Equal(t, pkgLayout.Pkg.Metadata.Version, configPkg.Metadata.Version)
	require.Equal(t, pkgLayout.Pkg.Build.Architecture, configPkg.Build.Architecture)
	require.Equal(t, pkgLayout.Pkg.Components[0].Name, configPkg.Components[0].Name)
}
