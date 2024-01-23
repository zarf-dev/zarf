// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"encoding/json"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// OCIManifest is a wrapper around the OCI manifest
//
// it includes the path to the index.json, oci-layout, and image blobs.
// as well as a few helper functions for locating layers and calculating the size of the layers.
type OCIManifest struct {
	ocispec.Manifest
}

// New returns a new OCIManifest
func New(manifest *ocispec.Manifest) *OCIManifest {
	return &OCIManifest{*manifest}
}

// Locate returns the descriptor for the first layer with the given path or digest.
func (m *OCIManifest) Locate(pathOrDigest string) ocispec.Descriptor {
	return helpers.Find(m.Layers, func(layer ocispec.Descriptor) bool {
		// Convert from the OS path separator to the standard '/' for Windows support
		return layer.Annotations[ocispec.AnnotationTitle] == filepath.ToSlash(pathOrDigest) || layer.Digest.Encoded() == pathOrDigest
	})
}

// SumLayersSize returns the sum of the size of all the layers in the manifest.
func (m *OCIManifest) SumLayersSize() int64 {
	var sum int64
	for _, layer := range m.Layers {
		sum += layer.Size
	}
	return sum
}

// MarshalJSON returns the JSON encoding of the manifest.
func (m *OCIManifest) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Manifest)
}
