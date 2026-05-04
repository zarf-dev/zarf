// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"oras.land/oras-go/v2"
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
func Unpack(ctx context.Context, imageArchive v1alpha1.ImageArchive, destDir string, platforms []ocispec.Platform) (_ []PulledImage, err error) {
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

		foundDesc, foundBytes, err := oras.FetchBytes(ctx, srcStore, manifestDesc.Digest.String(), oras.DefaultFetchBytesOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch manifest for %s: %w", imageName, err)
		}

		logger.From(ctx).Info("pulling image from archive", "image", manifestImg.Reference, "archive", imageArchive.Path)

		var desc ocispec.Descriptor
		switch {
		case IsIndex(foundDesc.MediaType) && len(platforms) > 1:
			// Multi-platform request on an index: copy only the matching child manifests, then
			// tag a synthesized index that references just those.
			desc, err = unpackFilteredIndex(ctx, srcStore, dstStore, foundBytes, platforms, manifestImg.Reference)
			if err != nil {
				return nil, fmt.Errorf("failed to unpack filtered index for %s from archive %s: %w", manifestImg.Reference, imageArchive.Path, err)
			}
		default:
			copyOpts := oras.DefaultCopyOptions
			if IsIndex(foundDesc.MediaType) && len(platforms) == 1 {
				p := platforms[0]
				copyOpts.WithTargetPlatform(&p)
			}
			desc, err = oras.Copy(ctx, srcStore, manifestDesc.Digest.String(), dstStore, manifestImg.Reference, copyOpts)
			if err != nil {
				return nil, fmt.Errorf("failed to copy image %s from archive %s: %w", manifestImg.Reference, imageArchive.Path, err)
			}
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

// unpackFilteredIndex walks indexBytes (recursing through any nested indexes), copies every leaf
// manifest whose platform matches requested from src to dst, then synthesizes and pushes a new
// flat index referencing those leaves. Returns the descriptor of the synthesized index (untagged).
func unpackFilteredIndex(ctx context.Context, src, dst *oci.Store, indexBytes []byte, requested []ocispec.Platform, ref string) (ocispec.Descriptor, error) {
	kept, err := collectLeafManifests(ctx, src, indexBytes, requested)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	if len(kept) == 0 {
		return ocispec.Descriptor{}, fmt.Errorf("no manifests in archive index for %s match requested platforms", ref)
	}
	for _, m := range kept {
		if _, err := oras.Copy(ctx, src, m.Digest.String(), dst, "", oras.DefaultCopyOptions); err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("failed to copy manifest %s: %w", m.Digest, err)
		}
	}
	newIdx := ocispec.Index{
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: kept,
	}
	newIdx.SchemaVersion = 2
	newIdxBytes, err := json.Marshal(newIdx)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to marshal synthesized index: %w", err)
	}
	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageIndex,
		Digest:    digest.FromBytes(newIdxBytes),
		Size:      int64(len(newIdxBytes)),
	}
	if err := dst.Push(ctx, desc, bytes.NewReader(newIdxBytes)); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to push synthesized index: %w", err)
	}
	return desc, nil
}

// collectLeafManifests walks indexBytes, recursing into any child that is itself an index, and
// returns the platform-matching leaf manifest descriptors. Nested-index descriptors are fetched
// from src to read their children.
func collectLeafManifests(ctx context.Context, src oras.ReadOnlyTarget, indexBytes []byte, requested []ocispec.Platform) ([]ocispec.Descriptor, error) {
	var idx ocispec.Index
	if err := json.Unmarshal(indexBytes, &idx); err != nil {
		return nil, fmt.Errorf("unable to unmarshal index: %w", err)
	}
	var leaves []ocispec.Descriptor
	for _, m := range idx.Manifests {
		if IsIndex(m.MediaType) {
			_, childBytes, err := oras.FetchBytes(ctx, src, m.Digest.String(), oras.DefaultFetchBytesOptions)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch nested index %s: %w", m.Digest, err)
			}
			childLeaves, err := collectLeafManifests(ctx, src, childBytes, requested)
			if err != nil {
				return nil, err
			}
			leaves = append(leaves, childLeaves...)
			continue
		}
		leaves = append(leaves, filterIndexManifests([]ocispec.Descriptor{m}, requested)...)
	}
	return leaves, nil
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
