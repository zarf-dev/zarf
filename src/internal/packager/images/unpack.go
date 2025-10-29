// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
)

// Unpack extracts an image tar and loads it into an OCI layout directory.
// It returns the OCI manifest for the image.
func Unpack(ctx context.Context, tarPath string, destDir string) (ocispec.Manifest, error) {
	// Create a temporary directory for extraction
	tmpDir, err := utils.MakeTempDir("")
	if err != nil {
		return ocispec.Manifest{}, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Extract the tar
	extractDir := filepath.Join(tmpDir, "extracted")
	if err := archive.Decompress(ctx, tarPath, extractDir, archive.DecompressOpts{}); err != nil {
		return ocispec.Manifest{}, fmt.Errorf("failed to extract tar: %w", err)
	}

	// Find the actual image directory (since we may have wrapped it)
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return ocispec.Manifest{}, fmt.Errorf("failed to read extracted directory: %w", err)
	}

	// If there's only one directory, assume it's the image directory
	var imageDir string
	if len(entries) == 1 && entries[0].IsDir() {
		imageDir = filepath.Join(extractDir, entries[0].Name())
	} else {
		imageDir = extractDir
	}

	// Create the OCI layout store at the destination
	if err := helpers.CreateDirectory(destDir, helpers.ReadExecuteAllWriteUser); err != nil {
		return ocispec.Manifest{}, fmt.Errorf("failed to create destination directory: %w", err)
	}

	dstStore, err := oci.NewWithContext(ctx, destDir)
	if err != nil {
		return ocispec.Manifest{}, fmt.Errorf("failed to create OCI store: %w", err)
	}

	// Create a source OCI store from the extracted image directory
	srcStore, err := oci.NewWithContext(ctx, imageDir)
	if err != nil {
		return ocispec.Manifest{}, fmt.Errorf("failed to create source OCI store: %w", err)
	}

	// Read the index.json from the source to get the manifest descriptor
	srcIdx, err := getIndexFromOCILayout(imageDir)
	if err != nil {
		return ocispec.Manifest{}, fmt.Errorf("failed to read source index.json: %w", err)
	}

	if len(srcIdx.Manifests) == 0 {
		return ocispec.Manifest{}, errors.New("no manifests found in index.json")
	}

	// Use the first manifest descriptor
	manifestDesc := srcIdx.Manifests[0]

	ref := manifestDesc.Annotations["io.containerd.image.name"]

	// Copy the image from source to destination using the digest
	copyOpts := oras.DefaultCopyOptions
	desc, err := oras.Copy(ctx, srcStore, manifestDesc.Digest.String(), dstStore, ref, copyOpts)
	if err != nil {
		return ocispec.Manifest{}, fmt.Errorf("failed to copy image: %w", err)
	}

	// Read the manifest from the destination store
	manifestBlobPath := filepath.Join(destDir, "blobs", "sha256", desc.Digest.Hex())
	manifestData, err := os.ReadFile(manifestBlobPath)
	if err != nil {
		return ocispec.Manifest{}, fmt.Errorf("failed to read manifest blob: %w", err)
	}

	var ociManifest ocispec.Manifest
	if err := json.Unmarshal(manifestData, &ociManifest); err != nil {
		return ocispec.Manifest{}, fmt.Errorf("failed to parse OCI manifest: %w", err)
	}

	return ociManifest, nil
}
