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

	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/mholt/archiver/v3"
	av4 "github.com/mholt/archiver/v4"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	ocistore "oras.land/oras-go/v2/content/oci"
)

var blobsDir = filepath.Join("blobs", "sha256")

type tarballProvider struct {
	ctx      context.Context
	src      string
	dst      string
	manifest *oci.ZarfOCIManifest
}

func extractJSON(j any) func(context.Context, av4.File) error {
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
		return json.Unmarshal(bytes, &j)
	}
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

	if err := utils.SHAsMatch(manifestPath, bundleManifestDesc.Digest.Encoded()); err != nil {
		return err
	}

	bytes, err = os.ReadFile(manifestPath)
	if err != nil {
		return err
	}

	var manifest *oci.ZarfOCIManifest

	if err := json.Unmarshal(bytes, &manifest); err != nil {
		return err
	}

	tp.manifest = manifest
	return nil
}

// LoadBundle loads a bundle from a tarball
func (tp *tarballProvider) LoadBundle(_ int) (PathMap, error) {
	loaded := make(PathMap)

	if err := tp.getBundleManifest(); err != nil {
		return nil, err
	}

	store, err := ocistore.NewWithContext(tp.ctx, tp.dst)
	if err != nil {
		return nil, err
	}

	layersToExtract := []ocispec.Descriptor{}

	format := av4.CompressedArchive{
		Compression: av4.Zstd{},
		Archival:    av4.Tar{},
	}

	sourceArchive, err := os.Open(tp.src)
	if err != nil {
		return nil, err
	}

	defer sourceArchive.Close()

	for _, layer := range tp.manifest.Layers {
		if layer.MediaType == ocispec.MediaTypeImageManifest {
			var manifest *oci.ZarfOCIManifest
			if err := format.Extract(tp.ctx, sourceArchive, []string{filepath.Join(blobsDir, layer.Digest.Encoded())}, extractJSON(manifest)); err != nil {
				return nil, err
			}
			layersToExtract = append(layersToExtract, layer)
			layersToExtract = append(layersToExtract, manifest.Layers...)
		} else if layer.MediaType == oci.ZarfLayerMediaTypeBlob {
			rel := layer.Annotations[ocispec.AnnotationTitle]
			layersToExtract = append(layersToExtract, layer)
			loaded[rel] = filepath.Join(tp.dst, blobsDir, layer.Digest.Encoded())
		}
	}

	cacheFunc := func(ctx context.Context, file av4.File) error {
		desc := helpers.Find(layersToExtract, func(layer ocispec.Descriptor) bool {
			return layer.Digest.Encoded() == filepath.Base(file.NameInArchive)
		})
		r, err := file.Open()
		if err != nil {
			return err
		}
		defer r.Close()
		return store.Push(ctx, desc, r)
	}

	pathsInArchive := []string{}
	for _, layer := range layersToExtract {
		sha := layer.Digest.Encoded()
		if layer.MediaType == oci.ZarfLayerMediaTypeBlob {
			pathsInArchive = append(pathsInArchive, filepath.Join(blobsDir, sha))
			loaded[sha] = filepath.Join(tp.dst, blobsDir, sha)
		}
	}

	if err := format.Extract(tp.ctx, sourceArchive, pathsInArchive, cacheFunc); err != nil {
		return nil, err
	}

	return loaded, nil
}

// LoadPackage loads a package from a tarball
func (tp *tarballProvider) LoadPackage(sha, destinationDir string, _ int) (PathMap, error) {
	if err := tp.getBundleManifest(); err != nil {
		return nil, err
	}

	format := av4.CompressedArchive{
		Compression: av4.Zstd{},
		Archival:    av4.Tar{},
	}

	sourceArchive, err := os.Open(tp.src)
	if err != nil {
		return nil, err
	}

	defer sourceArchive.Close()

	var manifest *oci.ZarfOCIManifest

	if err := format.Extract(tp.ctx, sourceArchive, []string{filepath.Join(blobsDir, sha)}, extractJSON(manifest)); err != nil {
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

		return nil
	}

	layersToExtract := []string{}
	loaded := make(PathMap)

	for _, layers := range manifest.Layers {
		layersToExtract = append(layersToExtract, filepath.Join(blobsDir, layers.Digest.Encoded()))
		loaded[layers.Annotations[ocispec.AnnotationTitle]] = filepath.Join(destinationDir, blobsDir, layers.Digest.Encoded())
	}

	if err := format.Extract(tp.ctx, sourceArchive, layersToExtract, extractLayer); err != nil {
		return nil, err
	}

	return loaded, nil
}

// LoadBundleMetadata loads a bundle's metadata from a tarball
func (tp *tarballProvider) LoadBundleMetadata() (PathMap, error) {
	if err := tp.getBundleManifest(); err != nil {
		return nil, err
	}
	pathsToExtract := BundleAlwaysPull

	loaded := make(PathMap)

	for _, path := range pathsToExtract {
		layer := tp.manifest.Locate(path)
		if !oci.IsEmptyDescriptor(layer) {
			pathInTarball := filepath.Join(blobsDir, layer.Digest.Encoded())
			abs := filepath.Join(tp.dst, pathInTarball)
			loaded[path] = abs
			if !utils.InvalidPath(abs) && utils.SHAsMatch(abs, layer.Digest.Encoded()) == nil {
				continue
			}
			if err := archiver.Extract(tp.src, pathInTarball, tp.dst); err != nil {
				return nil, fmt.Errorf("failed to extract %s from %s: %w", path, tp.src, err)
			}
		}
	}
	return loaded, nil
}
