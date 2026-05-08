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
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/oci"
)

// PulledImage describes an image that landed in the destination OCI layout.
type PulledImage struct {
	Image transform.Image
}

const (
	// This is the default docker annotation for the image name
	dockerRefAnnotation = "io.containerd.image.name"
	// When the Docker engine containerd image store is used only this annotation is used for sha referenced images
	dockerContainerdImageStoreAnnotation = "containerd.io/distribution.source.docker.io"
)

// Unpack extracts an image tar and loads it into an OCI layout directory.
// It returns a list of PulledImage for all images in the tar.
func Unpack(ctx context.Context, imageArchive v1alpha1.ImageArchive, destDir string, arch string) (_ []PulledImage, err error) {
	if len(imageArchive.Images) == 0 {
		return nil, fmt.Errorf("images must be defined")
	}
	// Create a temporary directory for extraction
	tmpdir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpdir))
	}()

	if err := archive.Decompress(ctx, imageArchive.Path, tmpdir, archive.DecompressOpts{}); err != nil {
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
	for _, img := range imageArchive.Images {
		ref, err := transform.ParseImageRef(img)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image reference %s: %w", img, err)
		}
		requestedImages[ref.Reference] = false
	}

	var pulledImages []PulledImage
	var foundImages []string
	for _, manifestDesc := range srcIdx.Manifests {
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

		foundDesc, _, err := oras.FetchBytes(ctx, srcStore, manifestDesc.Digest.String(), oras.DefaultFetchBytesOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch manifest for %s: %w", imageName, err)
		}

		logger.From(ctx).Info("pulling image from archive", "image", manifestImg.Reference, "archive", imageArchive.Path)
		// Mirror images.Pull: an index-digest reference preserves the full index (all platforms),
		// while a tag or manifest-digest reference resolves to a single platform manifest. For
		// indexes that's a recursive walk so nested indexes work.
		copyDesc := manifestDesc
		isIndexSha := manifestImg.Digest != "" && IsIndex(foundDesc.MediaType)
		if IsIndex(foundDesc.MediaType) && !isIndexSha {
			target := &ocispec.Platform{Architecture: arch, OS: "linux"}
			copyDesc, err = resolvePlatformManifest(ctx, srcStore, manifestDesc, target)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve %s/%s manifest for %s: %w", target.OS, target.Architecture, manifestImg.Reference, err)
			}
		}
		desc, err := oras.Copy(ctx, srcStore, copyDesc.Digest.String(), dstStore, manifestImg.Reference, oras.DefaultCopyOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to copy image %s from archive %s: %w", manifestImg.Reference, imageArchive.Path, err)
		}
		// Tag the image with annotations so that Syft and ORAS can see them
		desc = addNameAnnotationsToDesc(desc, manifestImg.Reference)
		err = dstStore.Tag(ctx, desc, manifestImg.Reference)
		if err != nil {
			return nil, fmt.Errorf("failed to tag image: %w", err)
		}

		pulledImages = append(pulledImages, PulledImage{Image: manifestImg})
	}

	explainErr := fmt.Sprintf("image references are determined by the inclusion of one of the following "+
		"annotations in the index.json: %s, %s, %s", dockerRefAnnotation, dockerContainerdImageStoreAnnotation, ocispec.AnnotationRefName)
	for img, found := range requestedImages {
		if !found {
			return nil, fmt.Errorf("could not find image %s: found images %s: %s", img, foundImages, explainErr)
		}
	}

	return pulledImages, nil
}

// resolvePlatformManifest walks an index (recursing into nested indexes) and returns the first
// leaf manifest descriptor whose platform matches target. If root is already a manifest, it is
// returned unchanged.
func resolvePlatformManifest(ctx context.Context, src content.ReadOnlyStorage, root ocispec.Descriptor, target *ocispec.Platform) (ocispec.Descriptor, error) {
	if !IsIndex(root.MediaType) {
		return root, nil
	}
	body, err := content.FetchAll(ctx, src, root)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to fetch index %s: %w", root.Digest, err)
	}
	var idx ocispec.Index
	if err := json.Unmarshal(body, &idx); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to unmarshal index %s: %w", root.Digest, err)
	}
	for _, child := range idx.Manifests {
		if IsManifest(child.MediaType) && platformMatches(child.Platform, target) {
			return child, nil
		}
	}
	for _, child := range idx.Manifests {
		if !IsIndex(child.MediaType) {
			continue
		}
		if desc, err := resolvePlatformManifest(ctx, src, child, target); err == nil {
			return desc, nil
		}
	}
	return ocispec.Descriptor{}, fmt.Errorf("no manifest matched platform %s/%s in index %s", target.OS, target.Architecture, root.Digest)
}

func platformMatches(got, want *ocispec.Platform) bool {
	if got == nil || want == nil {
		return false
	}
	if want.Architecture != "" && got.Architecture != want.Architecture {
		return false
	}
	if want.OS != "" && got.OS != want.OS {
		return false
	}
	return true
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
