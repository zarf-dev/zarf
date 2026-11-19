/*
Copyright The ORAS Authors.
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

package properties

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/oras-project/oras-go/v3/errdef"
)

// regular expressions for components.
var (
	// repositoryRegexp is adapted from the distribution implementation. The
	// repository name set under OCI distribution spec is a subset of the docker
	// spec. For maximum compatibility, the docker spec is verified client-side.
	// Further checks are left to the server-side.
	//
	// References:
	//   - https://github.com/distribution/distribution/blob/v2.7.1/reference/regexp.go#L53
	//   - https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#pulling-manifests
	repositoryRegexp = regexp.MustCompile(`^[a-z0-9]+(?:(?:[._]|__|[-]*)[a-z0-9]+)*(?:/[a-z0-9]+(?:(?:[._]|__|[-]*)[a-z0-9]+)*)*$`)

	// tagRegexp checks the tag name.
	// The docker and OCI spec have the same regular expression.
	//
	// Reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#pulling-manifests
	tagRegexp = regexp.MustCompile(`^[\w][\w.-]{0,127}$`)
)

// Reference represents a reference to a remote registry artifact.
// It contains the registry host, repository name, and optional tag or digest.
type Reference struct {
	// Registry is the name of the registry. It is usually the domain name of
	// the registry optionally with a port.
	Registry string

	// Repository is the name of the repository.
	Repository string

	// Tag is the tag of the artifact, if specified.
	// Either Tag or Digest may be set, but not both.
	Tag string

	// Digest is the digest of the artifact, if specified.
	// Either Tag or Digest may be set, but not both.
	Digest string
}

// NewReference parses a string (artifact) into a Reference.
// Corresponding cryptographic hash implementations are required to be imported
// as specified by https://pkg.go.dev/github.com/opencontainers/go-digest#readme-usage
// if the string contains a digest.
//
// ## URI Schemes
//
// NewReference automatically strips the following URI schemes if present:
//   - oci://    (used by Helm, Argo, Kustomize)
//   - http://   (HTTP registry endpoints)
//   - https://  (HTTPS registry endpoints)
//
// Schemes must be lowercase and at the start of the string. Examples:
//   - "oci://ghcr.io/repo:tag" → parses as "ghcr.io/repo:tag"
//   - "https://registry.example.com/repo" → parses as "registry.example.com/repo"
//
// ## Reference Forms
//
// The reference string can take one of four forms:
//
//	Form A: registry/repository@digest        (digest only)
//	Form B: registry/repository:tag@digest    (tag and digest, digest takes precedence)
//	Form C: registry/repository:tag           (tag only)
//	Form D: registry/repository               (no tag or digest)
//
// In Form B, both Tag and Digest fields are populated.
//
// The registry host is case-insensitive (DNS), so it is lower-cased during
// parsing. Repository, tag and digest are preserved as written.
func NewReference(artifact string) (Reference, error) {
	// Strip URI schemes if present
	artifact = strings.TrimPrefix(artifact, "oci://")
	artifact = strings.TrimPrefix(artifact, "http://")
	artifact = strings.TrimPrefix(artifact, "https://")

	registry, path := splitRegistry(artifact)
	if path == "" {
		return Reference{}, fmt.Errorf("%w: missing registry or repository", errdef.ErrInvalidReference)
	}

	repository, digestStr, tag, err := splitRepository(path)
	if err != nil {
		return Reference{}, err
	}

	ref := Reference{
		Registry:   strings.ToLower(registry),
		Repository: repository,
		Tag:        tag,
		Digest:     digestStr,
	}

	if err := ref.ValidateRegistry(); err != nil {
		return Reference{}, err
	}

	if err := ref.ValidateRepository(); err != nil {
		return Reference{}, err
	}

	if ref.Digest != "" {
		if err := ref.ValidateDigest(); err != nil {
			return Reference{}, err
		}
	}
	if ref.Tag != "" {
		if err := ref.ValidateTag(); err != nil {
			return Reference{}, err
		}
	}

	return ref, nil
}

// NewReferenceList parses a string containing a base reference with
// comma-separated tags or digests and returns a slice of references.
//
// The input string can be in one of two formats:
//   - Tag list: "registry/repository:tag1,tag2,tag3"
//   - Digest list: "registry/repository@digest1,digest2,digest3"
//
// Examples:
//   - "localhost:5000/hello:v1,v2,v3"
//   - "localhost:5000/hello@sha256:digest1,sha256:digest2"
//
// All references in the list must be of the same type (either all tags or all digests),
// so a tag list followed by a digest, such as "repository:tag1,tag2@digest", is rejected.
// A single "repository:tag@digest" is returned as one reference with both the tag and
// the digest, and its tag is trimmed and validated like the tags in a list.
func NewReferenceList(artifact string) ([]Reference, error) {
	// Strip URI schemes if present
	artifact = strings.TrimPrefix(artifact, "oci://")
	artifact = strings.TrimPrefix(artifact, "http://")
	artifact = strings.TrimPrefix(artifact, "https://")

	registry, path := splitRegistry(artifact)
	if path == "" {
		return nil, fmt.Errorf("%w: missing registry or repository", errdef.ErrInvalidReference)
	}

	repository, digestRef, tagRef, err := splitRepository(path)
	if err != nil {
		return nil, err
	}
	if tagRef != "" && digestRef != "" {
		// A single tag with a digest is one reference; a list cannot share one.
		if strings.Contains(tagRef, ",") {
			return nil, fmt.Errorf("%w: a list of tags cannot be combined with a digest", errdef.ErrInvalidReference)
		}
		tag := strings.TrimSpace(tagRef)
		ref, err := NewReference(registry + "/" + repository + ":" + tag + "@" + digestRef)
		if err != nil {
			return nil, err
		}
		if err := ref.ValidateTag(); err != nil {
			return nil, err
		}
		return []Reference{ref}, nil
	}

	// Determine if we have tags or digests
	references := tagRef
	isTag := tagRef != ""
	if !isTag {
		references = digestRef
	}

	delimiter := "@"
	if isTag {
		delimiter = ":"
	}
	base := registry + "/" + repository

	// Split the reference part by commas
	refItems := strings.Split(references, ",")
	if len(refItems) == 0 {
		return nil, fmt.Errorf("%w: empty reference list", errdef.ErrInvalidReference)
	}

	// Create a Reference for each item
	refs := make([]Reference, 0, len(refItems))
	for _, item := range refItems {
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, fmt.Errorf("%w: empty reference in list", errdef.ErrInvalidReference)
		}

		// Construct the full reference string
		fullRef := base + delimiter + item

		// Parse the reference
		ref, err := NewReference(fullRef)
		if err != nil {
			return nil, fmt.Errorf("failed to parse reference %q: %w", fullRef, err)
		}

		refs = append(refs, ref)
	}

	return refs, nil
}

// splitRepository splits the path into repository, digest, and tag.
func splitRepository(path string) (repository, digestStr, tag string, err error) {
	repository = path
	if index := strings.Index(path, "@"); index != -1 {
		repository = path[:index]
		digestStr = path[index+1:]
		if digestStr == "" {
			return "", "", "", fmt.Errorf("%w: missing digest", errdef.ErrInvalidReference)
		}
	}

	if index := strings.Index(repository, ":"); index != -1 {
		tag = repository[index+1:]
		repository = repository[:index]
		// A digest takes precedence over the tag, including an empty tag.
		if tag == "" && digestStr == "" {
			return "", "", "", fmt.Errorf("%w: missing tag", errdef.ErrInvalidReference)
		}
	}

	return repository, digestStr, tag, nil
}

func splitRegistry(artifact string) (string, string) {
	parts := strings.SplitN(artifact, "/", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

// Validate validates the entire reference.
func (r Reference) Validate() error {
	if err := r.ValidateRegistry(); err != nil {
		return err
	}
	if err := r.ValidateRepository(); err != nil {
		return err
	}
	if r.Digest != "" {
		if err := r.ValidateDigest(); err != nil {
			return err
		}
	}
	if r.Tag != "" {
		if err := r.ValidateTag(); err != nil {
			return err
		}
	}
	return nil
}

// ValidateRegistry validates the registry.
func (r Reference) ValidateRegistry() error {
	uri, err := url.ParseRequestURI("dummy://" + r.Registry)
	if err != nil || uri.Host == "" || uri.Host != r.Registry {
		return fmt.Errorf("%w: invalid registry %q", errdef.ErrInvalidReference, r.Registry)
	}
	if strings.Contains(uri.Hostname(), ":") && !strings.HasPrefix(r.Registry, "[") {
		return fmt.Errorf("%w: invalid registry %q", errdef.ErrInvalidReference, r.Registry)
	}
	return nil
}

// ValidateRepository validates the repository.
func (r Reference) ValidateRepository() error {
	if !repositoryRegexp.MatchString(r.Repository) {
		return fmt.Errorf("%w: invalid repository %q", errdef.ErrInvalidReference, r.Repository)
	}
	return nil
}

// ValidateTag validates the tag.
func (r Reference) ValidateTag() error {
	if !tagRegexp.MatchString(r.Tag) {
		return fmt.Errorf("%w: invalid tag %q", errdef.ErrInvalidReference, r.Tag)
	}
	return nil
}

// ValidateDigest validates the digest.
func (r Reference) ValidateDigest() error {
	if _, err := digest.Parse(r.Digest); err != nil {
		return fmt.Errorf("%w: invalid digest %q: %v", errdef.ErrInvalidReference, r.Digest, err)
	}
	return nil
}

// Host returns the host name of the registry.
// For docker.io, it returns registry-1.docker.io.
func (r Reference) Host() string {
	if r.Registry == "docker.io" {
		return "registry-1.docker.io"
	}
	return r.Registry
}

// Resource returns the registry resource addressed by r, dropping any tag or
// digest. The registry name is lower-cased, as registry host names are
// case-insensitive.
func (r Reference) Resource() Resource {
	return Resource{
		Registry: strings.ToLower(r.Registry),
		Path:     r.Repository,
	}
}

// GetReference returns the reference string (digest if present, otherwise tag).
func (r Reference) GetReference() string {
	if r.Digest != "" {
		return r.Digest
	}
	return r.Tag
}

// ReferenceOrDefault returns the reference or "latest" if empty.
func (r Reference) ReferenceOrDefault() string {
	if ref := r.GetReference(); ref != "" {
		return ref
	}
	return "latest"
}

// GetDigest returns the digest as a digest.Digest type.
// Returns an error if Digest is empty or invalid.
func (r Reference) GetDigest() (digest.Digest, error) {
	if r.Digest == "" {
		return "", fmt.Errorf("%w: no digest in reference", errdef.ErrInvalidReference)
	}
	return digest.Parse(r.Digest)
}

// String returns the reference as a string.
// The format is: registry/repository[:tag][@digest]
func (r Reference) String() string {
	if r.Repository == "" {
		return r.Registry
	}
	ref := r.Registry + "/" + r.Repository
	if r.Tag != "" {
		ref += ":" + r.Tag
	}
	if r.Digest != "" {
		ref += "@" + r.Digest
	}
	return ref
}
