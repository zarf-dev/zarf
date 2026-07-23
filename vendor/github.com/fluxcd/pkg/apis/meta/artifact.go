/*
Copyright 2025 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package meta

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Artifact represents the output of a Source reconciliation.
type Artifact struct {
	// Digest is the digest of the file in the form of '<algorithm>:<checksum>'.
	// +required
	// +kubebuilder:validation:Pattern="^[a-z0-9]+(?:[.+_-][a-z0-9]+)*:[a-zA-Z0-9=_-]+$"
	Digest string `json:"digest,omitempty"`

	// Path is the relative file path of the Artifact. It can be used to locate
	// the file in the root of the Artifact storage on the local file system of
	// the controller managing the Source.
	// +required
	Path string `json:"path"`

	// URL is the HTTP address of the Artifact as exposed by the controller
	// managing the Source. It can be used to retrieve the Artifact for
	// consumption, e.g. by another controller applying the Artifact contents.
	// +required
	URL string `json:"url"`

	// Revision is a human-readable identifier traceable in the origin source
	// system. It can be a Git commit SHA, Git tag, a Helm chart version, etc.
	// +required
	Revision string `json:"revision"`

	// LastUpdateTime is the timestamp corresponding to the last update of the
	// Artifact.
	// +required
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`

	// Size is the number of bytes in the file.
	// +optional
	Size *int64 `json:"size,omitempty"`

	// Metadata holds upstream information such as OCI annotations.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
}

// HasRevision returns if the given revision matches the current Revision of
// the Artifact.
func (in *Artifact) HasRevision(revision string) bool {
	if in == nil {
		return false
	}
	return in.Revision == revision
}

// HasDigest returns if the given digest matches the current Digest of the
// Artifact.
func (in *Artifact) HasDigest(digest string) bool {
	if in == nil {
		return false
	}
	return in.Digest == digest
}
