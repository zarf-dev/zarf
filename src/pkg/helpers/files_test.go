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

func TestCreatePathAndCopyRejectsSymlinksByDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlink creation requires elevated privileges")
	}

	target := filepath.Join(t.TempDir(), "target.txt")
	require.NoError(t, os.WriteFile(target, []byte("target"), ReadAllWriteUser))

	tests := []struct {
		name   string
		source string
	}{
		{
			name: "source is a symlink",
			source: func() string {
				source := filepath.Join(t.TempDir(), "source.txt")
				require.NoError(t, os.Symlink(target, source))
				return source
			}(),
		},
		{
			name: "source tree contains a symlink",
			source: func() string {
				source := filepath.Join(t.TempDir(), "source")
				require.NoError(t, os.Mkdir(source, ReadWriteExecuteUser))
				require.NoError(t, os.Symlink(target, filepath.Join(source, "nested.txt")))
				return source
			}(),
		},
		{
			name: "source tree contains a cycle",
			source: func() string {
				source := filepath.Join(t.TempDir(), "source")
				require.NoError(t, os.Mkdir(source, ReadWriteExecuteUser))
				require.NoError(t, os.Symlink(".", filepath.Join(source, "loop")))
				return source
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "destination")
			err := CreatePathAndCopy(tt.source, destination)
			require.ErrorContains(t, err, "contains unsupported symlink")
			require.NoFileExists(t, destination)
		})
	}
}
