// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci_test contains tests for interacting with Zarf packages stored in OCI registries.
package zoci_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

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

type virtualImage struct {
	layer    ocispec.Descriptor
	config   ocispec.Descriptor
	manifest ocispec.Descriptor
}

type virtualPackage struct {
	registryAddr string
	packagePath  string
	image        virtualImage
}

// publishPackage loads a package from packagePath, publishes it to upstream/zarf-packages,
// and returns a connected Remote plus the package's components.
func publishPackage(ctx context.Context, t *testing.T, packagePath, upstream string) (*zoci.Remote, []v1alpha1.ZarfComponent) {
	t.Helper()
	pkgLayout, err := layout.LoadFromTar(ctx, packagePath, layout.PackageLayoutOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(pkgLayout.Pkg.Metadata.Name) }) //nolint:errcheck

	dstRef := registry.Reference{Registry: upstream, Repository: "zarf-packages"}
	packageRef, err := packager.PublishPackage(ctx, pkgLayout, dstRef, packager.PublishPackageOptions{
		RemoteOptions:  types.RemoteOptions{PlainHTTP: true},
		OCIConcurrency: 3,
	})
	require.NoError(t, err)

	platform := oci.PlatformForArch(pkgLayout.Pkg.Build.Architecture)
	r, err := zoci.NewRemote(ctx, packageRef.String(), platform, oci.WithPlainHTTP(true))
	require.NoError(t, err)
	return r, pkgLayout.Pkg.Components
}

// buildVirtualPackage pushes a virtual image to a fresh in-memory registry and builds a zarf
// package referencing it.
func buildVirtualPackage(ctx context.Context, t *testing.T) virtualPackage {
	t.Helper()
	upstreamAddr := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	imageRepo := testutil.NewRepo(t, upstreamAddr+"/fixtures/test-image")
	layerDesc := testutil.PushBlob(ctx, t, imageRepo, ocispec.MediaTypeImageLayer, testutil.RandomBytes(t, 256))
	configDesc := testutil.PushBlob(ctx, t, imageRepo, ocispec.MediaTypeImageConfig, []byte(`{"architecture":"amd64","os":"linux"}`))
	manifestDesc := testutil.PushManifest(ctx, t, imageRepo, configDesc, []ocispec.Descriptor{layerDesc})
	require.NoError(t, imageRepo.Tag(ctx, manifestDesc, "test"))
	imageRef := fmt.Sprintf("%s/fixtures/test-image:test", upstreamAddr)

	packagePath := createVirtualPackage(ctx, t, imageRef)
	return virtualPackage{
		registryAddr: upstreamAddr,
		packagePath:  packagePath,
		image: virtualImage{
			layer:    layerDesc,
			config:   configDesc,
			manifest: manifestDesc,
		},
	}
}

// createVirtualPackage creates a package with an in memory zarf yaml and a single virtual image for the provided ref
func createVirtualPackage(ctx context.Context, t *testing.T, imageRef string) string {
	t.Helper()
	pkgDefDir := writeVirtualPackageDef(t, imageRef)
	tmpdir := t.TempDir()
	packagePath, err := packager.Create(ctx, pkgDefDir, tmpdir, packager.CreateOptions{
		OCIConcurrency: 3,
		CachePath:      tmpdir,
		RemoteOptions:  types.RemoteOptions{PlainHTTP: true},
		SkipSBOM:       true,
	})
	require.NoError(t, err)
	return packagePath
}

func TestAssembleLayers(t *testing.T) {
	ctx := testutil.TestContext(t)
	pkg := buildVirtualPackage(ctx, t)
	remote, components := publishPackage(ctx, t, pkg.packagePath, pkg.registryAddr)

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

func TestAllPublishedLayersArePulled(t *testing.T) {
	ctx := testutil.TestContext(t)

	dir := t.TempDir()
	zarfYAML := `kind: ZarfPackageConfig
metadata:
  name: all-layers-test
  version: 0.0.1
  architecture: amd64
values:
  files:
    - values.yaml
  schema: values.schema.json
documentation:
  readme: README.md
components:
  - name: with-file
    required: true
    files:
      - source: data.txt
        target: data.txt
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "zarf.yaml"), []byte(zarfYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.txt"), []byte("hello\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "values.yaml"), []byte("foo: bar\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "values.schema.json"), []byte(`{"type":"object"}`), 0o644))

	tmpdir := t.TempDir()
	packagePath, err := packager.Create(ctx, dir, tmpdir, packager.CreateOptions{
		CachePath:      tmpdir,
		SigningKeyPath: "testdata/cosign.key",
	})
	require.NoError(t, err)

	upstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	remote, components := publishPackage(ctx, t, packagePath, upstream)

	// Verify that for each entry on the manifest there is an associated layer pulled by remote.AssembleLayers
	root, err := remote.FetchRoot(ctx)
	require.NoError(t, err)

	pulled, err := remote.AssembleLayers(ctx, components, zoci.GetAllLayerTypes()...)
	require.NoError(t, err)
	pulledDigests := map[string]struct{}{}
	for _, l := range pulled {
		pulledDigests[l.Digest.String()] = struct{}{}
	}

	for _, published := range root.Layers {
		_, ok := pulledDigests[published.Digest.String()]
		require.True(t, ok, "published layer %q (%s) is not pulled by AssembleLayers", published.Annotations[ocispec.AnnotationTitle], published.Digest)
	}
}

func buildAndPublishPackage(ctx context.Context, t *testing.T, imageRef, upstream string) *zoci.Remote {
	t.Helper()
	packagePath := createVirtualPackage(ctx, t, imageRef)
	r, _ := publishPackage(ctx, t, packagePath, upstream)
	return r
}

// expectedLayerPaths walks the OCI graph rooted at rootDigest in repo and returns every blob path that LayersFromImages should emit.
func expectedLayerPaths(ctx context.Context, t *testing.T, repo *remote.Repository, rootDigest string) []string {
	t.Helper()
	blobDir := path.Join(layout.ImagesDir, "blobs", "sha256")
	paths := []string{
		path.Join(layout.ImagesDir, "index.json"),
		path.Join(layout.ImagesDir, "oci-layout"),
	}
	var walk func(d string)
	walk = func(d string) {
		paths = append(paths, path.Join(blobDir, strings.TrimPrefix(d, "sha256:")))
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
		paths = append(paths, path.Join(blobDir, m.Config.Digest.Encoded()))
		for _, l := range m.Layers {
			paths = append(paths, path.Join(blobDir, l.Digest.Encoded()))
		}
	}
	walk(rootDigest)
	return paths
}

func pathsFromLayers(layers []ocispec.Descriptor) []string {
	out := make([]string, 0, len(layers))
	for _, l := range layers {
		out = append(out, l.Annotations[ocispec.AnnotationTitle])
	}
	return out
}

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

	r := buildAndPublishPackage(ctx, t, imageRef, upstream)
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

	r := buildAndPublishPackage(ctx, t, imageRef, upstream)
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

	r := buildAndPublishPackage(ctx, t, imageRef, upstream)
	layers, err := r.LayersFromImages(ctx, map[string]bool{imageRef: true})
	require.NoError(t, err)

	expected := expectedLayerPaths(ctx, t, testutil.NewRepo(t, upstream+"/fixtures/nested"), digest)
	actual := pathsFromLayers(layers)
	require.ElementsMatch(t, expected, actual)
	requireNoDuplicatePaths(t, actual)
}
