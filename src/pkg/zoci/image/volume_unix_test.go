// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

//go:build unix

// Named pipes only exist on disk on Unix; on Windows they live in the
// \\.\pipe\ namespace, so a tree walk never encounters one and syscall.Mkfifo
// is not defined there.

package image

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/test/testutil"
)

// TestWriteTarFileSkipsIrregularFiles checks that a file type an image volume
// cannot hold is skipped rather than opened - opening a FIFO blocks until a
// writer appears.
func TestWriteTarFileSkipsIrregularFiles(t *testing.T) {
	t.Parallel()

	fifo := filepath.Join(t.TempDir(), "pipe")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("cannot create a FIFO here: %v", err)
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	written, err := writeTarFile(tw, "pipe", fifo)
	require.NoError(t, err)
	require.False(t, written, "a FIFO has no representation in an image volume")
	require.NoError(t, tw.Close())

	_, err = tar.NewReader(&buf).Next()
	require.ErrorIs(t, err, io.EOF, "nothing should have been written")
}

// TestVolumeAddDirectorySkipsIrregularFiles checks that a FIFO in the tree is
// left out instead of stalling or failing the whole build.
func TestVolumeAddDirectorySkipsIrregularFiles(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("a"), 0o644))
	if err := syscall.Mkfifo(filepath.Join(srcDir, "pipe"), 0o644); err != nil {
		t.Skipf("cannot create a FIFO here: %v", err)
	}

	iv := newTestVolume(t)
	require.NoError(t, iv.AddDirectory(ctx, srcDir, "test:latest"))

	require.Len(t, iv.layers, 1)
	require.Equal(t, "a.txt", iv.layers[0].Annotations[ocispec.AnnotationTitle])
}
