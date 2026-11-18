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

package registry

import (
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/oras-project/oras-go/v3/registry/remote/properties"
)

// Reference references either a resource descriptor (where Reference.Reference
// is a tag or a digest), or a resource repository (where Reference.Reference
// is the empty string).
//
// Deprecated: Use [github.com/oras-project/oras-go/v3/registry/remote/properties.Reference] instead.
// This type will be removed in a future version.
type Reference struct {
	// Registry is the name of the registry. It is usually the domain name of
	// the registry optionally with a port.
	Registry string

	// Repository is the name of the repository.
	Repository string

	// Reference is the reference of the object in the repository. This field
	// can take any one of the four valid forms (see ParseReference). In the
	// case where it's the empty string, it necessarily implies valid form D,
	// and where it is non-empty, then it is either a tag, or a digest
	// (implying one of valid forms A, B, or C).
	//
	// Deprecated: Use GetReference() method instead to retrieve the reference string.
	// For specific access to tag or digest values, use the Tag or Digest fields.
	Reference string

	// Tag is the tag of the object in the repository, if a tag was provided.
	// This field is empty if only a digest was provided (Form A) or if no reference was provided (Form D).
	Tag string

	// Digest is the digest of the object in the repository, if a digest was provided.
	// This field is empty if only a tag was provided (Form C) or if no reference was provided (Form D).
	Digest string
}

// ParseReference parses a string (artifact) into an `artifact reference`.
// Corresponding cryptographic hash implementations are required to be imported
// as specified by https://pkg.go.dev/github.com/opencontainers/go-digest#readme-usage
// if the string contains a digest.
//
// Note: An "image" is an "artifact", however, an "artifact" is not necessarily
// an "image".
//
// ## URI Schemes
//
// ParseReference automatically strips the following URI schemes if present:
//   - oci://    (used by Helm, Argo, Kustomize)
//   - http://   (HTTP registry endpoints)
//   - https://  (HTTPS registry endpoints)
//
// Schemes must be lowercase and at the start of the string. Examples:
//   - "oci://ghcr.io/repo:tag" → parses as "ghcr.io/repo:tag"
//   - "https://registry.example.com/repo" → parses as "registry.example.com/repo"
//
// ## Specification
//
// The token `artifact` is composed of other tokens, and those in turn are
// composed of others.  This definition recursivity requires a notation capable
// of recursion, thus the following two forms have been adopted:
//
//  1. Backus–Naur Form (BNF) has been adopted to address the recursive nature
//     of the definition.
//  2. Token opacity is revealed via its label letter-casing.  That is, "opaque"
//     tokens (i.e., tokens that are not final, and must therefore be further
//     broken down into their constituents) are denoted in *lowercase*, while
//     final tokens (i.e., leaf-node tokens that are final) are denoted in
//     *uppercase*.
//
// Finally, note that a number of the opaque tokens are polymorphic in nature;
// that is, they can take on one of numerous forms, not restricted to a single
// defining form.
//
// The top-level token, `artifact`, is composed of two (opaque) tokens; namely
// `socketaddr` and `path`:
//
//	<artifact> ::= <socketaddr> "/" <path>
//
// The former is described as follows:
//
//	   <socketaddr> ::= <host> | <host> ":" <PORT>
//		     <host> ::= <ip> | <FQDN>
//		       <ip> ::= <IPV4-ADDR> | <IPV6-ADDR>
//
// The latter, which is of greater interest here, is described as follows:
//
//	     <path> ::= <REPOSITORY> | <REPOSITORY> <reference>
//	<reference> ::= "@" <digest> | ":" <TAG> "@" <DIGEST> | ":" <TAG>
//	   <digest> ::= <ALGO> ":" <HASH>
//
// This second token--`path`--can take on exactly four forms, each of which will
// now be illustrated:
//
//	<--- path --------------------------------------------> |  - Decode `path`
//	<=== REPOSITORY ===> <--- reference ------------------> |    - Decode `reference`
//	<=== REPOSITORY ===> @ <=================== digest ===> |      - Valid Form A
//	<=== REPOSITORY ===> : <=== TAG ===> @ <=== digest ===> |      - Valid Form B
//	<=== REPOSITORY ===> : <=== TAG ======================> |      - Valid Form C
//	<=== REPOSITORY ======================================> |    - Valid Form D
//
// Note: In the case of Valid Form B, the digest takes precedence in the Reference
// field, but the TAG is captured and stored in the Tag field.
//
// Deprecated: Use [github.com/oras-project/oras-go/v3/registry/remote/properties.NewReference] instead.
// This function will be removed in a future version.
func ParseReference(artifact string) (Reference, error) {
	p, err := properties.NewReference(artifact)
	if err != nil {
		return Reference{}, err
	}

	reference := p.Digest
	if reference == "" {
		reference = p.Tag
	}

	return Reference{
		Registry:   p.Registry,
		Repository: p.Repository,
		Reference:  reference,
		Tag:        p.Tag,
		Digest:     p.Digest,
	}, nil
}

// Validate the entire reference object; the registry, the repository, and the
// reference.
func (r Reference) Validate() error {
	if err := r.ValidateRegistry(); err != nil {
		return err
	}

	if err := r.ValidateRepository(); err != nil {
		return err
	}

	return r.ValidateReference()
}

// ValidateRegistry validates the registry.
func (r Reference) ValidateRegistry() error {
	return r.toProperties().ValidateRegistry()
}

// ValidateRepository validates the repository.
func (r Reference) ValidateRepository() error {
	return r.toProperties().ValidateRepository()
}

// ValidateReferenceAsTag validates the reference as a tag.
func (r Reference) ValidateReferenceAsTag() error {
	return properties.Reference{Tag: r.Reference}.ValidateTag()
}

// ValidateReferenceAsDigest validates the reference as a digest.
func (r Reference) ValidateReferenceAsDigest() error {
	return properties.Reference{Digest: r.Reference}.ValidateDigest()
}

// ValidateReference where the reference is first tried as an empty string, then
// as a digest, and if that fails, as a tag.
func (r Reference) ValidateReference() error {
	if len(r.Reference) == 0 {
		return nil
	}

	if index := strings.IndexByte(r.Reference, ':'); index != -1 {
		return r.ValidateReferenceAsDigest()
	}

	return r.ValidateReferenceAsTag()
}

// Host returns the host name of the registry.
func (r Reference) Host() string {
	return r.toProperties().Host()
}

// toProperties returns the registry and repository of r as a
// properties.Reference. Tag and Digest are left empty: the methods of r read
// the legacy Reference field, which can disagree with r.Tag and r.Digest, so
// ValidateReferenceAsTag and ValidateReferenceAsDigest build their own
// properties.Reference from r.Reference instead.
func (r Reference) toProperties() properties.Reference {
	return properties.Reference{
		Registry:   r.Registry,
		Repository: r.Repository,
	}
}

// ReferenceOrDefault returns the reference or the default reference if empty.
func (r Reference) ReferenceOrDefault() string {
	if r.Reference == "" {
		return "latest"
	}
	return r.Reference
}

// GetDigest returns the reference as a digest.
// Corresponding cryptographic hash implementations are required to be imported
// as specified by https://pkg.go.dev/github.com/opencontainers/go-digest#readme-usage
func (r Reference) GetDigest() (digest.Digest, error) {
	return digest.Parse(r.Reference)
}

// GetReference returns the reference string (either a tag or a digest).
// This method should be used instead of directly accessing the deprecated Reference field.
func (r Reference) GetReference() string {

	if r.Digest == "" {
		return r.Reference
	}
	return r.Digest
}

// String implements `fmt.Stringer` and returns the reference string.
// The resulted string is meaningful only if the reference is valid.
func (r Reference) String() string {
	if r.Repository == "" {
		return r.Registry
	}
	ref := r.Registry + "/" + r.Repository
	if r.Reference == "" {
		return ref
	}
	if d, err := r.GetDigest(); err == nil {
		return ref + "@" + d.String()
	}
	return ref + ":" + r.Reference
}
