// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci

import (
	"os"
	"path/filepath"
	"testing"

	godigest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
)

func TestRestoreDuplicateLayers(t *testing.T) {
	t.Parallel()

	destination := t.TempDir()
	originalTitle := "documentation/secrets.md"
	duplicateTitle := "documentation/secrets-copy.md"
	contents := []byte("same documentation")
	originalPath := filepath.Join(destination, filepath.FromSlash(originalTitle))
	require.NoError(t, os.MkdirAll(filepath.Dir(originalPath), 0o700))
	require.NoError(t, os.WriteFile(originalPath, contents, 0o600))

	digest := godigest.FromBytes(contents)
	layers := []ocispec.Descriptor{
		{Digest: digest, Size: int64(len(contents)), Annotations: map[string]string{ocispec.AnnotationTitle: originalTitle}},
		{Digest: digest, Size: int64(len(contents)), Annotations: map[string]string{ocispec.AnnotationTitle: duplicateTitle}},
	}

	require.NoError(t, restoreDuplicateLayers(destination, layers))
	duplicateContents, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(duplicateTitle)))
	require.NoError(t, err)
	require.Equal(t, contents, duplicateContents)
}
