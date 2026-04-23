// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci_test contains tests for interacting with Zarf packages stored in OCI registries.
package zoci_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"github.com/zarf-dev/zarf/src/pkg/packager"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
	_ "modernc.org/sqlite"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
)

func createRegistry(ctx context.Context, t *testing.T) registry.Reference {
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

func TestAssembleLayers(t *testing.T) {
	ctx := testutil.TestContext(t)

	remote, pkgLayout := publishAndConnect(ctx, t, "testdata/basic")
	components := pkgLayout.Pkg.Components

	nonDeterministicLayers := []string{"zarf.yaml", "checksums.txt"}
	expectedImageLayers := []string{
		"sha256:da324ac903c3287a9ab7f12d10fea0177251ca5d1aae156b293f042a722c414d",
		"sha256:18f0797eab35a4597c1e9624aa4f15fd91f6254e5538c1e0d193b2a95dd4acc6",
		"sha256:1c4eef651f65e2f7daee7ee785882ac164b02b78fb74503052a26dc061c90474",
		"sha256:aded1e1a5b3705116fa0a92ba074a5e0b0031647d9c315983ccba2ee5428ec8b",
		"sha256:f18232174bc91741fdf3da96d85011092101a032a93a388b79e99e69c2d5c870",
	}

	tests := []struct {
		name           string
		include        []zoci.LayerType
		expectedLen    int
		verifyDigests  bool
		expectedDigest []string
	}{
		{
			name:        "all layers (default)",
			include:     nil,
			expectedLen: 10,
		},
		{
			name:        "sbom layers",
			include:     []zoci.LayerType{zoci.SbomLayers},
			expectedLen: 3,
		},
		{
			name:           "image layers",
			include:        []zoci.LayerType{zoci.ImageLayers},
			expectedLen:    7,
			verifyDigests:  true,
			expectedDigest: expectedImageLayers,
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

			if tt.verifyDigests {
				for _, layer := range layers {
					if !slices.Contains(nonDeterministicLayers, layer.Annotations["org.opencontainers.image.title"]) {
						t.Logf("Layer: %s, Title: %s", layer.Digest.String(), layer.Annotations["org.opencontainers.image.title"])
						require.Contains(t, tt.expectedDigest, layer.Digest.String())
					}
				}
			}
		})
	}
}

// writePackageDef writes a minimal zarf.yaml + pod.yaml to a temp dir and
// returns the dir path. The image reference is baked into both files.
func writePackageDef(t *testing.T, arch, imageRef string) string {
	t.Helper()
	dir := t.TempDir()
	zarfYAML := fmt.Sprintf(`kind: ZarfPackageConfig
metadata:
  name: layers-from-images-test
  version: 0.0.1
  architecture: %s
components:
  - name: app
    required: true
    manifests:
      - name: app
        namespace: test
        files:
          - pod.yaml
    images:
      - %s
`, arch, imageRef)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "zarf.yaml"), []byte(zarfYAML), 0o644))
	pod := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: app
  namespace: test
spec:
  containers:
    - name: app
      image: %s
`, imageRef)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pod.yaml"), []byte(pod), 0o644))
	return dir
}

// buildAndPublishPackage builds a zarf package from the given image ref and
// publishes it to a fresh destination registry. Returns a connected Remote.
func buildAndPublishPackage(ctx context.Context, t *testing.T, arch, imageRef, upstream string) *zoci.Remote {
	t.Helper()

	pkgDefDir := writePackageDef(t, arch, imageRef)
	tmpdir := t.TempDir()

	packagePath, err := packager.Create(ctx, pkgDefDir, tmpdir, packager.CreateOptions{
		OCIConcurrency: 3,
		CachePath:      tmpdir,
		RemoteOptions:  types.RemoteOptions{PlainHTTP: true},
		// Image layers in these fixtures are random bytes, not real tarballs; syft can't read them.
		SkipSBOM: true,
	})
	require.NoError(t, err)

	pkgLayout, err := layout.LoadFromTar(ctx, packagePath, layout.PackageLayoutOptions{})
	require.NoError(t, err)

	dstRef := registry.Reference{
		Registry:   upstream,
		Repository: "zarf-packages",
	}
	packageRef, err := packager.PublishPackage(ctx, pkgLayout, dstRef, packager.PublishPackageOptions{
		RemoteOptions:  types.RemoteOptions{PlainHTTP: true},
		OCIConcurrency: 3,
	})
	require.NoError(t, err)

	platform := oci.PlatformForArch(pkgLayout.Pkg.Build.Architecture)
	remote, err := zoci.NewRemote(ctx, packageRef.String(), platform, oci.WithPlainHTTP(true))
	require.NoError(t, err)
	return remote
}

// expectedLayerPaths walks an OCI graph rooted at rootDigest in repo and returns every blob path
// (relative to the package root) that LayersFromImages should emit. Handles nested indexes.
func expectedLayerPaths(ctx context.Context, t *testing.T, repo *remote.Repository, rootDigest string) []string {
	t.Helper()
	paths := []string{layout.IndexPath, layout.OCILayoutPath}
	var walk func(d string)
	walk = func(d string) {
		paths = append(paths, filepath.Join(layout.ImagesBlobsDir, strings.TrimPrefix(d, "sha256:")))
		desc, body, err := oras.FetchBytes(ctx, repo, d, oras.DefaultFetchBytesOptions)
		require.NoError(t, err)
		if images.IsIndex(desc.MediaType) {
			var idx ocispec.Index
			require.NoError(t, json.Unmarshal(body, &idx))
			for _, c := range idx.Manifests {
				walk(c.Digest.String())
			}
			return
		}
		var m ocispec.Manifest
		require.NoError(t, json.Unmarshal(body, &m))
		paths = append(paths, filepath.Join(layout.ImagesBlobsDir, m.Config.Digest.Encoded()))
		for _, l := range m.Layers {
			paths = append(paths, filepath.Join(layout.ImagesBlobsDir, l.Digest.Encoded()))
		}
	}
	walk(rootDigest)
	return paths
}

// pathsFromLayers extracts the image.title annotation (relative blob path) from each descriptor
// returned by LayersFromImages.
func pathsFromLayers(layers []ocispec.Descriptor) []string {
	out := make([]string, 0, len(layers))
	for _, l := range layers {
		out = append(out, l.Annotations[ocispec.AnnotationTitle])
	}
	return out
}

// requireNoDuplicatePaths fails the test if any path appears twice — LayersFromImages advertises
// RemoveDuplicateDescriptors, this catches regressions of that behavior.
func requireNoDuplicatePaths(t *testing.T, paths []string) {
	t.Helper()
	seen := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		_, dup := seen[p]
		require.False(t, dup, "duplicate layer path in result: %s", p)
		seen[p] = struct{}{}
	}
}

func TestLayersFromImages_SingleArch(t *testing.T) {
	ctx := testutil.TestContext(t)
	upstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	digest := testutil.PushImage(ctx, t, upstream+"/fixtures/single", "test")
	imageRef := fmt.Sprintf("%s/fixtures/single:test@%s", upstream, digest)

	r := buildAndPublishPackage(ctx, t, "amd64", imageRef, upstream)
	layers, err := r.LayersFromImages(ctx, map[string]bool{imageRef: true})
	require.NoError(t, err)

	expected := expectedLayerPaths(ctx, t, testutil.NewRepo(t, upstream+"/fixtures/single"), digest)
	actual := pathsFromLayers(layers)
	require.ElementsMatch(t, expected, actual)
	requireNoDuplicatePaths(t, actual)
}

func TestLayersFromImages_MultiArch(t *testing.T) {
	ctx := testutil.TestContext(t)
	upstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	platforms := []ocispec.Platform{
		{OS: "linux", Architecture: "amd64"},
		{OS: "linux", Architecture: "arm64"},
	}
	digest := testutil.PushMultiArchIndex(ctx, t, upstream+"/fixtures/multi", "test", platforms)
	imageRef := fmt.Sprintf("%s/fixtures/multi:test@%s", upstream, digest)

	r := buildAndPublishPackage(ctx, t, "multi", imageRef, upstream)
	layers, err := r.LayersFromImages(ctx, map[string]bool{imageRef: true})
	require.NoError(t, err)

	expected := expectedLayerPaths(ctx, t, testutil.NewRepo(t, upstream+"/fixtures/multi"), digest)
	actual := pathsFromLayers(layers)
	require.ElementsMatch(t, expected, actual)
	requireNoDuplicatePaths(t, actual)
}

func TestLayersFromImages_NestedIndex(t *testing.T) {
	ctx := testutil.TestContext(t)
	upstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	platforms := []ocispec.Platform{
		{OS: "linux", Architecture: "amd64"},
		{OS: "linux", Architecture: "arm64"},
	}
	digest := testutil.PushNestedIndex(ctx, t, upstream+"/fixtures/nested", "test", platforms)
	imageRef := fmt.Sprintf("%s/fixtures/nested:test@%s", upstream, digest)

	r := buildAndPublishPackage(ctx, t, "multi", imageRef, upstream)
	layers, err := r.LayersFromImages(ctx, map[string]bool{imageRef: true})
	require.NoError(t, err)

	expected := expectedLayerPaths(ctx, t, testutil.NewRepo(t, upstream+"/fixtures/nested"), digest)
	actual := pathsFromLayers(layers)
	require.ElementsMatch(t, expected, actual)
	requireNoDuplicatePaths(t, actual)
}
