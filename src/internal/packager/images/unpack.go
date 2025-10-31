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
	tmpdir, err := utils.MakeTempDir("")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpdir))
	}()

	if err := archive.Decompress(ctx, tarPath, tmpdir, archive.DecompressOpts{}); err != nil {
		return nil, fmt.Errorf("failed to extract tar: %w", err)
	}

	// Determine the image directory:
	// - If there's a single directory entry, the tar had a wrapping directory (e.g., "my-image/")
	// - If there are multiple entries, the tar contents are at the top level
	entries, err := os.ReadDir(tmpdir)
	if err != nil {
		return nil, fmt.Errorf("failed to read extracted directory: %w", err)
	}

	var imageDir string
	if len(entries) == 1 && entries[0].IsDir() {
		// Single directory entry - navigate into it
		imageDir = filepath.Join(tmpdir, entries[0].Name())
	} else {
		// Multiple entries or single file - use tmpdir directly
		imageDir = tmpdir
	}

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
		if manifestDesc.Annotations == nil {
			return nil, fmt.Errorf("manifest %s has empty annotations, couldn't find image name", manifestDesc.Digest)
		}
		imageName := getRefFromAnnotations(manifestDesc.Annotations)
		if imageName == "" {
			return nil, fmt.Errorf("no valid reference annotation found for manifest %s", manifestDesc.Digest)
		}

		img, err := transform.ParseImageRef(imageName)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image reference %s: %w", imageName, err)
		}

		copyOpts := oras.DefaultCopyOptions
		desc, err := oras.Copy(ctx, srcStore, manifestDesc.Digest.String(), dstStore, img.Reference, copyOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to copy image %s: %w", img.Reference, err)
		}

		// Tag the image with annotations so that Syft and ORAS can see them
		if desc.Annotations == nil {
			desc.Annotations = make(map[string]string)
		}
		fmt.Println("tagging image with", img.Reference)
		desc.Annotations[ocispec.AnnotationRefName] = img.Reference
		desc.Annotations[ocispec.AnnotationBaseImageName] = img.Reference
		err = dstStore.Tag(ctx, desc, img.Reference)
		if err != nil {
			return nil, fmt.Errorf("failed to tag image: %w", err)
		}

		// Read the manifest from the destination store
		manifestBlobPath := filepath.Join(destDir, "blobs", "sha256", desc.Digest.Encoded())
		manifestData, err := os.ReadFile(manifestBlobPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read manifest blob for %s: %w", imageName, err)
		}

		var ociManifest ocispec.Manifest
		if err := json.Unmarshal(manifestData, &ociManifest); err != nil {
			return nil, fmt.Errorf("failed to parse OCI manifest for %s: %w", imageName, err)
		}

		imagesWithManifests = append(imagesWithManifests, ImageWithManifest{
			Image:    img,
			Manifest: ociManifest,
		})
	}

	return imagesWithManifests, nil
}

// getRefFromAnnotations extracts the image reference from annotations.
// It checks in order: org.opencontainers.image.ref.name, org.opencontainers.image.base.name, io.containerd.image.name
func getRefFromAnnotations(annotations map[string]string) string {
	// This is the location with an OCI-layout that these respective tools expect the image name to be
	orasRefAnnotation := ocispec.AnnotationRefName
	dockerRefAnnotation := "io.containerd.image.name"
	craneRefAnnotation := "org.opencontainers.image.base.name"
	if ref, ok := annotations[orasRefAnnotation]; ok && ref != "" {
		return ref
	}
	if ref, ok := annotations[dockerRefAnnotation]; ok && ref != "" {
		return ref
	}
	if ref, ok := annotations[craneRefAnnotation]; ok && ref != "" {
		return ref
	}
	return ""
}
