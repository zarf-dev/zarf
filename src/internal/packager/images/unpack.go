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
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
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
func Unpack(ctx context.Context, imageTar v1alpha1.ImageTar, destDir string) (_ []ImageWithManifest, err error) {
	// Create a temporary directory for extraction
	tmpdir, err := utils.MakeTempDir("")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpdir))
	}()

	if err := archive.Decompress(ctx, imageTar.Path, tmpdir, archive.DecompressOpts{}); err != nil {
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
		imageDir = filepath.Join(tmpdir, entries[0].Name())
	} else {
		imageDir = tmpdir
	}

	if err := helpers.CreateDirectory(destDir, helpers.ReadExecuteAllWriteUser); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	dstStore, err := oci.NewWithContext(ctx, destDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create OCI store: %w", err)
	}

	srcStore, err := oci.NewWithContext(ctx, imageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create source OCI store: %w", err)
	}

	// Read the index.json from the source to get the manifest descriptors of each image
	srcIdx, err := getIndexFromOCILayout(imageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read source index.json: %w", err)
	}

	if len(srcIdx.Manifests) == 0 {
		return nil, errors.New("no manifests found in index.json")
	}

	// Build a set of requested images for filtering
	requestedImages := make(map[string]bool)
	for _, img := range imageTar.Images {
		ref, err := transform.ParseImageRef(img)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image reference %s: %w", img, err)
		}
		requestedImages[ref.Reference] = false
	}

	// Process manifests in the index
	var imagesWithManifests []ImageWithManifest
	for _, manifestDesc := range srcIdx.Manifests {
		if manifestDesc.Annotations == nil {
			return nil, fmt.Errorf("manifest %s has empty annotations, couldn't find image name", manifestDesc.Digest)
		}

		imageName := getRefFromAnnotations(manifestDesc.Annotations)
		if imageName == "" {
			return nil, fmt.Errorf("no valid reference annotation found for manifest %s", manifestDesc.Digest)
		}
		manifestImg, err := transform.ParseImageRef(imageName)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image reference %s: %w", imageName, err)
		}

		// If specific images were requested, skip those not in the list
		if len(imageTar.Images) > 0 {
			if _, requested := requestedImages[manifestImg.Reference]; !requested {
				continue
			}
			requestedImages[manifestImg.Reference] = true
		}

		copyOpts := oras.DefaultCopyOptions
		desc, err := oras.Copy(ctx, srcStore, manifestDesc.Digest.String(), dstStore, manifestImg.Reference, copyOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to copy image %s: %w", manifestImg.Reference, err)
		}

		// Tag the image with annotations so that Syft and ORAS can see them
		desc = addNameAnnotationsToDesc(desc, manifestImg.Reference)
		err = dstStore.Tag(ctx, desc, manifestImg.Reference)
		if err != nil {
			return nil, fmt.Errorf("failed to tag image: %w", err)
		}

		_, manifestData, err := oras.FetchBytes(ctx, srcStore, manifestDesc.Digest.String(), oras.DefaultFetchBytesOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch manifest for %s: %w", imageName, err)
		}

		var ociManifest ocispec.Manifest
		if err := json.Unmarshal(manifestData, &ociManifest); err != nil {
			return nil, fmt.Errorf("failed to parse OCI manifest for %s: %w", imageName, err)
		}

		imagesWithManifests = append(imagesWithManifests, ImageWithManifest{
			Image:    manifestImg,
			Manifest: ociManifest,
		})
	}

	// Verify all requested images were found
	for img, found := range requestedImages {
		if !found {
			return nil, fmt.Errorf("could not find image %s", img)
		}
	}

	return imagesWithManifests, nil
}

// getRefFromAnnotations extracts the image reference from annotations.
func getRefFromAnnotations(annotations map[string]string) string {
	// This is the location with an OCI-layout that these respective tools expect the image name to be
	orasRefAnnotation := ocispec.AnnotationRefName
	dockerRefAnnotation := "io.containerd.image.name"
	if ref, ok := annotations[orasRefAnnotation]; ok && ref != "" {
		return ref
	}
	if ref, ok := annotations[dockerRefAnnotation]; ok && ref != "" {
		return ref
	}
	return ""
}
