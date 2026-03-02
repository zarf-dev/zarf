// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

// writeBlobToDir writes content to pkgDir/images/blobs/sha256/<hex> and returns
// the blob's digest.
func writeBlobToDir(t *testing.T, pkgDir string, content []byte) digest.Digest {
	t.Helper()
	sum := sha256.Sum256(content)
	hex := fmt.Sprintf("%x", sum)
	blobsDir := filepath.Join(pkgDir, layout.ImagesBlobsDir)
	require.NoError(t, os.MkdirAll(blobsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(blobsDir, hex), content, 0o644))
	return digest.Digest("sha256:" + hex)
}

// writeIndexToDir writes idx as index.json under pkgDir/images/.
func writeIndexToDir(t *testing.T, pkgDir string, idx ocispec.Index) {
	t.Helper()
	b, err := json.Marshal(idx)
	require.NoError(t, err)
	imagesDir := filepath.Join(pkgDir, layout.ImagesDir)
	require.NoError(t, os.MkdirAll(imagesDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(imagesDir, layout.IndexJSON), b, 0o644))
}

// blobPath returns the package-relative path for a blob digest.
func blobPath(dgst digest.Digest) string {
	return layout.ImagesBlobsDir + "/" + dgst.Encoded()
}

// writeManifestBlob marshals mf, writes it as a blob, and returns its digest.
func writeManifestBlob(t *testing.T, pkgDir string, mf ocispec.Manifest) digest.Digest {
	t.Helper()
	b, err := json.Marshal(mf)
	require.NoError(t, err)
	return writeBlobToDir(t, pkgDir, b)
}

func TestBuildBlobMediaTypes(t *testing.T) {
	t.Parallel()

	t.Run("no images directory returns only seeded entries", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		result, err := buildBlobMediaTypes(dir)
		require.NoError(t, err)
		require.Equal(t, map[string]string{
			layout.OCILayoutPath: ocispec.MediaTypeLayoutHeader,
			layout.IndexPath:     ocispec.MediaTypeImageIndex,
		}, result)
	})

	t.Run("empty index returns only seeded entries", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeIndexToDir(t, dir, ocispec.Index{})
		result, err := buildBlobMediaTypes(dir)
		require.NoError(t, err)
		require.Equal(t, map[string]string{
			layout.OCILayoutPath: ocispec.MediaTypeLayoutHeader,
			layout.IndexPath:     ocispec.MediaTypeImageIndex,
		}, result)
	})

	t.Run("single manifest with gzip layer", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		layerDgst := writeBlobToDir(t, dir, []byte("layer data"))
		configDgst := writeBlobToDir(t, dir, []byte("config data"))

		mf := ocispec.Manifest{
			Config: ocispec.Descriptor{
				MediaType: ocispec.MediaTypeImageConfig,
				Digest:    configDgst,
			},
			Layers: []ocispec.Descriptor{
				{MediaType: ocispec.MediaTypeImageLayerGzip, Digest: layerDgst},
			},
		}
		manifestDgst := writeManifestBlob(t, dir, mf)

		writeIndexToDir(t, dir, ocispec.Index{
			Manifests: []ocispec.Descriptor{
				{MediaType: ocispec.MediaTypeImageManifest, Digest: manifestDgst},
			},
		})

		result, err := buildBlobMediaTypes(dir)
		require.NoError(t, err)
		require.Equal(t, map[string]string{
			layout.OCILayoutPath:   ocispec.MediaTypeLayoutHeader,
			layout.IndexPath:       ocispec.MediaTypeImageIndex,
			blobPath(manifestDgst): ocispec.MediaTypeImageManifest,
			blobPath(configDgst):   ocispec.MediaTypeImageConfig,
			blobPath(layerDgst):    ocispec.MediaTypeImageLayerGzip,
		}, result)
	})

	t.Run("single manifest with zstd layer preserves media type", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		layerDgst := writeBlobToDir(t, dir, []byte("zstd layer"))
		configDgst := writeBlobToDir(t, dir, []byte("zstd config"))

		mf := ocispec.Manifest{
			Config: ocispec.Descriptor{MediaType: ocispec.MediaTypeImageConfig, Digest: configDgst},
			Layers: []ocispec.Descriptor{
				{MediaType: ocispec.MediaTypeImageLayerZstd, Digest: layerDgst},
			},
		}
		manifestDgst := writeManifestBlob(t, dir, mf)

		writeIndexToDir(t, dir, ocispec.Index{
			Manifests: []ocispec.Descriptor{
				{MediaType: ocispec.MediaTypeImageManifest, Digest: manifestDgst},
			},
		})

		result, err := buildBlobMediaTypes(dir)
		require.NoError(t, err)
		require.Equal(t, ocispec.MediaTypeImageLayerZstd, result[blobPath(layerDgst)])
	})

	t.Run("multiple layers with different types", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		gzipDgst := writeBlobToDir(t, dir, []byte("gzip layer"))
		zstdDgst := writeBlobToDir(t, dir, []byte("zstd layer"))
		configDgst := writeBlobToDir(t, dir, []byte("config"))

		mf := ocispec.Manifest{
			Config: ocispec.Descriptor{MediaType: ocispec.MediaTypeImageConfig, Digest: configDgst},
			Layers: []ocispec.Descriptor{
				{MediaType: ocispec.MediaTypeImageLayerGzip, Digest: gzipDgst},
				{MediaType: ocispec.MediaTypeImageLayerZstd, Digest: zstdDgst},
			},
		}
		manifestDgst := writeManifestBlob(t, dir, mf)

		writeIndexToDir(t, dir, ocispec.Index{
			Manifests: []ocispec.Descriptor{
				{MediaType: ocispec.MediaTypeImageManifest, Digest: manifestDgst},
			},
		})

		result, err := buildBlobMediaTypes(dir)
		require.NoError(t, err)
		require.Equal(t, ocispec.MediaTypeImageLayerGzip, result[blobPath(gzipDgst)])
		require.Equal(t, ocispec.MediaTypeImageLayerZstd, result[blobPath(zstdDgst)])
		require.Equal(t, ocispec.MediaTypeImageConfig, result[blobPath(configDgst)])
	})

	t.Run("multiple manifests all collected", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		layer1 := writeBlobToDir(t, dir, []byte("layer 1"))
		config1 := writeBlobToDir(t, dir, []byte("config 1"))
		manifest1 := writeManifestBlob(t, dir, ocispec.Manifest{
			Config: ocispec.Descriptor{MediaType: ocispec.MediaTypeImageConfig, Digest: config1},
			Layers: []ocispec.Descriptor{
				{MediaType: ocispec.MediaTypeImageLayerGzip, Digest: layer1},
			},
		})

		layer2 := writeBlobToDir(t, dir, []byte("layer 2"))
		config2 := writeBlobToDir(t, dir, []byte("config 2"))
		manifest2 := writeManifestBlob(t, dir, ocispec.Manifest{
			Config: ocispec.Descriptor{MediaType: ocispec.MediaTypeImageConfig, Digest: config2},
			Layers: []ocispec.Descriptor{
				{MediaType: ocispec.MediaTypeImageLayerGzip, Digest: layer2},
			},
		})

		writeIndexToDir(t, dir, ocispec.Index{
			Manifests: []ocispec.Descriptor{
				{MediaType: ocispec.MediaTypeImageManifest, Digest: manifest1},
				{MediaType: ocispec.MediaTypeImageManifest, Digest: manifest2},
			},
		})

		result, err := buildBlobMediaTypes(dir)
		require.NoError(t, err)
		require.Equal(t, ocispec.MediaTypeImageManifest, result[blobPath(manifest1)])
		require.Equal(t, ocispec.MediaTypeImageManifest, result[blobPath(manifest2)])
		require.Equal(t, ocispec.MediaTypeImageConfig, result[blobPath(config1)])
		require.Equal(t, ocispec.MediaTypeImageConfig, result[blobPath(config2)])
		require.Equal(t, ocispec.MediaTypeImageLayerGzip, result[blobPath(layer1)])
		require.Equal(t, ocispec.MediaTypeImageLayerGzip, result[blobPath(layer2)])
	})

	t.Run("index entry with empty media type not added to map", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		configDgst := writeBlobToDir(t, dir, []byte("config"))
		layerDgst := writeBlobToDir(t, dir, []byte("layer"))

		mf := ocispec.Manifest{
			Config: ocispec.Descriptor{MediaType: ocispec.MediaTypeImageConfig, Digest: configDgst},
			Layers: []ocispec.Descriptor{
				{MediaType: ocispec.MediaTypeImageLayerGzip, Digest: layerDgst},
			},
		}
		manifestDgst := writeManifestBlob(t, dir, mf)

		// Index entry has no media type set.
		writeIndexToDir(t, dir, ocispec.Index{
			Manifests: []ocispec.Descriptor{
				{MediaType: "", Digest: manifestDgst},
			},
		})

		result, err := buildBlobMediaTypes(dir)
		require.NoError(t, err)
		// The manifest digest is NOT in the map (no media type on index entry).
		require.NotContains(t, result, blobPath(manifestDgst))
		// But config and layers ARE in the map since the manifest blob was parsed.
		require.Equal(t, ocispec.MediaTypeImageConfig, result[blobPath(configDgst)])
		require.Equal(t, ocispec.MediaTypeImageLayerGzip, result[blobPath(layerDgst)])
	})

	t.Run("config with empty media type not added to map", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		configDgst := writeBlobToDir(t, dir, []byte("config"))
		mf := ocispec.Manifest{
			Config: ocispec.Descriptor{MediaType: "", Digest: configDgst},
		}
		manifestDgst := writeManifestBlob(t, dir, mf)

		writeIndexToDir(t, dir, ocispec.Index{
			Manifests: []ocispec.Descriptor{
				{MediaType: ocispec.MediaTypeImageManifest, Digest: manifestDgst},
			},
		})

		result, err := buildBlobMediaTypes(dir)
		require.NoError(t, err)
		require.NotContains(t, result, blobPath(configDgst))
	})

	t.Run("layer with empty media type not added to map", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		layerDgst := writeBlobToDir(t, dir, []byte("layer"))
		configDgst := writeBlobToDir(t, dir, []byte("config"))
		mf := ocispec.Manifest{
			Config: ocispec.Descriptor{MediaType: ocispec.MediaTypeImageConfig, Digest: configDgst},
			Layers: []ocispec.Descriptor{
				{MediaType: "", Digest: layerDgst},
			},
		}
		manifestDgst := writeManifestBlob(t, dir, mf)

		writeIndexToDir(t, dir, ocispec.Index{
			Manifests: []ocispec.Descriptor{
				{MediaType: ocispec.MediaTypeImageManifest, Digest: manifestDgst},
			},
		})

		result, err := buildBlobMediaTypes(dir)
		require.NoError(t, err)
		require.NotContains(t, result, blobPath(layerDgst))
	})

	t.Run("invalid index JSON returns error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		imagesDir := filepath.Join(dir, layout.ImagesDir)
		require.NoError(t, os.MkdirAll(imagesDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(imagesDir, layout.IndexJSON), []byte("not json"), 0o644))

		_, err := buildBlobMediaTypes(dir)
		require.Error(t, err)
	})

	t.Run("invalid manifest JSON returns error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		// Write an invalid blob (not JSON) as the manifest.
		invalidDgst := writeBlobToDir(t, dir, []byte("not a manifest"))

		writeIndexToDir(t, dir, ocispec.Index{
			Manifests: []ocispec.Descriptor{
				{MediaType: ocispec.MediaTypeImageManifest, Digest: invalidDgst},
			},
		})

		_, err := buildBlobMediaTypes(dir)
		require.Error(t, err)
	})
}

func TestAnnotationsFromMetadata(t *testing.T) {
	t.Parallel()

	metadata := v1alpha1.ZarfMetadata{
		Name:          "foo",
		Description:   "bar",
		URL:           "https://example.com",
		Authors:       "Zarf",
		Documentation: "documentation",
		Source:        "source",
		Vendor:        "vendor",
		Annotations: map[string]string{
			"org.opencontainers.image.title": "overridden",
			"org.opencontainers.image.new":   "new-field",
		},
	}
	annotations := annotationsFromMetadata(metadata)
	expectedAnnotations := map[string]string{
		"org.opencontainers.image.title":         "overridden",
		"org.opencontainers.image.description":   "bar",
		"org.opencontainers.image.url":           "https://example.com",
		"org.opencontainers.image.authors":       "Zarf",
		"org.opencontainers.image.documentation": "documentation",
		"org.opencontainers.image.source":        "source",
		"org.opencontainers.image.vendor":        "vendor",
		"org.opencontainers.image.new":           "new-field",
	}
	require.Equal(t, expectedAnnotations, annotations)
}
