// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package signing

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/stretchr/testify/require"
)

func TestWriteEmbeddedTrustedRoot(t *testing.T) {
	t.Parallel()

	t.Run("writes valid JSON to a tempfile", func(t *testing.T) {
		t.Parallel()
		path, cleanup, err := writeEmbeddedTrustedRoot("")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, cleanup()) })
		require.NotEmpty(t, path)

		contents, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NotEmpty(t, contents)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal(contents, &parsed))
		require.Equal(t, "application/vnd.dev.sigstore.trustedroot+json;version=0.1", parsed["mediaType"],
			"embedded trusted root must be a Sigstore TrustedRoot v0.1 document")
	})

	t.Run("cleanup removes the tempfile", func(t *testing.T) {
		t.Parallel()
		path, cleanup, err := writeEmbeddedTrustedRoot("")
		require.NoError(t, err)
		require.FileExists(t, path)

		require.NoError(t, cleanup())
		_, statErr := os.Stat(path)
		require.ErrorIs(t, statErr, os.ErrNotExist)
	})

	t.Run("each call produces a distinct tempfile", func(t *testing.T) {
		t.Parallel()
		p1, c1, err := writeEmbeddedTrustedRoot("")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, c1()) })
		p2, c2, err := writeEmbeddedTrustedRoot("")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, c2()) })
		require.NotEqual(t, p1, p2)
	})
}

func TestConfiguredTUFOptions(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T) (string, func(*tuf.Options))
	}{
		{
			name: "uses explicit cache mirror and root",
			setup: func(t *testing.T) (string, func(*tuf.Options)) {
				cachePath := t.TempDir()
				rootPath := filepath.Join(t.TempDir(), "root.json")
				rootJSON := `{"root":"explicit"}`
				require.NoError(t, os.WriteFile(rootPath, []byte(rootJSON), 0o600))
				t.Setenv("TUF_ROOT", cachePath)
				t.Setenv("TUF_MIRROR", "https://mirror.example.test")
				t.Setenv("TUF_ROOT_JSON", rootPath)
				return "", func(opts *tuf.Options) {
					require.Equal(t, cachePath, opts.CachePath)
					require.Equal(t, "https://mirror.example.test", opts.RepositoryBaseURL)
					require.True(t, bytes.Equal([]byte(rootJSON), opts.Root))
				}
			},
		},
		{
			name: "uses cached mirror and root",
			setup: func(t *testing.T) (string, func(*tuf.Options)) {
				cachePath := t.TempDir()
				mirror := "https://mirror.example.test"
				rootJSON := `{"root":"cached"}`
				cachedRootPath := filepath.Join(cachePath, tuf.URLToPath(mirror), "root.json")
				require.NoError(t, os.MkdirAll(filepath.Dir(cachedRootPath), 0o700))
				require.NoError(t, os.WriteFile(cachedRootPath, []byte(rootJSON), 0o600))
				remoteJSON, err := json.Marshal(map[string]string{"mirror": mirror})
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(filepath.Join(cachePath, "remote.json"), remoteJSON, 0o600))
				t.Setenv("TUF_ROOT", cachePath)
				return "", func(opts *tuf.Options) {
					require.Equal(t, cachePath, opts.CachePath)
					require.Equal(t, mirror, opts.RepositoryBaseURL)
					require.True(t, bytes.Equal([]byte(rootJSON), opts.Root))
				}
			},
		},
		{
			name: "rejects custom mirror without root",
			setup: func(t *testing.T) (string, func(*tuf.Options)) {
				cachePath := t.TempDir()
				mirror := "https://mirror.example.test"
				t.Setenv("TUF_ROOT", cachePath)
				t.Setenv("TUF_MIRROR", mirror)
				return filepath.Join(cachePath, tuf.URLToPath(mirror), "root.json"), nil
			},
		},
		{
			name: "uses embedded root for default mirror",
			setup: func(t *testing.T) (string, func(*tuf.Options)) {
				cachePath := t.TempDir()
				rootPath := filepath.Join(t.TempDir(), "root.json")
				require.NoError(t, os.WriteFile(rootPath, []byte(`{"root":"ignored"}`), 0o600))
				t.Setenv("TUF_ROOT", cachePath)
				t.Setenv("TUF_MIRROR", "")
				t.Setenv("TUF_ROOT_JSON", rootPath)
				return "", func(opts *tuf.Options) {
					require.Equal(t, tuf.DefaultMirror, opts.RepositoryBaseURL)
					require.Equal(t, tuf.DefaultRoot(), opts.Root)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TUF_ROOT", "")
			t.Setenv("TUF_MIRROR", "")
			t.Setenv("TUF_ROOT_JSON", "")
			expectedErrorPath, check := tc.setup(t)
			opts, err := configuredTUFOptions()
			if expectedErrorPath != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, expectedErrorPath)
				return
			}
			require.NoError(t, err)
			check(opts)
		})
	}
}
