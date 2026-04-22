// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci_test contains tests for interacting with Zarf packages stored in OCI registries.
package zoci_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
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

// setupUpstreamRegistry starts a plaintext in-memory registry that doubles as
// both the source of fixture images pulled during zarf create and the
// destination for publishing the resulting zarf package.
func setupUpstreamRegistry(ctx context.Context, t *testing.T) string {
	t.Helper()
	port, err := helpers.GetAvailablePort()
	require.NoError(t, err)
	return testutil.SetupInMemoryRegistry(ctx, t, port)
}

// newRepo returns an oras-go Repository configured for plaintext HTTP.
func newRepo(t *testing.T, refStr string) *remote.Repository {
	t.Helper()
	repo, err := remote.NewRepository(refStr)
	require.NoError(t, err)
	repo.PlainHTTP = true
	return repo
}

// randomBytes returns n cryptographically random bytes; used as synthetic layer
// content that hashes differently on every test run.
func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b
}

// pushBlob pushes raw bytes and returns the resulting descriptor.
func pushBlob(ctx context.Context, t *testing.T, repo *remote.Repository, mediaType string, data []byte) ocispec.Descriptor {
	t.Helper()
	desc := ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    digest.FromBytes(data),
		Size:      int64(len(data)),
	}
	// Push is a no-op if the blob already exists.
	if exists, err := repo.Exists(ctx, desc); err == nil && exists {
		return desc
	}
	require.NoError(t, repo.Push(ctx, desc, bytes.NewReader(data)))
	return desc
}

// pushManifest constructs an image manifest pointing at the given config and
// layers, pushes it, and returns its descriptor.
func pushManifest(ctx context.Context, t *testing.T, repo *remote.Repository, config ocispec.Descriptor, layers []ocispec.Descriptor) ocispec.Descriptor {
	t.Helper()
	manifest := ocispec.Manifest{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    config,
		Layers:    layers,
	}
	body, err := json.Marshal(manifest)
	require.NoError(t, err)
	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.FromBytes(body),
		Size:      int64(len(body)),
	}
	require.NoError(t, repo.Push(ctx, desc, bytes.NewReader(body)))
	return desc
}

// pushIndex builds and pushes an OCI image index referencing the given child
// descriptors. Children may themselves be manifests or indexes — nesting is
// supported by the OCI spec and exercised in the nested-index test below.
func pushIndex(ctx context.Context, t *testing.T, repo *remote.Repository, children []ocispec.Descriptor) ocispec.Descriptor {
	t.Helper()
	idx := ocispec.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: children,
	}
	body, err := json.Marshal(idx)
	require.NoError(t, err)
	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageIndex,
		Digest:    digest.FromBytes(body),
		Size:      int64(len(body)),
	}
	require.NoError(t, repo.Push(ctx, desc, bytes.NewReader(body)))
	return desc
}

// pushSinglePlatformImage creates a config blob, a single layer, and a manifest
// that references both. The config embeds the arch so two platforms produce
// distinct config blobs. Returns the manifest descriptor.
func pushSinglePlatformImage(ctx context.Context, t *testing.T, repo *remote.Repository, arch string) ocispec.Descriptor {
	t.Helper()
	layer := pushBlob(ctx, t, repo, ocispec.MediaTypeImageLayer, randomBytes(t, 64))
	configJSON := fmt.Sprintf(`{"architecture":%q,"os":"linux","rootfs":{"type":"layers","diff_ids":[]}}`, arch)
	config := pushBlob(ctx, t, repo, ocispec.MediaTypeImageConfig, []byte(configJSON))
	return pushManifest(ctx, t, repo, config, []ocispec.Descriptor{layer})
}

// pushSyntheticImage pushes a single-manifest image and tags it; returns the
// manifest digest.
func pushSyntheticImage(ctx context.Context, t *testing.T, repoRef, tag string) string {
	t.Helper()
	repo := newRepo(t, repoRef)
	desc := pushSinglePlatformImage(ctx, t, repo, "amd64")
	require.NoError(t, repo.Tag(ctx, desc, tag))
	return desc.Digest.String()
}

// pushSyntheticIndex pushes a multi-arch OCI image index whose children are
// single-platform manifests. Returns the index digest.
func pushSyntheticIndex(ctx context.Context, t *testing.T, repoRef, tag string, platforms int) string {
	t.Helper()
	repo := newRepo(t, repoRef)
	archs := []string{"amd64", "arm64", "arm"}
	children := make([]ocispec.Descriptor, 0, platforms)
	for i := range platforms {
		arch := archs[i%len(archs)]
		desc := pushSinglePlatformImage(ctx, t, repo, arch)
		desc.Platform = &ocispec.Platform{
			Architecture: arch,
			OS:           "linux",
		}
		children = append(children, desc)
	}
	idx := pushIndex(ctx, t, repo, children)
	require.NoError(t, repo.Tag(ctx, idx, tag))
	return idx.Digest.String()
}

// pushSyntheticNestedIndex pushes an OCI image index whose only child is itself
// an image index (of `platforms` single-platform children). Returns the outer
// index digest.
func pushSyntheticNestedIndex(ctx context.Context, t *testing.T, repoRef, tag string, platforms int) string {
	t.Helper()
	repo := newRepo(t, repoRef)
	archs := []string{"amd64", "arm64"}
	inner := make([]ocispec.Descriptor, 0, platforms)
	for i := range platforms {
		arch := archs[i%len(archs)]
		desc := pushSinglePlatformImage(ctx, t, repo, arch)
		desc.Platform = &ocispec.Platform{
			Architecture: arch,
			OS:           "linux",
		}
		inner = append(inner, desc)
	}
	innerIdx := pushIndex(ctx, t, repo, inner)
	outerIdx := pushIndex(ctx, t, repo, []ocispec.Descriptor{innerIdx})
	require.NoError(t, repo.Tag(ctx, outerIdx, tag))
	return outerIdx.Digest.String()
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
		// Synthetic image layers are not real tarballs; syft can't read them.
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
	t.Cleanup(func() {
		os.Remove(pkgLayout.Pkg.Metadata.Name) //nolint: errcheck
	})

	platform := oci.PlatformForArch(pkgLayout.Pkg.Build.Architecture)
	remote, err := zoci.NewRemote(ctx, packageRef.String(), platform, oci.WithPlainHTTP(true))
	require.NoError(t, err)
	return remote
}

func TestLayersFromImages_SingleArch(t *testing.T) {
	ctx := testutil.TestContext(t)
	upstream := setupUpstreamRegistry(ctx, t)
	digest := pushSyntheticImage(ctx, t, upstream+"/fixtures/single", "test")
	imageRef := fmt.Sprintf("%s/fixtures/single:test@%s", upstream, digest)

	remote := buildAndPublishPackage(ctx, t, "amd64", imageRef, upstream)
	layers, err := remote.LayersFromImages(ctx, map[string]bool{imageRef: true})
	require.NoError(t, err)

	// Expected blob paths for a single-manifest image:
	//   - images/index.json
	//   - images/oci-layout
	//   - manifest blob
	//   - config blob
	//   - layer blob
	require.Len(t, layers, 5)
}

func TestLayersFromImages_MultiArch(t *testing.T) {
	ctx := testutil.TestContext(t)
	upstream := setupUpstreamRegistry(ctx, t)
	const platforms = 2
	digest := pushSyntheticIndex(ctx, t, upstream+"/fixtures/multi", "test", platforms)
	imageRef := fmt.Sprintf("%s/fixtures/multi:test@%s", upstream, digest)

	remote := buildAndPublishPackage(ctx, t, "multi", imageRef, upstream)
	layers, err := remote.LayersFromImages(ctx, map[string]bool{imageRef: true})
	require.NoError(t, err)

	// Expected blob paths for a multi-arch index with N single-arch children, 1 layer each:
	//   - images/index.json
	//   - images/oci-layout
	//   - root index blob
	//   - per platform: manifest blob + config blob + layer blob
	expected := 2 + 1 + platforms*3
	require.Len(t, layers, expected)
}

func TestLayersFromImages_NestedIndex(t *testing.T) {
	ctx := testutil.TestContext(t)
	upstream := setupUpstreamRegistry(ctx, t)
	const platforms = 2
	digest := pushSyntheticNestedIndex(ctx, t, upstream+"/fixtures/nested", "test", platforms)
	imageRef := fmt.Sprintf("%s/fixtures/nested:test@%s", upstream, digest)

	remote := buildAndPublishPackage(ctx, t, "multi", imageRef, upstream)
	layers, err := remote.LayersFromImages(ctx, map[string]bool{imageRef: true})
	require.NoError(t, err)

	// Expected blob paths for an outer index wrapping an inner multi-arch index:
	//   - images/index.json
	//   - images/oci-layout
	//   - outer index blob
	//   - inner index blob
	//   - per platform in inner: manifest blob + config blob + layer blob
	expected := 2 + 1 + 1 + platforms*3
	require.Len(t, layers, expected)
}
