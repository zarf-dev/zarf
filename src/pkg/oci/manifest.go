// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var (
	ZarfPackageIndexPath      = filepath.Join("images", "index.json")
	ZarfPackageLayoutPath     = filepath.Join("images", "oci-layout")
	ZarfPackageImagesBlobsDir = filepath.Join("images", "blobs", "sha256")
)

// ZarfOCIManifest is a wrapper around the OCI manifest
//
// it includes the path to the index.json, oci-layout, and image blobs.
// as well as a few helper functions for locating layers and calculating the size of the layers.
type ZarfOCIManifest struct {
	ocispec.Manifest
}

// NewZarfOCIManifest returns a new ZarfOCIManifest.
func NewZarfOCIManifest(manifest *ocispec.Manifest) *ZarfOCIManifest {
	return &ZarfOCIManifest{*manifest}
}

// Locate returns the descriptor for the layer with the given path.
func (m *ZarfOCIManifest) Locate(uri string) ocispec.Descriptor {
	return helpers.Find(m.Layers, func(layer ocispec.Descriptor) bool {
		return layer.Annotations[ocispec.AnnotationTitle] == uri || layer.Digest.Encoded() == uri
	})
}

// SumLayersSize returns the sum of the size of all the layers in the manifest.
func (m *ZarfOCIManifest) SumLayersSize() int64 {
	var sum int64
	for _, layer := range m.Layers {
		sum += layer.Size
	}
	return sum
}
