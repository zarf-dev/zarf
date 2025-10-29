// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestUnpack(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	// First, create a tar from the testdata/my-image directory
	imageSrcDir := "testdata/my-image"
	tarFile := filepath.Join(t.TempDir(), "my-image.tar")

	// Create the tar from the source directory
	err := archive.Compress(ctx, []string{imageSrcDir}, tarFile, archive.CompressOpts{})
	require.NoError(t, err)

	// Create destination directory for the OCI layout
	dstDir := t.TempDir()

	// Call Unpack
	manifest, err := Unpack(ctx, tarFile, dstDir)
	require.NoError(t, err)

	// Verify the manifest is not empty
	require.NotEmpty(t, manifest.Config.Digest)
	require.NotEmpty(t, manifest.Layers)

	// Verify the OCI layout was created properly
	idx, err := getIndexFromOCILayout(dstDir)
	require.NoError(t, err)
	require.NotEmpty(t, idx.Manifests)

	// Verify that the manifest config blob exists
	configBlobPath := filepath.Join(dstDir, "blobs", "sha256", manifest.Config.Digest.Hex())
	require.FileExists(t, configBlobPath)

	// Verify all layer blobs exist
	for _, layer := range manifest.Layers {
		layerBlobPath := filepath.Join(dstDir, "blobs", "sha256", layer.Digest.Hex())
		require.FileExists(t, layerBlobPath)
	}
}

func TestUnpackInvalidTar(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	// Create a temporary file that is not a valid tar
	invalidTar := filepath.Join(t.TempDir(), "invalid.tar")
	err := os.WriteFile(invalidTar, []byte("not a tar file"), 0644)
	require.NoError(t, err)

	dstDir := t.TempDir()

	// Call Unpack with invalid tar
	_, err = Unpack(ctx, invalidTar, dstDir)
	require.Error(t, err)
}
