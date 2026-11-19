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
	"context"
	"io"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/content"
	"github.com/oras-project/oras-go/v3/registry/remote/properties"
)

// Repository is an ORAS target and an union of the blob and the manifest CASs.
//
// As specified by https://distribution.github.io/distribution/spec/api/, it is natural to
// assume that content.Resolver interface only works for manifests. Tagging a
// blob may be resulted in an `ErrUnsupported` error. However, this interface
// does not restrict tagging blobs.
//
// Since a repository is an union of the blob and the manifest CASs, all
// operations defined in the `BlobStore` are executed depending on the media
// type of the given descriptor accordingly.
//
// Furthermore, this interface also provides the ability to enforce the
// separation of the blob and the manifests CASs.
type Repository interface {
	content.Storage
	content.Deleter
	content.TagResolver
	content.PredecessorFinder
	ReferenceFetcher
	ReferencePusher
	ReferrerLister
	TagLister

	// Blobs provides access to the blob CAS only, which contains config blobs,
	// layers, and other generic blobs.
	Blobs() BlobStore

	// Manifests provides access to the manifest CAS only.
	Manifests() ManifestStore
}

// BlobStore is a CAS with the ability to stat and delete its content.
type BlobStore interface {
	content.Storage
	content.Deleter
	content.Resolver
	ReferenceFetcher
}

// ManifestStore is a CAS with the ability to stat and delete its content.
// Besides, ManifestStore provides reference tagging and untagging.
type ManifestStore interface {
	BlobStore
	content.Tagger
	content.Untagger
	ReferencePusher
}

// ReferencePusher provides advanced push with the tag service.
type ReferencePusher interface {
	// PushReference pushes the manifest with a reference tag.
	PushReference(ctx context.Context, expected ocispec.Descriptor, content io.Reader, reference string) error
}

// ReferenceFetcher provides advanced fetch with the tag service.
type ReferenceFetcher interface {
	// FetchReference fetches the content identified by the reference.
	FetchReference(ctx context.Context, reference string) (ocispec.Descriptor, io.ReadCloser, error)
}

// ReferenceParser provides reference parsing.
//
// [github.com/oras-project/oras-go/v3.Tag] and
// [github.com/oras-project/oras-go/v3.TagN] check for this interface on
// targets that also implement [ReferenceFetcher] and [ReferencePusher], and
// use the parsed registry and repository as authentication scope hints.
type ReferenceParser interface {
	// ParseReference parses a reference to a fully qualified reference.
	ParseReference(reference string) (properties.Reference, error)
}

// ReferrerLister provides the Referrers API.
// Reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#listing-referrers
type ReferrerLister interface {
	Referrers(ctx context.Context, desc ocispec.Descriptor, artifactType string, fn func(referrers []ocispec.Descriptor) error) error
}

// TagLister lists tags by the tag service.
type TagLister interface {
	// Tags lists the tags available in the repository.
	// Since the returned tag list may be paginated by the underlying
	// implementation, a function should be passed in to process the paginated
	// tag list.
	//
	// `last` argument is the `last` parameter when invoking the tags API.
	// If `last` is NOT empty, the entries in the response start after the
	// tag specified by `last`. Otherwise, the response starts from the top
	// of the Tags list.
	//
	// Note: When implemented by a remote registry, the tags API is called.
	// However, not all registries supports pagination or conforms the
	// specification.
	//
	// References:
	//   - https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#content-discovery
	//   - https://distribution.github.io/distribution/spec/api/#tags
	// See also `Tags()` in this package.
	Tags(ctx context.Context, last string, fn func(tags []string) error) error
}

// Mounter allows cross-repository blob mounts.
// For backward compatibility reasons, this is not implemented by
// BlobStore: use a type assertion to check availability.
type Mounter interface {
	// Mount makes the blob with the given descriptor in fromRepo
	// available in the repository signified by the receiver.
	Mount(ctx context.Context,
		desc ocispec.Descriptor,
		fromRepo string,
		getContent func() (io.ReadCloser, error),
	) error
}
