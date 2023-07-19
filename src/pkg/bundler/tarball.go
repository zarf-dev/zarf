// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/mholt/archiver/v3"
	av4 "github.com/mholt/archiver/v4"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

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

	manifestRelativePath := filepath.Join("blobs", "sha256", bundleManifestDesc.Digest.Encoded())

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
				path := filepath.Join(tp.dst, "blobs", "sha256", layer.Digest.Encoded())
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

	if err := archiver.Extract(tp.src, filepath.Join("blobs", "sha256", bundleYAMLSHA), tp.dst); err != nil {
		return nil, fmt.Errorf("failed to extract %s from %s: %w", bundleYAMLSHA, tp.src, err)
	}

	// TODO: finish me
	// for _, pkg := range bundle.Packages {
	// 	manifestSha256 := strings.Split(pkg.Ref, "@sha256:")[1]
	// }

	// if err := utils.ReadYaml(filepath.Join(dst, config.ZarfBundleYAML), &bundle); err != nil {
	// 	return nil, err
	// }

	return layersExtracted, nil
}

func (tp *tarballProvider) LoadPackage(sha string) ([]ocispec.Descriptor, error) {
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

	if err := format.Extract(ctx, sourceArchive, packagePaths, handler); err != nil {
		return nil, err
	}

	return nil, nil
}

// LoadBundleMetadata loads a bundle's metadata from a tarball
func (tp *tarballProvider) LoadBundleMetadata() ([]ocispec.Descriptor, error) {
	if err := tp.getBundleManifest(); err != nil {
		return nil, err
	}
	pathsToExtract := oci.BundleAlwaysPull

	layersExtracted := []ocispec.Descriptor{}

	for _, path := range pathsToExtract {
		layer := tp.manifest.Locate(path)
		layersExtracted = append(layersExtracted, layer)
		pathInTarball := filepath.Join("blobs", "sha256", layer.Digest.Encoded())
		if err := archiver.Extract(tp.src, pathInTarball, tp.dst); err != nil {
			return nil, fmt.Errorf("failed to extract %s from %s: %w", path, tp.src, err)
		}
	}
	return layersExtracted, nil
}
