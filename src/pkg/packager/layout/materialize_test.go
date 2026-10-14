// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/helpers"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestMaterializeSourceToLayoutSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlink creation requires elevated privileges")
	}

	t.Run("contained targets", func(t *testing.T) {
		root := t.TempDir()
		source := filepath.Join(root, "source")
		targetDir := filepath.Join(root, "targets")
		require.NoError(t, os.MkdirAll(source, helpers.ReadWriteExecuteUser))
		require.NoError(t, os.MkdirAll(filepath.Join(targetDir, "directory"), helpers.ReadWriteExecuteUser))
		require.NoError(t, os.WriteFile(filepath.Join(targetDir, "file.txt"), []byte("file target"), helpers.ReadAllWriteUser))
		require.NoError(t, os.WriteFile(filepath.Join(targetDir, "directory", "nested.txt"), []byte("directory target"), helpers.ReadAllWriteUser))
		require.NoError(t, os.Symlink(filepath.Join("..", "targets", "file.txt"), filepath.Join(source, "file-link")))
		require.NoError(t, os.Symlink(filepath.Join("..", "targets", "directory"), filepath.Join(source, "directory-link")))

		destination := filepath.Join(t.TempDir(), "destination")
		require.NoError(t, MaterializeSourceToLayout(testutil.TestContext(t), source, destination))

		fileInfo, err := os.Lstat(filepath.Join(destination, "file-link"))
		require.NoError(t, err)
		require.Zero(t, fileInfo.Mode()&os.ModeSymlink)
		require.FileExists(t, filepath.Join(destination, "directory-link", "nested.txt"))

		fileContent, err := os.ReadFile(filepath.Join(destination, "file-link"))
		require.NoError(t, err)
		require.Equal(t, "file target", string(fileContent))
		directoryContent, err := os.ReadFile(filepath.Join(destination, "directory-link", "nested.txt"))
		require.NoError(t, err)
		require.Equal(t, "directory target", string(directoryContent))
	})

	t.Run("external target", func(t *testing.T) {
		externalFile := filepath.Join(t.TempDir(), "external.txt")
		require.NoError(t, os.WriteFile(externalFile, []byte("external target"), helpers.ReadAllWriteUser))
		source := filepath.Join(t.TempDir(), "source-link")
		require.NoError(t, os.Symlink(externalFile, source))

		destination := filepath.Join(t.TempDir(), "destination")
		require.NoError(t, MaterializeSourceToLayout(testutil.TestContext(t), source, destination))

		info, err := os.Lstat(destination)
		require.NoError(t, err)
		require.Zero(t, info.Mode()&os.ModeSymlink)
		content, err := os.ReadFile(destination)
		require.NoError(t, err)
		require.Equal(t, "external target", string(content))
	})
}

func TestMaterializeSourceToLayoutRejectsCycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlink creation requires elevated privileges")
	}

	source := filepath.Join(t.TempDir(), "source")
	require.NoError(t, os.Mkdir(source, helpers.ReadWriteExecuteUser))
	require.NoError(t, os.Symlink(".", filepath.Join(source, "loop")))

	err := MaterializeSourceToLayout(testutil.TestContext(t), source, filepath.Join(t.TempDir(), "destination"))
	require.ErrorContains(t, err, "symlink cycle")
}
