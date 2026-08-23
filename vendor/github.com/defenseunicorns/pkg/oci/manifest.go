// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024-Present Defense Unicorns

package oci

import (
	"encoding/json"
	"path/filepath"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/defenseunicorns/pkg/helpers/v2"
)

// Manifest is a wrapper around the OCI manifest
type Manifest struct {
	ocispec.Manifest
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
