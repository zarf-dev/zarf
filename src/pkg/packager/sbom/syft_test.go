// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package sbom

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/require"
)

func TestNewSyftGenerator(t *testing.T) {
	t.Parallel()

	gen := NewSyftGenerator()
	require.NotNil(t, gen)
}

func TestFilesSBOM(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	gen := NewSyftGenerator()

	t.Run("successfully generates SBOM for directory files", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		sampleFile := filepath.Join(tempDir, "test.txt")
		err := os.WriteFile(sampleFile, []byte("hello zarf sbom"), 0600)
		require.NoError(t, err)

		rawSBOM, err := gen.FilesSBOM(ctx, tempDir, []string{sampleFile})
		require.NoError(t, err)
		require.NotEmpty(t, rawSBOM)

		var parsed map[string]any
		err = json.Unmarshal(rawSBOM, &parsed)
		require.NoError(t, err)
		require.Contains(t, parsed, "artifacts")
		require.Contains(t, parsed, "descriptor")
	})

	t.Run("returns error when root dir does not exist", func(t *testing.T) {
		t.Parallel()

		nonExistentDir := filepath.Join(t.TempDir(), "missing-dir")
		_, err := gen.FilesSBOM(ctx, nonExistentDir, []string{"foo.txt"})
		require.Error(t, err)
	})

	t.Run("returns error when target file does not exist", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		missingFile := filepath.Join(tempDir, "non-existent.txt")
		_, err := gen.FilesSBOM(ctx, tempDir, []string{missingFile})
		require.Error(t, err)
	})
}
