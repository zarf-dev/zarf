// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package signing

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteEmbeddedTrustedRoot(t *testing.T) {
	t.Parallel()

	t.Run("writes valid JSON to a tempfile", func(t *testing.T) {
		t.Parallel()
		path, cleanup, err := writeEmbeddedTrustedRoot()
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
		path, cleanup, err := writeEmbeddedTrustedRoot()
		require.NoError(t, err)
		require.FileExists(t, path)

		require.NoError(t, cleanup())
		_, statErr := os.Stat(path)
		require.ErrorIs(t, statErr, os.ErrNotExist)
	})

	t.Run("each call produces a distinct tempfile", func(t *testing.T) {
		t.Parallel()
		p1, c1, err := writeEmbeddedTrustedRoot()
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, c1()) })
		p2, c2, err := writeEmbeddedTrustedRoot()
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, c2()) })
		require.NotEqual(t, p1, p2)
	})
}
