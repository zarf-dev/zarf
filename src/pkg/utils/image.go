// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

// LoadOCIImage returns a v1.Image with the image ref specified from a location provided, or an error if the image cannot be found.
func LoadOCIImage(imgPath string, refInfo transform.Image) (v1.Image, error) {
	// Use the manifest within the index.json to load the specific image we want
	layoutPath := layout.Path(imgPath)
	imgIdx, err := layoutPath.ImageIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to get image index: %w", err)
	}
	idxManifest, err := imgIdx.IndexManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to get image manifest: %w", err)
	}

	// Search through all the manifests within this package until we find the annotation that matches our ref
	for _, manifest := range idxManifest.Manifests {
		if manifest.Annotations[ocispec.AnnotationBaseImageName] == refInfo.Reference ||
			// A backwards compatibility shim for older Zarf versions that would leave docker.io off of image annotations
			(manifest.Annotations[ocispec.AnnotationBaseImageName] == refInfo.Path+refInfo.TagOrDigest && refInfo.Host == "docker.io") ||
			manifest.Annotations[ocispec.AnnotationRefName] == refInfo.Reference {
			// This is the image we are looking for, load it and then return
			img, err := layoutPath.Image(manifest.Digest)
			if err != nil {
				return nil, fmt.Errorf("failed to lookup image %s: %w", refInfo.Reference, err)
			}
			return img, nil
		}
	}

	return nil, fmt.Errorf("unable to find image (%s) at the path (%s)", refInfo.Reference, imgPath)
}

// PlatformImage pairs a loaded image with the platform it targets.
// Platform is nil for images stored as a single-platform manifest.
type PlatformImage struct {
	Image    v1.Image
	Platform *v1.Platform
}

// LoadOCIImagePlatforms returns the v1.Images for refInfo. Single-platform images return one entry
// with a nil Platform; multi-arch indexes return one entry per platform manifest. Attestation or
// unknown-platform manifests inside an index are skipped so syft doesn't try to scan them. Nested
// indexes are traversed so platform manifests buried under attestation wrappers are still found.
func LoadOCIImagePlatforms(imgPath string, refInfo transform.Image) ([]PlatformImage, error) {
	layoutPath := layout.Path(imgPath)
	imgIdx, err := layoutPath.ImageIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to get image index: %w", err)
	}
	idxManifest, err := imgIdx.IndexManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to get image manifest: %w", err)
	}

	for _, manifest := range idxManifest.Manifests {
		if manifest.Annotations[ocispec.AnnotationBaseImageName] != refInfo.Reference &&
			(manifest.Annotations[ocispec.AnnotationBaseImageName] != refInfo.Path+refInfo.TagOrDigest || refInfo.Host != "docker.io") &&
			manifest.Annotations[ocispec.AnnotationRefName] != refInfo.Reference {
			continue
		}

		if manifest.MediaType == types.OCIImageIndex || manifest.MediaType == types.DockerManifestList {
			platformImages, err := collectPlatformImagesFromIndex(imgIdx, manifest.Digest, refInfo.Reference)
			if err != nil {
				return nil, err
			}
			if len(platformImages) == 0 {
				return nil, fmt.Errorf("image index for %s contained no scannable platform manifests", refInfo.Reference)
			}
			return platformImages, nil
		}

		img, err := layoutPath.Image(manifest.Digest)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup image %s: %w", refInfo.Reference, err)
		}
		return []PlatformImage{{Image: img}}, nil
	}

	return nil, fmt.Errorf("unable to find image (%s) at the path (%s)", refInfo.Reference, imgPath)
}

// collectPlatformImagesFromIndex walks an index by digest and returns one PlatformImage per
// platform leaf manifest. Nested indexes are descended into; entries with no/unknown platform
// (buildx attestations, etc.) are skipped.
func collectPlatformImagesFromIndex(parent v1.ImageIndex, indexDigest v1.Hash, ref string) ([]PlatformImage, error) {
	idx, err := parent.ImageIndex(indexDigest)
	if err != nil {
		return nil, fmt.Errorf("failed to load image index for %s: %w", ref, err)
	}
	manifest, err := idx.IndexManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to parse image index manifest for %s: %w", ref, err)
	}
	var platformImages []PlatformImage
	for _, child := range manifest.Manifests {
		if child.MediaType == types.OCIImageIndex || child.MediaType == types.DockerManifestList {
			nested, err := collectPlatformImagesFromIndex(idx, child.Digest, ref)
			if err != nil {
				return nil, err
			}
			platformImages = append(platformImages, nested...)
			continue
		}
		if child.Platform == nil || child.Platform.Architecture == "" || child.Platform.Architecture == "unknown" {
			continue
		}
		img, err := idx.Image(child.Digest)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup platform image for %s: %w", ref, err)
		}
		platformImages = append(platformImages, PlatformImage{
			Image:    img,
			Platform: child.Platform,
		})
	}
	return platformImages, nil
}

// AddImageNameAnnotation adds an annotation to the index.json file so that the deploying code can figure out what the image reference <-> digest shasum will be.
func AddImageNameAnnotation(ociPath string, referenceToDigest map[string]string) error {
	indexPath := filepath.Join(ociPath, "index.json")

	var index ocispec.Index
	b, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("unable to read the contents of the file (%s) so we can add an annotation: %w", indexPath, err)
	}
	if err = json.Unmarshal(b, &index); err != nil {
		return fmt.Errorf("unable to process the contents of the file (%s): %w", indexPath, err)
	}

	// Loop through the manifests and add the appropriate OCI Base Image Name Annotation
	for idx, manifest := range index.Manifests {
		if manifest.Annotations == nil {
			manifest.Annotations = make(map[string]string)
		}

		var baseImageName string

		for reference, digest := range referenceToDigest {
			if digest == manifest.Digest.String() {
				baseImageName = reference
			}
		}

		if baseImageName != "" {
			manifest.Annotations[ocispec.AnnotationBaseImageName] = baseImageName
			index.Manifests[idx] = manifest
			delete(referenceToDigest, baseImageName)
		}
	}

	// Write the file back to the package
	b, err = json.Marshal(index)
	if err != nil {
		return err
	}
	return os.WriteFile(indexPath, b, helpers.ReadWriteUser)
}

// SortImagesIndex sorts the index.json by digest.
func SortImagesIndex(ociPath string) error {
	indexPath := filepath.Join(ociPath, "index.json")
	b, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}
	var index ocispec.Index
	err = json.Unmarshal(b, &index)
	if err != nil {
		return err
	}
	slices.SortFunc(index.Manifests, func(a, b ocispec.Descriptor) int {
		return strings.Compare(string(a.Digest), string(b.Digest))
	})
	b, err = json.Marshal(index)
	if err != nil {
		return err
	}
	return os.WriteFile(indexPath, b, helpers.ReadWriteUser)
}
