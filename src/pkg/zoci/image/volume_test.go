// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package image

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content/oci"

	"github.com/zarf-dev/zarf/src/test/testutil"
)

func newTestVolume(t *testing.T) *Volume {
	t.Helper()
	iv, err := New(t.TempDir(), "linux", "amd64")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, iv.Clean()) })
	return iv
}

func TestNew(t *testing.T) {
	t.Parallel()

	iv, err := New(t.TempDir(), "linux", "amd64")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, iv.Clean()) })

	require.NotNil(t, iv.Store())
	require.DirExists(t, iv.tmp)
	require.Equal(t, "linux", iv.config.Platform.OS)
	require.Equal(t, "amd64", iv.config.Platform.Architecture)
	require.Equal(t, static, *iv.config.Created)
	require.Empty(t, iv.config.History)
	require.Equal(t, "layers", iv.config.RootFS.Type)
	require.Empty(t, iv.config.RootFS.DiffIDs)
	require.Empty(t, iv.layers)
}

func TestVolumeAddFile(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	iv := newTestVolume(t)

	srcDir := t.TempDir()
	content := []byte("hello world")
	filePath := filepath.Join(srcDir, "sub", "hello.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o755))
	require.NoError(t, os.WriteFile(filePath, content, 0o644))

	desc, err := iv.AddFile(ctx, srcDir, filePath)
	require.NoError(t, err)

	require.Equal(t, ocispec.MediaTypeImageLayer, desc.MediaType)
	require.Equal(t, "sub/hello.txt", desc.Annotations[ocispec.AnnotationTitle])
	require.Equal(t, format, desc.Annotations[ocispec.AnnotationCreated])
	require.Positive(t, desc.Size)
	require.NotEmpty(t, desc.Digest)

	exists, err := iv.Store().Exists(ctx, desc)
	require.NoError(t, err)
	require.True(t, exists)

	require.Equal(t, []ocispec.Descriptor{desc}, iv.layers)
	require.Equal(t, []digest.Digest{desc.Digest}, iv.config.RootFS.DiffIDs)
	require.Len(t, iv.config.History, 1)
	require.Equal(t, "ADD sub/hello.txt /", iv.config.History[0].CreatedBy)
	require.Equal(t, "dev.zarf.zoci.volume.v0", iv.config.History[0].Comment)

	assertLayerTarMatches(ctx, t, iv.Store(), desc, "sub/hello.txt", content)
}

func TestVolumeAddFileAccumulatesLayers(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	iv := newTestVolume(t)
	srcDir := t.TempDir()

	var descs []ocispec.Descriptor
	for _, name := range []string{"a.txt", "b.txt"} {
		p := filepath.Join(srcDir, name)
		require.NoError(t, os.WriteFile(p, []byte("content-"+name), 0o644))

		desc, err := iv.AddFile(ctx, srcDir, p)
		require.NoError(t, err)
		descs = append(descs, desc)
	}

	require.Equal(t, descs, iv.layers)
	require.Len(t, iv.config.RootFS.DiffIDs, 2)
	require.Len(t, iv.config.History, 2)
}

func TestVolumeAddDirectory(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	iv := newTestVolume(t)
	srcDir := t.TempDir()

	files := map[string]string{
		"a.txt":            "top level",
		"sub/b.txt":        "one level deep",
		"sub/nested/c.txt": "two levels deep",
	}
	for rel, content := range files {
		p := filepath.Join(srcDir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}

	const ref = "test:latest"
	err := iv.AddDirectory(ctx, srcDir, ref)
	require.NoError(t, err)

	require.Len(t, iv.layers, len(files))
	require.Len(t, iv.config.RootFS.DiffIDs, len(files))
	require.Len(t, iv.config.History, len(files))

	manifestDesc, err := iv.Store().Resolve(ctx, ref)
	require.NoError(t, err)
	require.Equal(t, ocispec.MediaTypeImageManifest, manifestDesc.MediaType)

	seen := map[string]bool{}
	for _, desc := range iv.layers {
		title := desc.Annotations[ocispec.AnnotationTitle]
		require.Contains(t, files, title)
		seen[title] = true

		exists, err := iv.Store().Exists(ctx, desc)
		require.NoError(t, err)
		require.True(t, exists)

		assertLayerTarMatches(ctx, t, iv.Store(), desc, title, []byte(files[title]))
	}
	require.Len(t, seen, len(files))
}

func TestVolumeAddDirectoryEmpty(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	iv := newTestVolume(t)
	srcDir := t.TempDir()

	err := iv.AddDirectory(ctx, srcDir, "test:latest")
	require.NoError(t, err)
	require.Empty(t, iv.layers)
	require.Empty(t, iv.config.RootFS.DiffIDs)
}

func TestVolumeClean(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	iv, err := New(t.TempDir(), "linux", "amd64")
	require.NoError(t, err)
	require.DirExists(t, iv.tmp)

	require.NoError(t, iv.Clean())
	require.NoDirExists(t, iv.tmp)

	srcDir := t.TempDir()
	p := filepath.Join(srcDir, "a.txt")
	require.NoError(t, os.WriteFile(p, []byte("a"), 0o644))

	_, err = iv.AddFile(ctx, srcDir, p)
	require.ErrorIs(t, err, fs.ErrNotExist, "AddFile should fail once Clean has removed the temp workspace")
}

func TestVolumeArchive(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	iv := newTestVolume(t)

	srcDir := t.TempDir()
	content := []byte("hello world")
	filePath := filepath.Join(srcDir, "hello.txt")
	require.NoError(t, os.WriteFile(filePath, content, 0o644))

	desc, err := iv.AddFile(ctx, srcDir, filePath)
	require.NoError(t, err)

	a := iv.Archive()

	info, err := a.Info(ctx, desc.Digest)
	require.NoError(t, err)
	require.Equal(t, desc.Digest, info.Digest)
	require.Equal(t, desc.Size, info.Size)

	ra, err := a.ReaderAt(ctx, desc)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ra.Close()) })
	require.Equal(t, desc.Size, ra.Size())

	buf := make([]byte, ra.Size())
	_, err = ra.ReadAt(buf, 0)
	require.NoError(t, err)

	tr := tar.NewReader(bytes.NewReader(buf))
	hdr, err := tr.Next()
	require.NoError(t, err)
	require.Equal(t, "hello.txt", hdr.Name)

	got, err := io.ReadAll(tr)
	require.NoError(t, err)
	require.Equal(t, content, got)
}

func TestVolumeAddFileCompression(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	content := []byte("hello world, this is a payload that is worth compressing")

	type result struct {
		desc   ocispec.Descriptor
		diffID digest.Digest
	}
	results := make(map[VolumeCompression]result)

	for _, compression := range []VolumeCompression{
		VolumeCompressionUncompressed,
		VolumeCompressionGzip,
		VolumeCompressionZstd,
	} {
		iv, err := New(t.TempDir(), "linux", "amd64")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, iv.Clean()) })
		iv.Compression = compression

		srcDir := t.TempDir()
		p := filepath.Join(srcDir, "hello.txt")
		require.NoError(t, os.WriteFile(p, content, 0o644))

		desc, err := iv.AddFile(ctx, srcDir, p)
		require.NoError(t, err, "compression %s", compression)

		exists, err := iv.Store().Exists(ctx, desc)
		require.NoError(t, err)
		require.True(t, exists)

		assertLayerTarMatches(ctx, t, iv.Store(), desc, "hello.txt", content)

		require.Len(t, iv.config.RootFS.DiffIDs, 1)
		results[compression] = result{desc: desc, diffID: iv.config.RootFS.DiffIDs[0]}
	}

	require.Equal(t, ocispec.MediaTypeImageLayer, results[VolumeCompressionUncompressed].desc.MediaType)
	require.Equal(t, ocispec.MediaTypeImageLayerGzip, results[VolumeCompressionGzip].desc.MediaType)
	require.Equal(t, ocispec.MediaTypeImageLayerZstd, results[VolumeCompressionZstd].desc.MediaType)

	// The diff ID always identifies the uncompressed tar content, regardless
	// of which compression produced the pushed blob.
	require.Equal(t, results[VolumeCompressionUncompressed].diffID, results[VolumeCompressionGzip].diffID)
	require.Equal(t, results[VolumeCompressionUncompressed].diffID, results[VolumeCompressionZstd].diffID)

	// An uncompressed blob's digest equals its diff ID; compressed blob
	// digests differ from the diff ID (and from each other).
	require.Equal(t, results[VolumeCompressionUncompressed].diffID, results[VolumeCompressionUncompressed].desc.Digest)
	require.NotEqual(t, results[VolumeCompressionGzip].diffID, results[VolumeCompressionGzip].desc.Digest)
	require.NotEqual(t, results[VolumeCompressionZstd].diffID, results[VolumeCompressionZstd].desc.Digest)
	require.NotEqual(t, results[VolumeCompressionGzip].desc.Digest, results[VolumeCompressionZstd].desc.Digest)
}

func TestVolumeWriteTar(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	iv := newTestVolume(t)
	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("hello world"), 0o644))

	const ref = "test:latest"
	require.NoError(t, iv.AddDirectory(ctx, srcDir, ref))

	var buf bytes.Buffer
	require.NoError(t, iv.WriteTar(ctx, ref, &buf))

	names := map[string]bool{}
	tr := tar.NewReader(&buf)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		names[hdr.Name] = true
	}

	require.True(t, names["oci-layout"], "expected oci-layout entry")
	require.True(t, names["index.json"], "expected index.json entry")
	require.True(t, names["manifest.json"], "expected manifest.json entry")
}

// TestVolumeWriteTarWithoutManifest documents that calling WriteTar before
// AddDirectory does not error: the zero-value manifest descriptor has no
// digest, so the containerd exporter silently drops it (after logging a
// warning) and produces a valid but empty archive.
func TestVolumeWriteTarWithoutManifest(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	iv := newTestVolume(t)

	var buf bytes.Buffer
	require.NoError(t, iv.WriteTar(ctx, "test:latest", &buf))

	names := map[string]bool{}
	tr := tar.NewReader(&buf)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		names[hdr.Name] = true
	}

	require.True(t, names["oci-layout"])
	require.True(t, names["index.json"])
	require.False(t, names["manifest.json"], "no image was recorded, so no docker manifest.json should be written")
}

func TestVolumeAddFileUnsupportedCompression(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	iv := newTestVolume(t)
	iv.Compression = VolumeCompression("bogus")

	srcDir := t.TempDir()
	p := filepath.Join(srcDir, "a.txt")
	require.NoError(t, os.WriteFile(p, []byte("a"), 0o644))

	_, err := iv.AddFile(ctx, srcDir, p)
	require.ErrorContains(t, err, "bogus")
}

func TestVolumeCompressionZeroValueIsUncompressed(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	ociDir := t.TempDir()
	store, err := oci.New(ociDir)
	require.NoError(t, err)

	iv := &Volume{store: store, tmp: t.TempDir(), root: ociDir}
	t.Cleanup(func() { require.NoError(t, iv.Clean()) })

	srcDir := t.TempDir()
	p := filepath.Join(srcDir, "a.txt")
	require.NoError(t, os.WriteFile(p, []byte("a"), 0o644))

	desc, err := iv.AddFile(ctx, srcDir, p)
	require.NoError(t, err)
	require.Equal(t, ocispec.MediaTypeImageLayer, desc.MediaType)
}

// assertLayerTarMatches fetches the pushed layer, decompresses it according
// to its media type, and verifies it is a single-entry tar archive
// containing name with the given content.
func assertLayerTarMatches(ctx context.Context, t *testing.T, store *oci.Store, desc ocispec.Descriptor, name string, content []byte) {
	t.Helper()

	rc, err := store.Fetch(ctx, desc)
	require.NoError(t, err)
	defer func() { require.NoError(t, rc.Close()) }()

	var r io.Reader
	switch desc.MediaType {
	case ocispec.MediaTypeImageLayer:
		r = rc
	case ocispec.MediaTypeImageLayerGzip:
		gr, err := gzip.NewReader(rc)
		require.NoError(t, err)
		defer func() { require.NoError(t, gr.Close()) }()
		r = gr
	case ocispec.MediaTypeImageLayerZstd:
		zr, err := zstd.NewReader(rc)
		require.NoError(t, err)
		defer zr.Close()
		r = zr
	default:
		t.Fatalf("unexpected layer media type %q", desc.MediaType)
	}

	tr := tar.NewReader(r)
	hdr, err := tr.Next()
	require.NoError(t, err)
	require.Equal(t, name, hdr.Name)

	got, err := io.ReadAll(tr)
	require.NoError(t, err)
	require.Equal(t, content, got)

	_, err = tr.Next()
	require.ErrorIs(t, err, io.EOF)
}
