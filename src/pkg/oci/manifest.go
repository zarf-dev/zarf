// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import (
	"encoding/json"
	"path/filepath"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// Manifest wraps an OCI image manifest.
type Manifest struct{ v1.Manifest }

// Locate returns the layer whose title annotation or digest matches pathOrDigest.
func (manifest *Manifest) Locate(pathOrDigest string) v1.Descriptor {
	for _, layer := range manifest.Layers {
		if layer.Annotations[v1.AnnotationTitle] == filepath.ToSlash(pathOrDigest) || layer.Digest.Encoded() == pathOrDigest {
			return layer
		}
	}
	return v1.Descriptor{}
}

// MarshalJSON implements json.Marshaler.
func (manifest *Manifest) MarshalJSON() ([]byte, error) { return json.Marshal(manifest.Manifest) }
