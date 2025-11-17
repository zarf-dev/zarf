// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetTrustedRootPath(t *testing.T) {
	t.Run("returns custom path when provided and exists", func(t *testing.T) {
		// Create a temporary custom trusted root file
		tmpDir := t.TempDir()
		customPath := filepath.Join(tmpDir, "custom_trusted_root.json")
		err := os.WriteFile(customPath, []byte(`{"test": "data"}`), 0o644)
		require.NoError(t, err)

		// Should use custom path
		path, cleanup, err := GetTrustedRootPath(customPath)
		defer cleanup()

		require.NoError(t, err)
		require.Equal(t, customPath, path)
	})

	t.Run("returns error when custom path provided but does not exist", func(t *testing.T) {
		customPath := "/nonexistent/path/trusted_root.json"

		_, _, err := GetTrustedRootPath(customPath)

		require.Error(t, err)
		require.Contains(t, err.Error(), "custom trusted root not found")
	})

	t.Run("uses embedded root when no custom path provided", func(t *testing.T) {
		// Should fall back to embedded root
		path, cleanup, err := GetTrustedRootPath("")
		defer cleanup()

		require.NoError(t, err)
		require.NotEmpty(t, path)

		// Verify the temp file exists and has content
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NotEmpty(t, content)
		require.Contains(t, string(content), "mediaType")
		require.Contains(t, string(content), "sigstore")
	})

	t.Run("cleanup function removes temp file", func(t *testing.T) {
		path, cleanup, err := GetTrustedRootPath("")
		require.NoError(t, err)
		require.NotEmpty(t, path)

		// Verify file exists
		_, err = os.Stat(path)
		require.NoError(t, err)

		// Call cleanup
		cleanup()

		// Verify file is removed
		_, err = os.Stat(path)
		require.True(t, os.IsNotExist(err))
	})
}

func TestGetTrustedRootMaterial(t *testing.T) {
	t.Run("loads custom trusted root from path", func(t *testing.T) {
		// Use the embedded root as our test custom root
		tmpDir := t.TempDir()
		customPath := filepath.Join(tmpDir, "custom_root.json")
		err := os.WriteFile(customPath, EmbeddedTrustedRoot, 0o644)
		require.NoError(t, err)

		material, err := GetTrustedRootMaterial(customPath)

		require.NoError(t, err)
		require.NotNil(t, material)
	})

	t.Run("uses embedded root when no custom path provided", func(t *testing.T) {
		material, err := GetTrustedRootMaterial("")

		require.NoError(t, err)
		require.NotNil(t, material)

		// Verify it has the expected methods
		cas := material.FulcioCertificateAuthorities()
		require.NotEmpty(t, cas, "should have certificate authorities")

		tlogs := material.RekorLogs()
		require.NotEmpty(t, tlogs, "should have transparency logs")
	})

	t.Run("returns error for invalid custom path", func(t *testing.T) {
		_, err := GetTrustedRootMaterial("/nonexistent/root.json")

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to load custom trusted root")
	})
}

func TestEmbeddedTrustedRoot(t *testing.T) {
	t.Run("embedded root is not empty", func(t *testing.T) {
		require.NotEmpty(t, EmbeddedTrustedRoot, "embedded trusted root should not be empty")
		require.Greater(t, len(EmbeddedTrustedRoot), 1000, "embedded root should be substantial")
	})

	t.Run("embedded root is valid JSON", func(t *testing.T) {
		require.Contains(t, string(EmbeddedTrustedRoot), "mediaType")
		require.Contains(t, string(EmbeddedTrustedRoot), "certificateAuthorities")
		require.Contains(t, string(EmbeddedTrustedRoot), "tlogs")
	})

	t.Run("embedded root can be parsed as TrustedMaterial", func(t *testing.T) {
		material, err := GetTrustedRootMaterial("")
		require.NoError(t, err)
		require.NotNil(t, material)
	})
}
