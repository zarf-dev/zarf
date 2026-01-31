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

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/logger"
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

const (
	// This is the default docker annotation for the image name
	dockerRefAnnotation = "io.containerd.image.name"
	// When the Docker engine containerd image store is used only this annotation is used for sha referenced images
	dockerContainerdImageStoreAnnotation = "containerd.io/distribution.source.docker.io"
)

// GetManifestsFromArchive take an image archive and returns a list of image descriptiors
func GetManifestsFromArchive(ctx context.Context, imageArchive string) ([]ocispec.Descriptor, error) {
	// Create a temporary directory for extraction
	extractionDir, err := utils.MakeTempDir("")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(extractionDir))
	}()

	if err := archive.Decompress(ctx, imageArchive, extractionDir, archive.DecompressOpts{}); err != nil {
		return nil, fmt.Errorf("failed to extract tar: %w", err)
	}
	imageDir, err := determineImageDirectory(extractionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to determine image directory: %w", err)
	}

	manifests, err := getManifestsFromOCILayout(imageDir)

	return manifests, err
}

// FindImagesInManifests takes a list of OCI Descriptors and returns image References
func FindImagesInManifests(manifests []ocispec.Descriptor) ([]string, error) {
	var foundImages []string
	for _, manifestDesc := range manifests {
		imageName := getRefFromManifest(manifestDesc)
		if imageName == "" {
			continue
		}
		manifestImg, err := transform.ParseImageRef(imageName)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image reference %s: %w", imageName, err)
		}
		foundImages = append(foundImages, manifestImg.Reference)
	}

	return foundImages, nil
}

// Unpack extracts an image tar and loads it into an OCI layout directory.
// It returns a list of ImageWithManifest for all images in the tar.
func Unpack(ctx context.Context, imageArchive v1alpha1.ImageArchive, destDir string, arch string) (_ []ImageWithManifest, err error) {
	if len(imageArchive.Images) == 0 {
		return nil, fmt.Errorf("images must be defined")
	}

	extractionDir, err := utils.MakeTempDir("")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(extractionDir))
	}()

	if err := archive.Decompress(ctx, imageArchive.Path, extractionDir, archive.DecompressOpts{}); err != nil {
		return nil, fmt.Errorf("failed to extract tar: %w", err)
	}

	imageDir, err := determineImageDirectory(extractionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to determine image directory: %w", err)
	}

	manifests, err := getManifestsFromOCILayout(imageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifests from archive: %w", err)
	}

	dstStore, err := oci.NewWithContext(ctx, destDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create OCI store: %w", err)
	}

	// imageDir is the directory into which the archive was decompressed
	srcStore, err := oci.NewWithContext(ctx, imageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create source OCI store: %w", err)
	}

	// Build a set of requested images for filtering
	requestedImages := make(map[string]bool)
	for _, img := range imageArchive.Images {
		ref, err := transform.ParseImageRef(img)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image reference %s: %w", img, err)
		}
		requestedImages[ref.Reference] = false
	}

	var imagesWithManifest []ImageWithManifest
	var foundImages []string
	for _, manifestDesc := range manifests {
		imageName := getRefFromManifest(manifestDesc)
		if imageName == "" {
			continue
		}
		manifestImg, err := transform.ParseImageRef(imageName)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image reference %s: %w", imageName, err)
		}
		foundImages = append(foundImages, manifestImg.Reference)

		if _, requested := requestedImages[manifestImg.Reference]; !requested {
			continue
		}
		requestedImages[manifestImg.Reference] = true

		foundDesc, manifestData, err := oras.FetchBytes(ctx, srcStore, manifestDesc.Digest.String(), oras.DefaultFetchBytesOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch manifest for %s: %w", imageName, err)
		}
		// If an image index is returned, then grab the manifest at the specific platform, and set the platform for the later oras.Copy
		var platform *ocispec.Platform
		if foundDesc.MediaType == ocispec.MediaTypeImageIndex {
			platform = &ocispec.Platform{
				Architecture: arch,
				OS:           "linux",
			}
			fbOptions := oras.DefaultFetchBytesOptions
			fbOptions.TargetPlatform = platform
			foundDesc, manifestData, err = oras.FetchBytes(ctx, srcStore, foundDesc.Digest.String(), fbOptions)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch manifest for %s: %w", imageName, err)
			}
		}

		var ociManifest ocispec.Manifest
		if err := json.Unmarshal(manifestData, &ociManifest); err != nil {
			return nil, fmt.Errorf("failed to parse OCI manifest for %s: %w", imageName, err)
		}

		logger.From(ctx).Info("pulling image from archive", "image", manifestImg.Reference, "archive", imageArchive.Path)
		copyOpts := oras.DefaultCopyOptions
		copyOpts.WithTargetPlatform(platform)
		desc, err := oras.Copy(ctx, srcStore, manifestDesc.Digest.String(), dstStore, manifestImg.Reference, copyOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to copy image %s from archive %s: %w", manifestImg.Reference, imageArchive.Path, err)
		}
		// Tag the image with annotations so that Syft and ORAS can see them
		desc = addNameAnnotationsToDesc(desc, manifestImg.Reference)
		err = dstStore.Tag(ctx, desc, manifestImg.Reference)
		if err != nil {
			return nil, fmt.Errorf("failed to tag image: %w", err)
		}

		imagesWithManifest = append(imagesWithManifest, ImageWithManifest{
			Image:    manifestImg,
			Manifest: ociManifest,
		})
	}

	explainErr := fmt.Sprintf("image references are determined by the inclusion of one of the following "+
		"annotations in the index.json: %s, %s, %s", dockerRefAnnotation, dockerContainerdImageStoreAnnotation, ocispec.AnnotationRefName)
	for img, found := range requestedImages {
		if !found {
			return nil, fmt.Errorf("could not find image %s: found images %s: %s", img, foundImages, explainErr)
		}
	}

	return imagesWithManifest, nil
}

func determineImageDirectory(dir string) (string, error) {
	// Determine the image directory:
	// - If there's a single directory entry, the tar had a wrapping directory (e.g., "my-image/")
	// - If there are multiple entries, the tar contents are at the top level
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("failed to read extracted directory: %w", err)
	}
	var imageDir string
	if len(entries) == 1 && entries[0].IsDir() {
		imageDir = filepath.Join(dir, entries[0].Name())
	} else {
		imageDir = dir
	}
	return imageDir, nil
}

// getRefFromManifest extracts the image reference from a manifest descriptor.
func getRefFromManifest(manifestDesc ocispec.Descriptor) string {
	if manifestDesc.Annotations == nil {
		return ""
	}

	if ref, ok := manifestDesc.Annotations[dockerRefAnnotation]; ok && ref != "" {
		return ref
	}

	if repo, ok := manifestDesc.Annotations[dockerContainerdImageStoreAnnotation]; ok && repo != "" {
		return fmt.Sprintf("%s@%s", repo, manifestDesc.Digest.String())
	}

	// This is the annotation oras-go uses to check for the name during oras.copy
	// This may change for oras https://github.com/oras-project/oras/issues/1893
	// podman also uses this field
	if ref, ok := manifestDesc.Annotations[ocispec.AnnotationRefName]; ok && ref != "" {
		return ref
	}

	return ""
}
