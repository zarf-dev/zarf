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

// Manifest is a wrapper around the OCI manifest
//
// it includes the path to the index.json, oci-layout, and image blobs.
// as well as a few helper functions for locating layers and calculating the size of the layers.
type Manifest struct {
	ocispec.Manifest
}

// New returns a new OCIManifest
func New(manifest *ocispec.Manifest) *Manifest {
	return &Manifest{*manifest}
}

// Locate returns the descriptor for the first layer with the given path or digest.
func (m *Manifest) Locate(pathOrDigest string) ocispec.Descriptor {
	return helpers.Find(m.Layers, func(layer ocispec.Descriptor) bool {
		// Convert from the OS path separator to the standard '/' for Windows support
		return layer.Annotations[ocispec.AnnotationTitle] == filepath.ToSlash(pathOrDigest) || layer.Digest.Encoded() == pathOrDigest
	})
}

// MarshalJSON returns the JSON encoding of the manifest.
func (m *Manifest) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Manifest)
}

// GetLayers returns all the layers in the manifest
func (m *Manifest) GetLayers(include func(d ocispec.Descriptor) bool) []ocispec.Descriptor {
	var layers []ocispec.Descriptor
	for _, layer := range m.Layers {
		if include != nil && include(layer) {
			layers = append(layers, layer)
		} else if include == nil {
			layers = append(layers, layer)
		}
	}
	layers = append(layers, m.Config)
	return layers
}
