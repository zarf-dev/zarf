// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci_test contains tests for interacting with Zarf packages stored in OCI registries.
package zoci_test

import (
	"context"
	"os"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/internal/packager2"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/test/testutil"
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
		opts packager2.PublishPackageOpts
	}{
		{
			name: "Publish package",
			path: "../../internal/packager2/testdata/zarf-package-test-amd64-0.0.1.tar.zst",
			opts: packager2.PublishPackageOpts{
				WithPlainHTTP: true,
				Architecture:  "amd64",
				Concurrency:   3,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.TestContext(t)
			registryRef := createRegistry(t, ctx)

			// Publish test package
			err := packager2.PublishPackage(ctx, tc.path, registryRef, tc.opts)
			require.NoError(t, err)

			// We want to pull the package and sure the content is the same as the local package
			layoutExpected, err := layout.LoadFromTar(ctx, tc.path, layout.PackageLayoutOptions{Filter: filters.Empty()})
			require.NoError(t, err)
			// // Publish creates a local oci manifest file using the package name, delete this to clean up test name
			defer os.Remove(layoutExpected.Pkg.Metadata.Name) //nolint:errcheck
			// // Format url and instantiate remote
			packageRef, err := zoci.ReferenceFromMetadata(registryRef.String(), &layoutExpected.Pkg.Metadata, &layoutExpected.Pkg.Build)
			require.NoError(t, err)

			platform := oci.PlatformForArch(tc.opts.Architecture)
			remote, err := zoci.NewRemote(ctx, packageRef, platform, oci.WithPlainHTTP(tc.opts.WithPlainHTTP))
			require.NoError(t, err)

			// get all layers
			layers, err := remote.AssembleLayers(ctx, layoutExpected.Pkg.Components, false, zoci.AllLayers)
			require.NoError(t, err)
			t.Logf("Layers: %v", layers)
			require.NotEmpty(t, layers)

			// get sbom layers
			sbomLayers, err := remote.AssembleLayers(ctx, layoutExpected.Pkg.Components, false, zoci.SbomLayers)
			require.NoError(t, err)
			t.Logf("SBOM Layers: %v", sbomLayers)
			require.NotEmpty(t, sbomLayers)

			// get image layers
			imageLayers, err := remote.AssembleLayers(ctx, layoutExpected.Pkg.Components, false, zoci.ImageLayers)
			require.NoError(t, err)
			t.Logf("Image Layers: %v", imageLayers)
			require.NotEmpty(t, imageLayers)

			// get component layers
			componentLayers, err := remote.AssembleLayers(ctx, layoutExpected.Pkg.Components, false, zoci.ComponentLayers)
			require.NoError(t, err)
			t.Logf("Component Layers: %v", componentLayers)
			require.NotEmpty(t, componentLayers)
		})
	}
}
