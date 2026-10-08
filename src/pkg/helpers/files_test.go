// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helpers

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadFileByChunksRejectsInvalidSize(t *testing.T) {
	_, _, err := ReadFileByChunks("unused", 0)
	require.ErrorContains(t, err, "chunk size")
}

func TestCreatePathAndCopyPreservesSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlink creation requires elevated privileges")
	}

	target := filepath.Join(t.TempDir(), "target.txt")
	require.NoError(t, os.WriteFile(target, []byte("target"), ReadAllWriteUser))
	source := filepath.Join(t.TempDir(), "source.txt")
	require.NoError(t, os.Symlink(target, source))

	destination := filepath.Join(t.TempDir(), "destination")
	require.NoError(t, CreatePathAndCopy(source, destination))

	info, err := os.Lstat(destination)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeSymlink)
	linkTarget, err := os.Readlink(destination)
	require.NoError(t, err)
	require.Equal(t, target, linkTarget)
}

func TestMaterializePathAndCopyContainedSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlink creation requires elevated privileges")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source")
	targetDir := filepath.Join(root, "targets")
	require.NoError(t, os.MkdirAll(source, ReadWriteExecuteUser))
	require.NoError(t, os.MkdirAll(filepath.Join(targetDir, "directory"), ReadWriteExecuteUser))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, "file.txt"), []byte("file target"), ReadAllWriteUser))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, "directory", "nested.txt"), []byte("directory target"), ReadAllWriteUser))
	require.NoError(t, os.Symlink(filepath.Join("..", "targets", "file.txt"), filepath.Join(source, "file-link")))
	require.NoError(t, os.Symlink(filepath.Join("..", "targets", "directory"), filepath.Join(source, "directory-link")))

	var materialized [][2]string
	destination := filepath.Join(t.TempDir(), "destination")
	err := MaterializePathAndCopy(source, destination, MaterializeOptions{
		SourceRoot: root,
		OnMaterializedSymlink: func(link, target string) {
			materialized = append(materialized, [2]string{link, target})
		},
	})
	require.NoError(t, err)
	require.Equal(t, [][2]string{
		{filepath.Join(source, "directory-link"), filepath.Join(targetDir, "directory")},
		{filepath.Join(source, "file-link"), filepath.Join(targetDir, "file.txt")},
	}, materialized)

	fileInfo, err := os.Lstat(filepath.Join(destination, "file-link"))
	require.NoError(t, err)
	require.Zero(t, fileInfo.Mode()&os.ModeSymlink)
	require.FileExists(t, filepath.Join(destination, "directory-link", "nested.txt"))
	require.Equal(t, "file target", string(mustReadFile(t, filepath.Join(destination, "file-link"))))
	require.Equal(t, "directory target", string(mustReadFile(t, filepath.Join(destination, "directory-link", "nested.txt"))))
}

func TestMaterializePathAndCopyRejectsUnsafeSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlink creation requires elevated privileges")
	}

	t.Run("outside source root", func(t *testing.T) {
		root := t.TempDir()
		outside := filepath.Join(t.TempDir(), "outside.txt")
		require.NoError(t, os.WriteFile(outside, []byte("outside"), ReadAllWriteUser))
		source := filepath.Join(root, "source")
		require.NoError(t, os.Mkdir(source, ReadWriteExecuteUser))
		require.NoError(t, os.Symlink(outside, filepath.Join(source, "escape")))

		err := MaterializePathAndCopy(source, filepath.Join(t.TempDir(), "destination"), MaterializeOptions{
			SourceRoot: root,
		})
		require.ErrorContains(t, err, "resolves outside source root")
	})

	t.Run("cycle", func(t *testing.T) {
		root := t.TempDir()
		source := filepath.Join(root, "source")
		require.NoError(t, os.Mkdir(source, ReadWriteExecuteUser))
		require.NoError(t, os.Symlink(".", filepath.Join(source, "loop")))

		err := MaterializePathAndCopy(source, filepath.Join(t.TempDir(), "destination"), MaterializeOptions{
			SourceRoot: root,
		})
		require.ErrorContains(t, err, "symlink cycle")
	})

	err := MaterializePathAndCopy(t.TempDir(), filepath.Join(t.TempDir(), "destination"), MaterializeOptions{})
	require.ErrorContains(t, err, "source root is required")
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}
