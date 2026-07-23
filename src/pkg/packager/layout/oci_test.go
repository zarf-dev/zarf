// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	godigest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"oras.land/oras-go/v2/errdef"
)

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
	annotations := AnnotationsFromMetadata(metadata)
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

// newTestLayout creates a minimal PackageLayout with a computed manifest.
// It writes a single known blob file and returns its contents so callers
// can verify Fetch returns the right bytes.
func newTestLayout(t *testing.T) (*PackageLayout, []byte) {
	t.Helper()
	dir := t.TempDir()

	blobContent := []byte("test blob content")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), blobContent, 0600))

	// checksums.txt is required by computeManifest; empty means all files are hashed on-demand.
	require.NoError(t, os.WriteFile(filepath.Join(dir, Checksums), []byte{}, 0600))

	// zarf.yaml is required; computeManifest reads it from disk for the OCI config.
	zarfYAML := "apiVersion: zarf.dev/v1alpha1\nkind: ZarfPackageConfig\nmetadata:\n  name: test-pkg\n  version: 1.0.0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ZarfYAML), []byte(zarfYAML), 0600))

	p := &PackageLayout{
		dirPath: dir,
		Pkg: v1alpha1.ZarfPackage{
			Metadata: v1alpha1.ZarfMetadata{Name: "test-pkg", Version: "1.0.0"},
			Build:    v1alpha1.ZarfBuildData{Architecture: "amd64"},
		},
	}
	require.NoError(t, p.computeManifest(context.Background()))
	return p, blobContent
}

func TestDigest(t *testing.T) {
	t.Parallel()
	p, _ := newTestLayout(t)

	d := p.Digest()
	assert.Equal(t, "sha256:25242bc565875477a9f691d8ce135b433bb014340a46b87113d977f0c08bd728", d, "digest should match expected precomputed digest")
}

func TestTotalSize(t *testing.T) {
	t.Parallel()

	empty := &PackageLayout{}
	assert.Equal(t, int64(0), empty.TotalSize(), "TotalSize should be 0 before manifest is computed")

	p, _ := newTestLayout(t)
	assert.Positive(t, p.TotalSize())
}

func TestComputeManifestDeterministic(t *testing.T) {
	t.Parallel()
	p, _ := newTestLayout(t)
	first := p.Digest()

	require.NoError(t, p.computeManifest(context.Background()))
	assert.Equal(t, first, p.Digest(), "repeated computeManifest calls should produce the same digest")
}

func TestResolve(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, _ := newTestLayout(t)

	t.Run("by manifest digest", func(t *testing.T) {
		desc, err := p.Resolve(ctx, p.Digest())
		require.NoError(t, err)
		assert.Equal(t, p.Digest(), desc.Digest.String())
	})

	t.Run("by package name", func(t *testing.T) {
		desc, err := p.Resolve(ctx, p.Pkg.Metadata.Name)
		require.NoError(t, err)
		assert.Equal(t, p.Digest(), desc.Digest.String())
	})

	t.Run("unknown reference returns ErrNotFound", func(t *testing.T) {
		_, err := p.Resolve(ctx, "sha256:0000000000000000000000000000000000000000000000000000000000000000")
		assert.ErrorIs(t, err, errdef.ErrNotFound)
	})

	t.Run("nil cache returns ErrNotFound", func(t *testing.T) {
		empty := &PackageLayout{}
		_, err := empty.Resolve(ctx, "anything")
		assert.ErrorIs(t, err, errdef.ErrNotFound)
	})
}

func TestFetch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, blobContent := newTestLayout(t)

	t.Run("manifest", func(t *testing.T) {
		r, err := p.Fetch(ctx, p.cache.desc)
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, p.cache.manifestJSON, got)
	})

	t.Run("config", func(t *testing.T) {
		r, err := p.Fetch(ctx, ocispec.Descriptor{Digest: p.cache.configDigest})
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, p.cache.configBytes, got)
	})

	t.Run("blob", func(t *testing.T) {
		// Use the deterministic digest of the known blob content written by newTestLayout.
		blobDigest := godigest.FromBytes(blobContent)
		require.Contains(t, p.cache.blobs, blobDigest, "test.txt digest should be in the blob cache")

		r, err := p.Fetch(ctx, ocispec.Descriptor{Digest: blobDigest})
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, blobContent, got)
	})

	t.Run("unknown descriptor returns ErrNotFound", func(t *testing.T) {
		unknown := ocispec.Descriptor{Digest: "sha256:0000000000000000000000000000000000000000000000000000000000000000"}
		_, err := p.Fetch(ctx, unknown)
		assert.ErrorIs(t, err, errdef.ErrNotFound)
	})

	t.Run("nil cache returns ErrNotFound", func(t *testing.T) {
		empty := &PackageLayout{}
		_, err := empty.Fetch(ctx, ocispec.Descriptor{})
		assert.ErrorIs(t, err, errdef.ErrNotFound)
	})
}

func TestExists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, _ := newTestLayout(t)

	t.Run("manifest", func(t *testing.T) {
		ok, err := p.Exists(ctx, p.cache.desc)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("config", func(t *testing.T) {
		ok, err := p.Exists(ctx, ocispec.Descriptor{Digest: p.cache.configDigest})
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("blob", func(t *testing.T) {
		var blobDigest godigest.Digest
		for d := range p.cache.blobs {
			blobDigest = d
			break
		}
		require.NotEmpty(t, blobDigest)

		ok, err := p.Exists(ctx, ocispec.Descriptor{Digest: blobDigest})
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("unknown returns false", func(t *testing.T) {
		unknown := ocispec.Descriptor{Digest: "sha256:0000000000000000000000000000000000000000000000000000000000000000"}
		ok, err := p.Exists(ctx, unknown)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("nil cache returns false", func(t *testing.T) {
		empty := &PackageLayout{}
		ok, err := empty.Exists(ctx, ocispec.Descriptor{})
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

func TestSetRegistryDigest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, _ := newTestLayout(t)

	require.NotEmpty(t, p.Digest())
	require.Positive(t, p.TotalSize())

	override := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	p.SetRegistryDigest(override)

	assert.Equal(t, override, p.Digest())
	assert.Equal(t, int64(0), p.TotalSize(), "cache should be nil after SetRegistryDigest")

	_, err := p.Resolve(ctx, override)
	assert.ErrorIs(t, err, errdef.ErrNotFound, "Resolve should return ErrNotFound after cache is cleared")
}
