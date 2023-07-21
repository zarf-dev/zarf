// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"context"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ociProvider is a Processor that works with OCI images
type ociProvider struct {
	ctx context.Context
	src string
	dst string
	*oci.OrasRemote
	manifest *oci.ZarfOCIManifest
}

func (op *ociProvider) getBundleManifest() error {
	if op.manifest != nil {
		return nil
	}
	root, err := op.FetchRoot()
	if err != nil {
		return err
	}
	bundleYamlDesc := root.Locate(config.ZarfBundleYAML)
	manifest, err := op.FetchManifest(bundleYamlDesc)
	if err != nil {
		return err
	}
	op.manifest = manifest
	return nil
}

func (op *ociProvider) LoadPackage(sha, destinationDir string) (PathMap, error) {
	layers, err := op.PullBundle(destinationDir, config.CommonOptions.OCIConcurrency, []string{sha})
	if err != nil {
		return nil, err
	}
	paths := make(PathMap)
	for _, layer := range layers {
		rel := layer.Annotations[ocispec.AnnotationTitle]
		paths[rel] = filepath.Join(destinationDir, rel)
	}
	return paths, nil
}

// LoadBundleMetadata loads a bundle's metadata from an OCI image
func (op *ociProvider) LoadBundleMetadata() (PathMap, error) {
	layers, err := op.PullBundleMetadata(op.dst)
	if err != nil {
		return nil, err
	}
	paths := make(PathMap)
	for _, layer := range layers {
		rel := layer.Annotations[ocispec.AnnotationTitle]
		paths[rel] = filepath.Join(op.dst, rel)
	}
	return paths, nil
}
