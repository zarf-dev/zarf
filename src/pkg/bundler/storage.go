// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func (b *Bundler) LocateBundleYAML(dir string) (path string, err error) {

	var index ocispec.Index

	bytes, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		return path, err
	}

	if err := json.Unmarshal(bytes, &index); err != nil {
		return path, err
	}

	// due to logic during the bundle pull process, this index.json should only have one manifest
	bundleManifestDesc := index.Manifests[0]

	if len(index.Manifests) > 1 {
		return path, fmt.Errorf("expected only one manifest in index.json, found %d", len(index.Manifests))
	}

	bundleManifestPath := filepath.Join(dir, "blobs", "sha256", bundleManifestDesc.Digest.Encoded())

	if err := shasMatch(bundleManifestPath, bundleManifestDesc.Digest.Encoded()); err != nil {
		return path, err
	}

	bytes, err = os.ReadFile(bundleManifestPath)
	if err != nil {
		return path, err
	}

	var bundleManifest oci.ZarfOCIManifest

	if err := json.Unmarshal(bytes, &bundleManifest); err != nil {
		return path, err
	}

	bundleYAMLPath := filepath.Join(dir, "blobs", "sha256", bundleManifest.Locate(config.ZarfBundleYAML).Digest.Encoded())

	if err := shasMatch(bundleYAMLPath, bundleManifest.Locate(config.ZarfBundleYAML).Digest.Encoded()); err != nil {
		return path, err
	}

	return bundleYAMLPath, nil
}

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
