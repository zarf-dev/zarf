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

// imageManifest represents the structure of the manifest.json file in an image tar.
type imageManifest struct {
	Config       string                      `json:"Config"`
	RepoTags     []string                    `json:"RepoTags"`
	Layers       []string                    `json:"Layers"`
	LayerSources map[string]imageLayerSource `json:"LayerSources"`
}

// imageLayerSource represents metadata about a layer in the manifest.
type imageLayerSource struct {
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Digest    string `json:"digest"`
}

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

	// Read the manifest.json file
	manifestPath := filepath.Join(imageDir, "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return ocispec.Manifest{}, fmt.Errorf("failed to read manifest.json: %w", err)
	}

	// Parse the manifest
	var manifests []imageManifest
	if err := json.Unmarshal(manifestBytes, &manifests); err != nil {
		return ocispec.Manifest{}, fmt.Errorf("failed to parse manifest.json: %w", err)
	}

	if len(manifests) == 0 {
		return ocispec.Manifest{}, errors.New("no manifests found in manifest.json")
	}

	// For now, we only handle single image tars
	imgManifest := manifests[0]

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

	// Get the reference from RepoTags
	if len(imgManifest.RepoTags) == 0 {
		return ocispec.Manifest{}, errors.New("no RepoTags found in manifest")
	}
	ref := imgManifest.RepoTags[0]

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
