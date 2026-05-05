// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci_test contains tests for interacting with Zarf packages stored in OCI registries.
package zoci_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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

func createRegistry(ctx context.Context, t *testing.T) registry.Reference {
	t.Helper()
	return registry.Reference{
		Registry:   testutil.SetupInMemoryRegistryDynamic(ctx, t),
		Repository: "my-namespace",
	}
}

// publishAndConnect creates a package from srcPath, publishes it to a fresh registry,
// and returns a connected Remote along with the loaded PackageLayout.
func publishAndConnect(ctx context.Context, t *testing.T, srcPath string) (*zoci.Remote, *layout.PackageLayout) {
	t.Helper()
	registryRef := createRegistry(ctx, t)
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

// writeVirtualPackageDef writes a minimal zarf package definition that references imageRef.
func writeVirtualPackageDef(t *testing.T, imageRef string) string {
	t.Helper()
	dir := t.TempDir()
	zarfYAML := fmt.Sprintf(`kind: ZarfPackageConfig
metadata:
  name: assemble-layers-test
  version: 0.0.1
  architecture: amd64
documentation:
  readme: README.md
components:
  - name: alpine
    required: true
    manifests:
      - name: alpine
        namespace: test
        files:
          - pod.yaml
    images:
      - %s
`, imageRef)
	pod := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
    - name: test
      image: %s
`, imageRef)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "zarf.yaml"), []byte(zarfYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pod.yaml"), []byte(pod), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644))
	return dir
}

// virtualImage holds descriptors of the image pushed by buildVirtualPackage so callers can
// assert the package layers reference these exact blobs.
type virtualImage struct {
	layer    ocispec.Descriptor
	config   ocispec.Descriptor
	manifest ocispec.Descriptor
}

// virtualPackage bundles the in-memory registry address, built package tar path, build tmpdir,
// and image descriptors returned by buildVirtualPackage.
type virtualPackage struct {
	registryAddr string
	packagePath  string
	tmpdir       string
	image        virtualImage
}

// buildVirtualPackage pushes a virtual image to a fresh in-memory registry and runs
// packager.Create against a generated package def. SBOM is skipped as the image has random bytes
func buildVirtualPackage(ctx context.Context, t *testing.T) virtualPackage {
	t.Helper()
	upstreamAddr := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	imageRepo := testutil.NewRepo(t, upstreamAddr+"/fixtures/test-image")
	layerDesc := testutil.PushBlob(ctx, t, imageRepo, ocispec.MediaTypeImageLayer, testutil.RandomBytes(t, 256))
	configDesc := testutil.PushBlob(ctx, t, imageRepo, ocispec.MediaTypeImageConfig, []byte(`{"architecture":"amd64","os":"linux"}`))
	manifestDesc := testutil.PushManifest(ctx, t, imageRepo, configDesc, []ocispec.Descriptor{layerDesc})
	require.NoError(t, imageRepo.Tag(ctx, manifestDesc, "test"))
	imageRef := fmt.Sprintf("%s/fixtures/test-image:test", upstreamAddr)

	pkgDefDir := writeVirtualPackageDef(t, imageRef)
	tmpdir := t.TempDir()
	packagePath, err := packager.Create(ctx, pkgDefDir, tmpdir, packager.CreateOptions{
		CachePath:     tmpdir,
		RemoteOptions: types.RemoteOptions{PlainHTTP: true},
		SkipSBOM:      true, // random-bytes layer can't be syft-scanned
	})
	require.NoError(t, err)
	return virtualPackage{
		registryAddr: upstreamAddr,
		packagePath:  packagePath,
		tmpdir:       tmpdir,
		image: virtualImage{
			layer:    layerDesc,
			config:   configDesc,
			manifest: manifestDesc,
		},
	}
}

func TestAssembleLayers(t *testing.T) {
	ctx := testutil.TestContext(t)
	pkg := buildVirtualPackage(ctx, t)

	pkgLayout, err := layout.LoadFromTar(ctx, pkg.packagePath, layout.PackageLayoutOptions{})
	require.NoError(t, err)

	registryRef := registry.Reference{Registry: pkg.registryAddr, Repository: "zarf-packages"}
	packageRef, err := packager.PublishPackage(ctx, pkgLayout, registryRef, packager.PublishPackageOptions{
		RemoteOptions:  types.RemoteOptions{PlainHTTP: true},
		OCIConcurrency: 3,
	})
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(pkgLayout.Pkg.Metadata.Name) }) //nolint:errcheck

	cacheModifier, err := zoci.GetOCICacheModifier(ctx, pkg.tmpdir)
	require.NoError(t, err)
	platform := oci.PlatformForArch(pkgLayout.Pkg.Build.Architecture)
	remote, err := zoci.NewRemote(ctx, packageRef.String(), platform, append([]oci.Modifier{oci.WithPlainHTTP(true)}, cacheModifier)...)
	require.NoError(t, err)

	components := pkgLayout.Pkg.Components

	tests := []struct {
		name        string
		include     []zoci.LayerType
		expectedLen int
	}{
		{
			name:        "all layers (default)",
			include:     nil,
			expectedLen: 9,
		},
		{
			name:        "image layers",
			include:     []zoci.LayerType{zoci.ImageLayers},
			expectedLen: 7,
		},
		{
			name:        "component layers",
			include:     []zoci.LayerType{zoci.ComponentLayers},
			expectedLen: 3,
		},
		{
			name:        "documentation layers",
			include:     []zoci.LayerType{zoci.DocLayers},
			expectedLen: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layers, err := remote.AssembleLayers(ctx, components, tt.include...)
			require.NoError(t, err)
			require.Len(t, layers, tt.expectedLen)
		})
	}

	// Verify image-walking logic against known digests instead of upstream-drifting ones.
	imageLayers, err := remote.AssembleLayers(ctx, components, zoci.ImageLayers)
	require.NoError(t, err)
	digests := map[string]struct{}{}
	for _, l := range imageLayers {
		digests[l.Digest.String()] = struct{}{}
	}
	require.Contains(t, digests, pkg.image.manifest.Digest.String(), "image manifest blob present")
	require.Contains(t, digests, pkg.image.config.Digest.String(), "image config blob present")
	require.Contains(t, digests, pkg.image.layer.Digest.String(), "image layer blob present")
}
