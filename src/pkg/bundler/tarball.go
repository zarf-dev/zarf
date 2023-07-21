// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/mholt/archiver/v3"
	av4 "github.com/mholt/archiver/v4"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var blobsDir = filepath.Join("blobs", "sha256")

// PathMap is a map of absolute paths on disk to relative paths within packages/bundles
type PathMap map[string]string

type tarballProvider struct {
	ctx      context.Context
	src      string
	dst      string
	manifest *oci.ZarfOCIManifest
}

func (tp *tarballProvider) getBundleManifest() error {
	if tp.manifest != nil {
		return nil
	}

	if err := archiver.Extract(tp.src, "index.json", tp.dst); err != nil {
		return fmt.Errorf("failed to extract index.json from %s: %w", tp.src, err)
	}

	indexPath := filepath.Join(tp.dst, "index.json")

	defer os.Remove(indexPath)

	bytes, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}

	var index ocispec.Index

	if err := json.Unmarshal(bytes, &index); err != nil {
		return err
	}

	// due to logic during the bundle pull process, this index.json should only have one manifest
	bundleManifestDesc := index.Manifests[0]

	if len(index.Manifests) > 1 {
		return fmt.Errorf("expected only one manifest in index.json, found %d", len(index.Manifests))
	}

	manifestRelativePath := filepath.Join(blobsDir, bundleManifestDesc.Digest.Encoded())

	if err := archiver.Extract(tp.src, manifestRelativePath, tp.dst); err != nil {
		return fmt.Errorf("failed to extract %s from %s: %w", bundleManifestDesc.Digest.Encoded(), tp.src, err)
	}

	manifestPath := filepath.Join(tp.dst, manifestRelativePath)

	defer os.Remove(manifestPath)

	if err := shasMatch(manifestPath, bundleManifestDesc.Digest.Encoded()); err != nil {
		return err
	}

	bytes, err = os.ReadFile(manifestPath)
	if err != nil {
		return err
	}

	var manifest *oci.ZarfOCIManifest

	if err := json.Unmarshal(bytes, manifest); err != nil {
		return err
	}

	tp.manifest = manifest
	return nil
}

// LoadBundle loads a bundle from a tarball
func (tp *tarballProvider) LoadBundle(requestedPackages []string) ([]ocispec.Descriptor, error) {
	if err := tp.getBundleManifest(); err != nil {
		return nil, err
	}

	layersExtracted := []ocispec.Descriptor{}

	if len(requestedPackages) == 0 {
		if err := archiver.Unarchive(tp.src, tp.dst); err != nil {
			return nil, fmt.Errorf("failed to extract %s to %s: %w", tp.src, tp.dst, err)
		}
		layersExtracted = tp.manifest.Layers
		for _, layer := range tp.manifest.Layers {
			if layer.MediaType == ocispec.MediaTypeImageConfig {
				var manifest ocispec.Manifest
				path := filepath.Join(tp.dst, blobsDir, layer.Digest.Encoded())
				bytes, err := os.ReadFile(path)
				if err != nil {
					return nil, err
				}
				if err := json.Unmarshal(bytes, &manifest); err != nil {
					return nil, err
				}
				layersExtracted = append(layersExtracted, manifest.Layers...)
			}
		}
	}

	bundleYAMLSHA := tp.manifest.Locate(config.ZarfBundleYAML).Digest.Encoded()

	if err := archiver.Extract(tp.src, filepath.Join(blobsDir, bundleYAMLSHA), tp.dst); err != nil {
		return nil, fmt.Errorf("failed to extract %s from %s: %w", bundleYAMLSHA, tp.src, err)
	}

	return layersExtracted, nil
}

// LoadPackage loads a package from a tarball
func (tp *tarballProvider) LoadPackage(sha, destinationDir string) (PathMap, error) {
	if err := tp.getBundleManifest(); err != nil {
		return nil, err
	}

	format := av4.CompressedArchive{
		Compression: av4.Zstd{},
		Archival:    av4.Tar{},
	}

	ctx := context.TODO()

	sourceArchive, err := os.Open(tp.src)
	if err != nil {
		return nil, err
	}

	defer sourceArchive.Close()

	var manifest *oci.ZarfOCIManifest

	extractJSON := func(j any) func(context.Context, av4.File) error {
		return func(_ context.Context, file av4.File) error {
			stream, err := file.Open()
			if err != nil {
				return err
			}
			defer stream.Close()

			bytes, err := io.ReadAll(stream)
			if err != nil {
				return err
			}
			return json.Unmarshal(bytes, j)
		}
	}

	if err := format.Extract(ctx, sourceArchive, []string{filepath.Join(blobsDir, sha)}, extractJSON(manifest)); err != nil {
		return nil, err
	}

	extractLayer := func(_ context.Context, file av4.File) error {
		if file.IsDir() {
			return nil
		}
		stream, err := file.Open()
		if err != nil {
			return err
		}
		defer stream.Close()

		desc := helpers.Find(manifest.Layers, func(layer ocispec.Descriptor) bool {
			return layer.Digest.Encoded() == filepath.Base(file.NameInArchive)
		})

		path := desc.Annotations[ocispec.AnnotationTitle]

		size := desc.Size

		dst := filepath.Join(destinationDir, path)

		if err := utils.CreateDirectory(filepath.Dir(dst), 0700); err != nil {
			return err
		}

		target, err := os.Create(dst)
		if err != nil {
			return err
		}
		defer target.Close()

		written, err := io.Copy(target, stream)
		if err != nil {
			return err
		}
		if written != size {
			return fmt.Errorf("expected to write %d bytes to %s, wrote %d", size, path, written)
		}

		// TODO: also check sha256?
		return nil
	}

	layersToExtract := []string{}
	paths := make(PathMap)

	for _, layers := range manifest.Layers {
		layersToExtract = append(layersToExtract, filepath.Join(blobsDir, layers.Digest.Encoded()))
		paths[layers.Annotations[ocispec.AnnotationTitle]] = filepath.Join(destinationDir, blobsDir, layers.Digest.Encoded())
	}

	if err := format.Extract(ctx, sourceArchive, layersToExtract, extractLayer); err != nil {
		return nil, err
	}

	return paths, nil
}

// LoadBundleMetadata loads a bundle's metadata from a tarball
func (tp *tarballProvider) LoadBundleMetadata() (PathMap, error) {
	if err := tp.getBundleManifest(); err != nil {
		return nil, err
	}
	pathsToExtract := oci.BundleAlwaysPull

	paths := make(PathMap)

	for _, path := range pathsToExtract {
		layer := tp.manifest.Locate(path)
		pathInTarball := filepath.Join(blobsDir, layer.Digest.Encoded())
		paths[path] = filepath.Join(tp.dst, pathInTarball)
		if err := archiver.Extract(tp.src, pathInTarball, tp.dst); err != nil {
			return nil, fmt.Errorf("failed to extract %s from %s: %w", path, tp.src, err)
		}
	}
	return paths, nil
}

// TODO: move this to helpers/utils
func shasMatch(path, expected string) error {
	sha, err := utils.GetSHA256OfFile(path)
	if err != nil {
		return err
	}
	if sha != expected {
		return fmt.Errorf("expected sha256 of %s to be %s, found %s", path, expected, sha)
	}
	return nil
}
