// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"context"

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

// LoadBundle loads a bundle from an OCI image
func (op *ociProvider) LoadBundle(requestedPackages []string) ([]ocispec.Descriptor, error) {
	return op.PullBundle(op.dst, config.CommonOptions.OCIConcurrency, requestedPackages)
}

// LoadBundleMetadata loads a bundle's metadata from an OCI image
func (op *ociProvider) LoadBundleMetadata() ([]ocispec.Descriptor, error) {
	return op.PullBundleMetadata(op.dst)
}
