// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package image

import (
	"archive/tar"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content/file"

	"github.com/zarf-dev/zarf/src/test/testutil"
)

func newTestVolume(t *testing.T) *ImageVolume {
	t.Helper()
	iv, err := NewImageVolume(t.TempDir(), "linux", "amd64")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, iv.Clean()) })
	return iv
}

func TestNewImageVolume(t *testing.T) {
	t.Parallel()

	iv, err := NewImageVolume(t.TempDir(), "linux", "amd64")
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

func TestImageVolumeAddFile(t *testing.T) {
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
	require.Equal(t, "dev.zarf.image.volume.v0", iv.config.History[0].Comment)

	assertLayerTarMatches(ctx, t, iv.Store(), desc, "sub/hello.txt", content)
}

func TestImageVolumeAddFileAccumulatesLayers(t *testing.T) {
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

func TestImageVolumeAddDirectory(t *testing.T) {
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

	err := iv.AddDirectory(ctx, srcDir)
	require.NoError(t, err)

	require.Len(t, iv.layers, len(files))
	require.Len(t, iv.config.RootFS.DiffIDs, len(files))
	require.Len(t, iv.config.History, len(files))

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

func TestImageVolumeAddDirectoryEmpty(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	iv := newTestVolume(t)
	srcDir := t.TempDir()

	err := iv.AddDirectory(ctx, srcDir)
	require.NoError(t, err)
	require.Empty(t, iv.layers)
	require.Empty(t, iv.config.RootFS.DiffIDs)
}

func TestImageVolumeClean(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	iv, err := NewImageVolume(t.TempDir(), "linux", "amd64")
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

func TestImageVolumePushAfterCloseFails(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	iv := newTestVolume(t)
	require.NoError(t, iv.Store().Close())

	err := iv.Store().Push(ctx, ocispec.Descriptor{}, nil)
	require.ErrorIs(t, err, file.ErrStoreClosed)
}

// assertLayerTarMatches fetches the pushed layer and verifies it is a
// single-entry tar archive containing name with the given content.
func assertLayerTarMatches(ctx context.Context, t *testing.T, store *file.Store, desc ocispec.Descriptor, name string, content []byte) {
	t.Helper()

	rc, err := store.Fetch(ctx, desc)
	require.NoError(t, err)
	defer func() { require.NoError(t, rc.Close()) }()

	tr := tar.NewReader(rc)
	hdr, err := tr.Next()
	require.NoError(t, err)
	require.Equal(t, name, hdr.Name)

	got, err := io.ReadAll(tr)
	require.NoError(t, err)
	require.Equal(t, content, got)

	_, err = tr.Next()
	require.ErrorIs(t, err, io.EOF)
}
