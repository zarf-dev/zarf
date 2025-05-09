// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci_test contains tests for interacting with Zarf packages stored in OCI registries.
package zoci_test

import (
	"context"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager2"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/lint"
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
	lint.ZarfSchema = testutil.LoadSchema(t, "../../../zarf.schema.json")
	tt := []struct {
		name string
		path string
		opts packager2.PublishPackageOpts
	}{
		{
			name: "Assemble layers from a package",
			path: "testdata/basic",
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
			tmpdir := t.TempDir()

			//
			config.CommonOptions.CachePath = tmpdir

			// create the package
			opt := packager2.CreateOptions{
				Output:         tmpdir,
				OCIConcurrency: tc.opts.Concurrency,
			}
			err := packager2.Create(ctx, tc.path, opt)
			require.NoError(t, err)
			src := fmt.Sprintf("%s/%s-%s-0.0.1.tar.zst", tmpdir, "zarf-package-basic-pod", tc.opts.Architecture)

			// Publish test package
			err = packager2.PublishPackage(ctx, src, registryRef, tc.opts)
			require.NoError(t, err)

			// We want to pull the package and sure the content is the same as the local package
			layoutExpected, err := layout.LoadFromTar(ctx, src, layout.PackageLayoutOptions{Filter: filters.Empty()})
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
			require.Len(t, layers, 9)

			nonDeterministicLayers := []string{"zarf.yaml", "checksums.txt"}

			// get sbom layers - it appears as though the sbom layers are not deterministic
			sbomInspectLayers, err := remote.AssembleLayers(ctx, layoutExpected.Pkg.Components, false, zoci.SbomLayers)
			require.NoError(t, err)
			require.Len(t, sbomInspectLayers, 3)

			// get image layers
			expectedImageLayers := []string{"sha256:eda48e36dc18bbe4547311bdce8878f9e06b4bee032c85c4ff368bd53af6aecb",
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
		})
	}
}
