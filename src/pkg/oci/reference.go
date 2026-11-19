// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import "github.com/oras-project/oras-go/v3/registry/remote/properties"

// Reference identifies a registry repository and, optionally, an artifact.
// A reference may contain both a tag and a digest; reads select the digest.
type Reference struct {
	Registry   string
	Repository string
	Tag        string
	Digest     string
}

// ParseReference parses a repository or artifact reference, preserving both its tag and digest.
// An optional URI scheme does not configure the transport used to access the registry.
func ParseReference(raw string) (Reference, error) {
	ref, err := properties.NewReference(raw)
	return Reference(ref), err
}

// String returns registry/repository[:tag][@digest], or just the registry when no repository is set.
func (r Reference) String() string {
	return properties.Reference(r).String()
}

// GetReference returns the digest when present, otherwise the tag.
func (r Reference) GetReference() string {
	return properties.Reference(r).GetReference()
}

// Validate validates the registry, repository, and any tag and digest.
func (r Reference) Validate() error {
	return properties.Reference(r).Validate()
}

// ValidateRegistry validates the registry, allowing a destination without a repository.
func (r Reference) ValidateRegistry() error {
	return properties.Reference(r).ValidateRegistry()
}
