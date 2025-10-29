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
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
)

// ImageWithManifest represents an image reference and its associated OCI manifest.
type ImageWithManifest struct {
	Image    transform.Image
	Manifest ocispec.Manifest
}

// Unpack extracts an image tar and loads it into an OCI layout directory.
// It returns a list of ImageWithManifest for all images in the tar.
func Unpack(ctx context.Context, tarPath string, destDir string) (_ []ImageWithManifest, err error) {
	// Create a temporary directory for extraction
	tmpDir, err := utils.MakeTempDir("")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpDir))
	}()

	if err := archive.Decompress(ctx, tarPath, tmpDir, archive.DecompressOpts{}); err != nil {
		return nil, fmt.Errorf("failed to extract tar: %w", err)
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read extracted directory: %w", err)
	}

	if len(entries) != 1 {
		return nil, fmt.Errorf("failed to properly extract directory")
	}
	imageDir := filepath.Join(tmpDir, entries[0].Name())

	// Create the OCI layout store at the destination
	if err := helpers.CreateDirectory(destDir, helpers.ReadExecuteAllWriteUser); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	dstStore, err := oci.NewWithContext(ctx, destDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create OCI store: %w", err)
	}

	// Create a source OCI store from the extracted image directory
	srcStore, err := oci.NewWithContext(ctx, imageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create source OCI store: %w", err)
	}

	// Read the index.json from the source to get the manifest descriptors
	srcIdx, err := getIndexFromOCILayout(imageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read source index.json: %w", err)
	}

	if len(srcIdx.Manifests) == 0 {
		return nil, errors.New("no manifests found in index.json")
	}

	// Process all manifests in the index
	var imagesWithManifests []ImageWithManifest

	for _, manifestDesc := range srcIdx.Manifests {
		// Try to get the reference from annotations in order of preference
		ref := getRefFromAnnotations(manifestDesc.Annotations)
		if ref == "" {
			return nil, fmt.Errorf("no valid reference annotation found for manifest %s", manifestDesc.Digest)
		}

		// Copy the image from source to destination using the digest
		copyOpts := oras.DefaultCopyOptions
		desc, err := oras.Copy(ctx, srcStore, manifestDesc.Digest.String(), dstStore, ref, copyOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to copy image %s: %w", ref, err)
		}

		// Read the manifest from the destination store
		manifestBlobPath := filepath.Join(destDir, "blobs", "sha256", desc.Digest.Hex())
		manifestData, err := os.ReadFile(manifestBlobPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read manifest blob for %s: %w", ref, err)
		}

		var ociManifest ocispec.Manifest
		if err := json.Unmarshal(manifestData, &ociManifest); err != nil {
			return nil, fmt.Errorf("failed to parse OCI manifest for %s: %w", ref, err)
		}

		// Parse the reference into a transform.Image
		imgRef, err := transform.ParseImageRef(ref)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image reference %s: %w", ref, err)
		}

		imagesWithManifests = append(imagesWithManifests, ImageWithManifest{
			Image:    imgRef,
			Manifest: ociManifest,
		})
	}

	return imagesWithManifests, nil
}

// getRefFromAnnotations extracts the image reference from annotations.
// It checks in order: org.opencontainers.image.ref.name, org.opencontainers.image.base.name, io.containerd.image.name
func getRefFromAnnotations(annotations map[string]string) string {
	if ref, ok := annotations[ocispec.AnnotationRefName]; ok && ref != "" {
		return ref
	}
	if ref, ok := annotations[ocispec.AnnotationBaseImageName]; ok && ref != "" {
		return ref
	}
	if ref, ok := annotations["io.containerd.image.name"]; ok && ref != "" {
		return ref
	}
	return ""
}
