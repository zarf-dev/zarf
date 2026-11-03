// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package assemble

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestPackageFile(t *testing.T) {
	const content = "package file content\n"
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "source.txt"), []byte(content), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(root, "directory"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "directory", "source.txt"), []byte(content), 0o600))

	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "nested/source.txt", Mode: 0o600, Size: int64(len(content))}))
	_, err := tw.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, os.WriteFile(filepath.Join(root, "source.tar"), archive.Bytes(), 0o600))
	server := httptest.NewServer(http.FileServer(http.Dir(root)))
	defer server.Close()

	validChecksum := api.FileChecksum{Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(content)))}
	invalidChecksum := api.FileChecksum{Digest: fmt.Sprintf("%x", sha256.Sum256(nil))}
	tests := []struct {
		name      string
		file      api.File
		directory bool
		wantErr   string
	}{
		{name: "relative source", file: api.File{Source: "source.txt", Checksum: validChecksum}},
		{name: "absolute source", file: api.File{Source: filepath.Join(root, "source.txt")}},
		{name: "executable", file: api.File{Source: "source.txt", Executable: true}},
		{name: "directory", file: api.File{Source: "directory"}, directory: true},
		{name: "download", file: api.File{Source: server.URL + "/source.txt", Checksum: validChecksum}},
		{name: "local archive", file: api.File{Source: "source.tar", ExtractPath: "nested/source.txt", Checksum: validChecksum}},
		{name: "downloaded archive", file: api.File{Source: server.URL + "/source.tar", ExtractPath: "nested/source.txt", Checksum: validChecksum}},
		{name: "local checksum mismatch", file: api.File{Source: "source.txt", Checksum: invalidChecksum}, wantErr: "sha mismatch for"},
		{name: "downloaded archive checksum mismatch", file: api.File{Source: server.URL + "/source.tar", ExtractPath: "nested/source.txt", Checksum: invalidChecksum}, wantErr: "sha mismatch for"},
		{name: "missing source", file: api.File{Source: "missing.txt"}, wantErr: "unable to access local resource"},
		{name: "missing download", file: api.File{Source: server.URL + "/missing.txt"}, wantErr: "missing.txt"},
		{name: "invalid local archive", file: api.File{Source: "source.txt", ExtractPath: "nested/source.txt"}, wantErr: "nested/source.txt"},
		{name: "invalid downloaded archive", file: api.File{Source: server.URL + "/source.txt", ExtractPath: "nested/source.txt"}, wantErr: "nested/source.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := filepath.Join(t.TempDir(), "staging", "renamed.txt")
			err := PackageFile(testutil.TestContext(t), tt.file, dst, load.NewResourceSet(root))
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			info, err := os.Stat(dst)
			require.NoError(t, err)
			if runtime.GOOS != "windows" {
				mode := os.FileMode(0o600)
				if tt.directory || tt.file.Executable {
					mode = 0o700
				}
				require.Equal(t, mode, info.Mode().Perm())
			}
			if tt.directory {
				require.True(t, info.IsDir())
				dst = filepath.Join(dst, "source.txt")
			}
			data, err := os.ReadFile(dst)
			require.NoError(t, err)
			require.Equal(t, content, string(data))
		})
	}
}

func TestPackageFileMaterializesSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlink creation requires elevated privileges")
	}
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "source.txt")
	require.NoError(t, os.WriteFile(source, []byte("materialized"), 0o600))
	require.NoError(t, os.Symlink(source, filepath.Join(root, "link.txt")))
	dst := filepath.Join(t.TempDir(), "destination.txt")
	require.NoError(t, PackageFile(testutil.TestContext(t), api.File{Source: "link.txt"}, dst, load.NewResourceSet(root)))
	info, err := os.Lstat(dst)
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular())
	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	require.Equal(t, "materialized", string(data))
}
