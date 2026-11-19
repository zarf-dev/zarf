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

package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/content"
	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/internal/cas"
	"github.com/oras-project/oras-go/v3/internal/httputil"
	"github.com/oras-project/oras-go/v3/internal/ioutil"
	"github.com/oras-project/oras-go/v3/internal/spec"
	"github.com/oras-project/oras-go/v3/internal/syncutil"
	"github.com/oras-project/oras-go/v3/registry"
	"github.com/oras-project/oras-go/v3/registry/remote/auth"
	"github.com/oras-project/oras-go/v3/registry/remote/errcode"
	"github.com/oras-project/oras-go/v3/registry/remote/internal/errutil"
	"github.com/oras-project/oras-go/v3/registry/remote/internal/urlutil"
	"github.com/oras-project/oras-go/v3/registry/remote/policy"
	"github.com/oras-project/oras-go/v3/registry/remote/properties"
)

const (
	// headerDockerContentDigest is the "Docker-Content-Digest" header.
	// If present on the response, it contains the canonical digest of the
	// uploaded blob.
	//
	// References:
	//   - https://distribution.github.io/distribution/spec/api/#digest-header
	//   - https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#pull
	headerDockerContentDigest = "Docker-Content-Digest"

	// headerOCIFiltersApplied is the "OCI-Filters-Applied" header.
	// If present on the response, it contains a comma-separated list of the
	// applied filters.
	//
	// Reference:
	//   - https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#listing-referrers
	headerOCIFiltersApplied = "OCI-Filters-Applied"

	// headerOCISubject is the "OCI-Subject" header.
	// If present on the response, it contains the digest of the subject,
	// indicating that Referrers API is supported by the registry.
	headerOCISubject = "OCI-Subject"
)

// filterTypeArtifactType is the "artifactType" filter applied on the list of
// referrers.
//
// References:
//   - Latest spec: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#listing-referrers
//   - Compatible spec: https://github.com/opencontainers/distribution-spec/blob/v1.1.0-rc1/spec.md#listing-referrers
const filterTypeArtifactType = "artifactType"

func override[T comparable](child, parent T) T {
	var zero T
	if child != zero {
		return child
	}
	return parent
}

func overrideSlice[T any](child, parent []T) []T {
	if len(child) > 0 {
		return child
	}
	return parent
}

// Client is an interface for a HTTP client.
type Client interface {
	// Do sends an HTTP request and returns an HTTP response.
	//
	// Unlike http.RoundTripper, Client can attempt to interpret the response
	// and handle higher-level protocol details such as redirects and
	// authentication.
	//
	// Like http.RoundTripper, Client should not modify the request, and must
	// always close the request body.
	Do(*http.Request) (*http.Response, error)
}

// Compile-time checks for the optional interfaces probed by the oras package.
var (
	_ registry.ReferenceParser = (*Repository)(nil)
	_ registry.ReferenceParser = (*manifestStore)(nil)
)

var _ content.UnsafePusher = (*Repository)(nil)

// Repository is an HTTP client to a remote repository.
type Repository struct {
	// Registry is the parent registry. Must not be nil.
	// Registry holds shared configuration like Client, PlainHTTP, Policy, etc.
	Registry *Registry

	// RepositoryName is the name within the registry (e.g., "library/alpine").
	RepositoryName string

	// ManifestMediaTypes overrides Registry default if set.
	// Used in `Accept` header for resolving manifests from references.
	// It is also used in identifying manifests and blobs from descriptors.
	// If an empty list is present, default manifest media types are used.
	ManifestMediaTypes []string

	// TagListPageSize overrides Registry default if set.
	// Specifies the page size when invoking the tag list API.
	// If zero, the page size is determined by the remote registry.
	// Reference: https://distribution.github.io/distribution/spec/api/#tags
	TagListPageSize int

	// ReferrerListPageSize overrides Registry default if set.
	// Specifies the page size when invoking the Referrers API.
	// If zero, the page size is determined by the remote registry.
	//
	// NOTE: Pagination for the Referrers API is not defined in the distribution
	// spec, so not all registries support it. ReferrerListPageSize may be
	// ignored if pagination is unsupported by the remote registry.
	//
	// Reference: https://github.com/oras-project/oras-go/issues/841
	ReferrerListPageSize int

	// TagListMaxPages overrides Registry default if > 0.
	// Limits the total number of pages fetched during tag listing.
	// If 0, Registry.TagListMaxPages is used. If that is also 0, there is no
	// limit.
	TagListMaxPages int

	// ReferrerListMaxPages overrides Registry default if > 0.
	// Limits the total number of pages fetched during referrer listing.
	// Zero means to use Registry.ReferrerListMaxPages. Referrer listing is
	// unlimited only if the effective Registry/default value is also zero.
	ReferrerListMaxPages int

	// SkipReferrersGC overrides Registry default if set.
	// Specifies whether to delete the dangling referrers
	// index when referrers tag schema is utilized.
	//  - If false, the old referrers index will be deleted after the new one
	//    is successfully uploaded.
	//  - If true, the old referrers index is kept.
	// By default, it is disabled (set to false). See also:
	//  - https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#referrers-tag-schema
	//  - https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#pushing-manifests-with-subject
	//  - https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#deleting-manifests
	SkipReferrersGC bool

	// MaxChunkSize overrides Registry default if > 0.
	// It specifies the target size in bytes for each PATCH chunk when
	// pushing a blob in chunks.
	//
	// If zero or negative (the default), Registry.MaxChunkSize is used. If
	// both settings are non-positive, blobs are pushed monolithically in a
	// single request, preserving the previous behavior. If positive, a blob
	// larger than the effective chunk size is uploaded in chunks per the OCI
	// Distribution Spec: an upload session is opened, the content is streamed in
	// PATCH requests, and the session is closed with a final PUT. A
	// registry-advertised OCI-Chunk-Min-Length raises this floor for all but the
	// final chunk, up to a fixed limit (currently 32 MiB) since each chunk is
	// buffered in memory; a larger advertised minimum makes the push fall back
	// to a monolithic upload. If the chunked upload cannot be started (e.g. the
	// registry does not support it), the push transparently falls back to a
	// monolithic upload before any content is consumed.
	//
	// MaxChunkSize applies to [Repository.Push] only. Blob mount falls back to a
	// monolithic upload when the registry does not implement the mount endpoint,
	// regardless of this setting.
	//
	// Reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#pushing-a-blob-in-chunks
	MaxChunkSize int64

	// SingleRequestPushThreshold overrides Registry default if > 0.
	// It specifies the largest blob size in bytes that is pushed in a single
	// POST request carrying the content, instead of a POST that opens an
	// upload session followed by a PUT.
	//
	// If zero or negative (the default), Registry.SingleRequestPushThreshold is
	// used. If both settings are non-positive, blobs are never pushed in a
	// single request, preserving the previous behavior. If positive, a
	// non-empty blob no larger than the threshold is read into memory, so the
	// request can be replayed after an authentication challenge or by a
	// retrying transport, and sent per the OCI Distribution Spec as
	// POST ?digest=. If the registry ignores the digest and opens an upload
	// session instead, the blob is uploaded to that session with a PUT. If the
	// registry rejects the request with a 4xx status other than 401, 403 or
	// 429, or with 501, the push falls back to a POST followed by a PUT. After
	// a 202, or a 405, 415 or 501 that says the feature is missing, the
	// Repository remembers that the registry does not support single-request
	// push, and later pushes go straight to a POST followed by a PUT.
	//
	// A blob larger than the effective MaxChunkSize is pushed in chunks. If the
	// chunked upload cannot be started, the push falls back as if chunking were
	// off, so a blob no larger than the threshold is then pushed in a single
	// request.
	//
	// SingleRequestPushThreshold applies to [Repository.Push] only. Blob mount
	// falls back to a POST followed by a PUT when the registry does not
	// implement the mount endpoint, regardless of this setting.
	//
	// Reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#single-post
	SingleRequestPushThreshold int64

	// MaxUnsafePushBytes overrides Registry default if > 0.
	// It limits the number of bytes accepted for both known and unknown sizes
	// and applies to the blob store's UnsafePush only. If zero or negative,
	// Registry.MaxUnsafePushBytes is used. If both settings are non-positive,
	// the default (currently 4 GiB) is used.
	MaxUnsafePushBytes int64

	// mirrors is an ordered list of mirror repositories to try before the
	// primary registry for read operations. When empty, all reads go directly
	// to the primary. Mirrors are populated from a [properties.Registry] when a
	// Repository is constructed via the ClientBuilder; a Repository created
	// directly has no mirrors.
	mirrors []mirrorRepository
	// sourceReference is the registry and repository before a registries.conf
	// location rewrite. ParseReference re-roots references that match it.
	sourceReference properties.Reference

	// The fields below are not copied by clone(); it resets them.
	// TestRepository_clone_copiesEveryField covers both groups.

	// referrersState indicates whether the repository supports the Referrers
	// API. It stores a properties.ReferrersAPI value.
	// default: properties.ReferrersAPIUnknown
	// Must be accessed atomically.
	referrersState int32

	// referrersPingLock locks the pingReferrers() method and allows only
	// one go-routine to send the request.
	referrersPingLock sync.Mutex

	// referrersMergePool provides a way to manage concurrent updates to a
	// referrers index tagged by referrers tag schema.
	referrersMergePool syncutil.Pool[syncutil.Merge[referrerChange]]

	// singleRequestUnsupported records that the registry ignored or rejected a
	// single-request blob push, so later pushes skip it. See
	// SingleRequestPushThreshold.
	singleRequestUnsupported atomic.Bool
}

// NewRepository creates a client to the remote repository identified by a
// reference. A default Registry is created internally.
// Example: localhost:5000/hello-world
//
// The reference must not contain a tag or a digest, since a Repository
// identifies a repository rather than a specific artifact. A reference
// carrying one is rejected with an error wrapping
// [errdef.ErrInvalidReference].
func NewRepository(reference string) (*Repository, error) {
	ref, err := properties.NewReference(reference)
	if err != nil {
		return nil, err
	}
	if ref.Tag != "" || ref.Digest != "" {
		return nil, fmt.Errorf("%w: %q: expected a repository without a tag or digest", errdef.ErrInvalidReference, reference)
	}

	// Create a default Registry
	reg := &Registry{
		Reference: properties.Reference{
			Registry: ref.Registry,
		},
	}

	return &Repository{
		Registry:       reg,
		RepositoryName: ref.Repository,
	}, nil
}

// reference returns the properties reference for this repository.
func (r *Repository) reference() properties.Reference {
	return properties.Reference{
		Registry:   r.Registry.Reference.Registry,
		Repository: r.RepositoryName,
	}
}

// Reference returns the full properties.Reference for this repository.
func (r *Repository) Reference() properties.Reference {
	return r.reference()
}

// clone makes a copy of the Repository being careful not to copy non-copyable fields (sync.Mutex and syncutil.Pool types)
func (r *Repository) clone() *Repository {
	return &Repository{
		Registry:                   r.Registry,
		RepositoryName:             r.RepositoryName,
		ManifestMediaTypes:         slices.Clone(r.ManifestMediaTypes),
		TagListPageSize:            r.TagListPageSize,
		ReferrerListPageSize:       r.ReferrerListPageSize,
		TagListMaxPages:            r.TagListMaxPages,
		ReferrerListMaxPages:       r.ReferrerListMaxPages,
		SkipReferrersGC:            r.SkipReferrersGC,
		MaxChunkSize:               r.MaxChunkSize,
		SingleRequestPushThreshold: r.SingleRequestPushThreshold,
		MaxUnsafePushBytes:         r.MaxUnsafePushBytes,
		mirrors:                    r.mirrors,
		sourceReference:            r.sourceReference,
	}
}

// client returns an HTTP client used to access the remote repository.
// A default HTTP client is returned if the client is not configured.
func (r *Repository) client() Client {
	if r.Registry.Client == nil {
		return auth.DefaultClient
	}
	return r.Registry.Client
}

// plainHTTP returns whether plain HTTP should be used.
func (r *Repository) plainHTTP() bool {
	return r.Registry.PlainHTTP
}

// maxMetadataBytes returns the maximum metadata bytes limit.
func (r *Repository) maxMetadataBytes() int64 {
	return r.Registry.MaxMetadataBytes
}

// maxChunkSize returns the effective target chunk size in bytes for chunked
// blob upload. A value <= 0 disables chunking.
// Repository-level setting takes precedence over Registry default.
func (r *Repository) maxChunkSize() int64 {
	return override(max(r.MaxChunkSize, 0), r.Registry.MaxChunkSize)
}

// maxUnsafePushBytes returns the effective size limit for unsafe blob pushes.
// Repository-level setting takes precedence over Registry default.
func (r *Repository) maxUnsafePushBytes() int64 {
	limit := override(max(r.MaxUnsafePushBytes, 0), max(r.Registry.MaxUnsafePushBytes, 0))
	if limit <= 0 {
		return ioutil.DefaultMaxUnsafePushBytes
	}
	return limit
}

// singleRequestPushThreshold returns the effective size limit in bytes for
// pushing a blob in a single POST request. A value <= 0 disables it.
// Repository-level setting takes precedence over Registry default.
func (r *Repository) singleRequestPushThreshold() int64 {
	return override(max(r.SingleRequestPushThreshold, 0), r.Registry.SingleRequestPushThreshold)
}

// handleWarning returns the warning handler function.
func (r *Repository) handleWarning() func(warning Warning) {
	return r.Registry.HandleWarning
}

// policy returns the policy evaluator.
func (r *Repository) policy() *policy.Evaluator {
	return r.Registry.Policy
}

// tagListPageSize returns the effective tag list page size.
// Repository-level setting takes precedence over Registry default.
func (r *Repository) tagListPageSize() int {
	return override(max(r.TagListPageSize, 0), r.Registry.TagListPageSize)
}

// referrerListPageSize returns the effective referrer list page size.
// Repository-level setting takes precedence over Registry default.
func (r *Repository) referrerListPageSize() int {
	return override(max(r.ReferrerListPageSize, 0), r.Registry.ReferrerListPageSize)
}

// tagListMaxPages returns the effective maximum number of tag list pages.
// Repository-level setting takes precedence over Registry default.
func (r *Repository) tagListMaxPages() int {
	return override(max(r.TagListMaxPages, 0), r.Registry.TagListMaxPages)
}

// referrerListMaxPages returns the effective maximum number of referrer list pages.
// Repository-level setting takes precedence over Registry default.
func (r *Repository) referrerListMaxPages() int {
	return override(max(r.ReferrerListMaxPages, 0), r.Registry.ReferrerListMaxPages)
}

// manifestMediaTypes returns the effective manifest media types.
// Repository-level setting takes precedence over Registry default.
func (r *Repository) manifestMediaTypes() []string {
	return overrideSlice(r.ManifestMediaTypes, r.Registry.ManifestMediaTypes)
}

// skipReferrersGC returns the effective skip referrers GC setting.
// Repository-level setting takes precedence over Registry default.
func (r *Repository) skipReferrersGC() bool {
	return override(r.SkipReferrersGC, r.Registry.SkipReferrersGC)
}

// SetReferrersCapability indicates the Referrers API capability of the remote
// repository. true: capable; false: not capable.
//
// The capability is fixed once set; the first value set wins and conflicting
// later calls are ignored. Callers can use ReferrersCapability to read back
// the effective capability.
//   - When the capability is set to true, the Referrers() function will always
//     request the Referrers API. Reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#listing-referrers
//   - When the capability is set to false, the Referrers() function will always
//     request the Referrers Tag. Reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#referrers-tag-schema
//   - When the capability is not set, the Referrers() function will automatically
//     determine which API to use.
func (r *Repository) SetReferrersCapability(capable bool) {
	state := properties.ReferrersAPIUnsupported
	if capable {
		state = properties.ReferrersAPISupported
	}
	// first value set wins; a conflicting later set is silently ignored.
	atomic.CompareAndSwapInt32(&r.referrersState, int32(properties.ReferrersAPIUnknown), int32(state))
}

// ReferrersCapability reports the repository's current Referrers API
// capability. The value is either what was configured via
// SetReferrersCapability / registry properties, or the result of
// auto-detection performed during a Referrers call. It returns
// properties.ReferrersAPIUnknown when the capability has not been determined.
func (r *Repository) ReferrersCapability() properties.ReferrersAPI {
	return r.loadReferrersState()
}

// loadReferrersState atomically loads r.referrersState.
func (r *Repository) loadReferrersState() properties.ReferrersAPI {
	return properties.ReferrersAPI(atomic.LoadInt32(&r.referrersState))
}

// do sends an HTTP request and returns an HTTP response using the HTTP client
// returned by r.client().
func (r *Repository) do(req *http.Request) (*http.Response, error) {
	handler := r.handleWarning()
	if handler == nil {
		return r.client().Do(req)
	}

	resp, err := r.client().Do(req)
	if err != nil {
		return nil, err
	}
	handleWarningHeaders(resp.Header.Values(headerWarning), handler)
	return resp, nil
}

// blobStore detects the blob store for the given descriptor.
func (r *Repository) blobStore(desc ocispec.Descriptor) registry.BlobStore {
	if isManifest(r.manifestMediaTypes(), desc) {
		return r.Manifests()
	}
	return r.Blobs()
}

// Fetch fetches the content identified by the descriptor.
// If mirrors are configured, they are tried in order before the primary.
// Manifest responses with a different Docker-Content-Digest algorithm are
// verified and buffered up to MaxMetadataBytes before the reader is returned.
// Each such in-flight fetch may hold that much data in memory.
func (r *Repository) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	if err := r.checkDescriptorPolicy(ctx, target); err != nil {
		return nil, err
	}
	fetch := func(repo *Repository) (io.ReadCloser, error) {
		return repo.fetch(ctx, target)
	}
	return tryMirrors(r.mirrors, r, target.Digest.String(), fetch, fetch, acceptAll[io.ReadCloser])
}

// fetch is Fetch without the policy check and the mirrors.
func (r *Repository) fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	if isManifest(r.manifestMediaTypes(), target) {
		return r.manifests().fetch(ctx, target)
	}
	return r.blobs().fetch(ctx, target)
}

// Push pushes the content, matching the expected descriptor.
func (r *Repository) Push(ctx context.Context, expected ocispec.Descriptor, content io.Reader) error {
	return r.blobStore(expected).Push(ctx, expected, content)
}

// UnsafePush pushes content whose digest or size may be unknown. See
// [content.UnsafePusher] for verification and error semantics. Like Push, it
// routes by media type (see [Repository.ManifestMediaTypes]): manifests are
// buffered and verified up to [Registry.MaxMetadataBytes] before the PUT and
// indexed for referrers as with Push; other content is pushed as a blob,
// bounded by [Repository.MaxUnsafePushBytes].
func (r *Repository) UnsafePush(ctx context.Context, expected ocispec.Descriptor, content io.Reader) (ocispec.Descriptor, error) {
	if isManifest(r.manifestMediaTypes(), expected) {
		return r.manifests().UnsafePush(ctx, expected, content)
	}
	return r.blobs().UnsafePush(ctx, expected, content)
}

// Mount makes the blob with the given digest in fromRepo
// available in the repository signified by the receiver.
//
// This avoids the need to pull content down from fromRepo only to push it to r.
//
// If the registry does not implement mounting, getContent will be used to get the
// content to push. If getContent is nil, the content will be pulled from the source
// repository. If getContent returns an error, it will be wrapped inside the error
// returned from Mount.
func (r *Repository) Mount(ctx context.Context, desc ocispec.Descriptor, fromRepo string, getContent func() (io.ReadCloser, error)) error {
	mounter, ok := r.Blobs().(registry.Mounter)
	if !ok {
		return errors.New("blob store does not support mounting")
	}
	return mounter.Mount(ctx, desc, fromRepo, getContent)
}

// Exists returns true if the described content exists.
// If mirrors are configured, they are tried in order before the primary.
func (r *Repository) Exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	if err := r.checkDescriptorWritePolicy(ctx, target); err != nil {
		return false, err
	}
	exists := func(repo *Repository) (bool, error) {
		return repo.exists(ctx, target)
	}
	return tryMirrors(r.mirrors, r, target.Digest.String(), exists, exists, func(ok bool) bool {
		// Only a positive result is authoritative. A mirror that does not
		// have the content says nothing about the primary, so keep
		// looking; treating "absent here" as "absent everywhere" would
		// report content that exists as missing.
		return ok
	})
}

// exists is Exists without the policy check and the mirrors.
func (r *Repository) exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	if isManifest(r.manifestMediaTypes(), target) {
		return r.manifests().exists(ctx, target)
	}
	return r.blobs().exists(ctx, target)
}

// Delete removes the content identified by the descriptor.
func (r *Repository) Delete(ctx context.Context, target ocispec.Descriptor) error {
	return r.blobStore(target).Delete(ctx, target)
}

// Blobs provides access to the blob CAS only, which contains config blobs,
// layers, and other generic blobs.
func (r *Repository) Blobs() registry.BlobStore {
	return r.blobs()
}

// Manifests provides access to the manifest CAS only.
func (r *Repository) Manifests() registry.ManifestStore {
	return r.manifests()
}

// blobs is Blobs with the concrete type, whose unexported methods skip the
// policy check.
func (r *Repository) blobs() *blobStore {
	return &blobStore{repo: r}
}

// manifests is Manifests with the concrete type, whose unexported methods
// skip the policy check.
func (r *Repository) manifests() *manifestStore {
	return &manifestStore{repo: r}
}

// Resolve resolves a reference to a manifest descriptor.
// If mirrors are configured, they are tried in order before the primary.
// See also `ManifestMediaTypes`.
func (r *Repository) Resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	if err := r.checkPolicy(ctx, reference); err != nil {
		return ocispec.Descriptor{}, err
	}
	desc, err := tryMirrorsRef(r.mirrors, r, reference,
		func(repo *Repository, ref string) (ocispec.Descriptor, error) {
			return repo.manifests().resolve(ctx, ref)
		})
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	if err := r.checkPolicyResolved(ctx, reference, desc); err != nil {
		return ocispec.Descriptor{}, err
	}
	return desc, nil
}

// Tag tags a manifest descriptor with a reference string.
// A registry may return a Docker-Content-Digest header in a different
// allowlisted algorithm; the pushed bytes are verified against it. Tag pushes
// buffer the manifest up to MaxMetadataBytes (default 4MiB) for that check, so
// an oversized manifest or index fails with [errdef.ErrSizeExceedsLimit].
func (r *Repository) Tag(ctx context.Context, desc ocispec.Descriptor, reference string) error {
	return r.Manifests().Tag(ctx, desc, reference)
}

// Untag removes the association between the given tag and the manifest it
// currently points to. Only tags are accepted; to delete a manifest by
// digest, use Delete.
//
// Tag deletion is an optional, loosely specified capability. Some registries
// do not support it (returning 400 or 405, surfaced here as an error), and
// some delete the underlying manifest — and thus every tag pointing to it —
// rather than only the given tag. This differs from the local content/oci
// store's Untag, which never deletes the underlying content.
//
// Reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#deleting-tags
func (r *Repository) Untag(ctx context.Context, reference string) error {
	return r.Manifests().Untag(ctx, reference)
}

// PushReference pushes the manifest with a reference tag.
//
// When reference is a tag, the registry chooses the stored digest algorithm. A
// Docker-Content-Digest header in a different allowlisted algorithm is accepted
// only if it equals that algorithm's digest of the bytes pushed. Tag pushes
// buffer the manifest up to MaxMetadataBytes (default 4MiB) for that check, so
// an oversized manifest or index fails with [errdef.ErrSizeExceedsLimit].
// expected.Digest is not rewritten; on a registry that indexes only SHA-256,
// Resolve by the caller's digest may fail afterwards, and referrers naming that
// digest may not be found. A digest reference is strict: the URL names the
// identity.
func (r *Repository) PushReference(ctx context.Context, expected ocispec.Descriptor, content io.Reader, reference string) error {
	return r.Manifests().PushReference(ctx, expected, content, reference)
}

// FetchReference fetches the manifest identified by the reference.
// The reference can be a tag or digest.
// If mirrors are configured, they are tried in order before the primary.
func (r *Repository) FetchReference(ctx context.Context, reference string) (ocispec.Descriptor, io.ReadCloser, error) {
	if err := r.checkPolicy(ctx, reference); err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	type fetchResult struct {
		desc ocispec.Descriptor
		rc   io.ReadCloser
	}
	result, err := tryMirrorsRef(r.mirrors, r, reference,
		func(repo *Repository, ref string) (fetchResult, error) {
			desc, rc, err := repo.manifests().fetchReference(ctx, ref)
			return fetchResult{desc: desc, rc: rc}, err
		})
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	if err := r.checkPolicyResolved(ctx, reference, result.desc); err != nil {
		result.rc.Close()
		return ocispec.Descriptor{}, nil, err
	}
	return result.desc, result.rc, nil
}

// ParseReference resolves a tag or a digest reference to a fully qualified
// reference from a base reference r.Reference().
// Tag, digest, or fully qualified references are accepted as input.
//
// If reference is a fully qualified reference, then ParseReference parses it
// and returns the parsed reference. If the parsed reference matches the
// repository's base as it was before a registries.conf location rewrite,
// ParseReference re-roots it at the rewritten location and keeps the tag or
// digest. Otherwise, if the parsed reference does not share the same base
// reference with the Repository r, ParseReference returns a wrapped error
// ErrInvalidReference.
func (r *Repository) ParseReference(reference string) (properties.Reference, error) {
	repoRef := r.reference()
	ref, err := properties.NewReference(reference)
	if err != nil {
		ref = properties.Reference{
			Registry:   repoRef.Registry,
			Repository: repoRef.Repository,
		}

		// reference is not a FQDN
		if index := strings.IndexByte(reference, '@'); index != -1 {
			if strings.Contains(reference[:index], "/") {
				// A repository path before `@` is not a bare tag.
				return properties.Reference{}, err
			}
			// `@` implies *digest*, so drop the *tag* (irrespective of what it is).
			ref.Digest = reference[index+1:]
			err = ref.ValidateDigest()
		} else if d, digestErr := digest.Parse(reference); digestErr == nil {
			ref.Digest = d.String()
			err = nil
		} else {
			ref.Tag = reference
			err = ref.ValidateTag()
		}

		if err != nil {
			return properties.Reference{}, err
		}
	} else if ref.Registry != repoRef.Registry || ref.Repository != repoRef.Repository {
		if r.sourceReference.Registry != "" && r.sourceReference.Repository != "" &&
			ref.Registry == r.sourceReference.Registry && ref.Repository == r.sourceReference.Repository {
			ref.Registry = repoRef.Registry
			ref.Repository = repoRef.Repository
		} else {
			return properties.Reference{}, fmt.Errorf(
				"%w: mismatch between received %q and expected %q",
				errdef.ErrInvalidReference, ref, repoRef,
			)
		}
	}

	if ref.GetReference() == "" {
		return properties.Reference{}, errdef.ErrInvalidReference
	}

	return ref, nil
}

// Tags lists the tags available in the repository.
// See also `TagListPageSize`.
// If `last` is NOT empty, the entries in the response start after the
// tag specified by `last`. Otherwise, the response starts from the top
// of the Tags list.
//
// References:
//   - https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#content-discovery
//   - https://distribution.github.io/distribution/spec/api/#tags
func (r *Repository) Tags(ctx context.Context, last string, fn func(tags []string) error) error {
	if err := r.checkPolicy(ctx, ""); err != nil {
		return err
	}
	repoRef := r.reference()
	ctx = auth.AppendRepositoryScope(ctx, repoRef, auth.ActionPull)
	url := buildRepositoryTagListURL(r.plainHTTP(), repoRef)
	var err error
	maxPages := r.tagListMaxPages()
	for page := 0; err == nil; page++ {
		if maxPages > 0 && page >= maxPages {
			return fmt.Errorf("tag listing exceeded %d pages: %w", maxPages, errdef.ErrTooManyPages)
		}
		url, err = r.tags(ctx, last, fn, url)
		// clear `last` for subsequent pages
		last = ""
	}
	if err != errNoLink {
		return err
	}
	return nil
}

// tags returns a single page of tag list with the next link.
func (r *Repository) tags(ctx context.Context, last string, fn func(tags []string) error, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	pageSize := r.tagListPageSize()
	if pageSize > 0 || last != "" {
		q := req.URL.Query()
		if pageSize > 0 {
			q.Set("n", strconv.Itoa(pageSize))
		}
		if last != "" {
			q.Set("last", last)
		}
		req.URL.RawQuery = q.Encode()
	}
	resp, err := r.do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errutil.ParseErrorResponse(resp)
	}
	var page struct {
		Tags []string `json:"tags"`
	}
	lr := limitReader(resp.Body, r.maxMetadataBytes())
	if err := json.NewDecoder(lr).Decode(&page); err != nil {
		return "", fmt.Errorf("%s %q: failed to decode response: %w", resp.Request.Method, resp.Request.URL, err)
	}
	if err := fn(page.Tags); err != nil {
		return "", err
	}

	return parseLink(resp)
}

// Predecessors returns the descriptors of image or artifact manifests directly
// referencing the given manifest descriptor.
// Predecessors internally leverages Referrers.
// Reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#listing-referrers
func (r *Repository) Predecessors(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	var res []ocispec.Descriptor
	if err := r.Referrers(ctx, desc, "", func(referrers []ocispec.Descriptor) error {
		res = append(res, referrers...)
		return nil
	}); err != nil {
		return nil, err
	}
	return res, nil
}

// Referrers lists the descriptors of image or artifact manifests directly
// referencing the given manifest descriptor.
//
// fn is called for each page of the referrers result.
// If artifactType is not empty, only referrers of the same artifact type are
// fed to fn.
//
// Reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#listing-referrers
func (r *Repository) Referrers(ctx context.Context, desc ocispec.Descriptor, artifactType string, fn func(referrers []ocispec.Descriptor) error) error {
	if err := r.checkPolicy(ctx, ""); err != nil {
		return err
	}
	state := r.loadReferrersState()
	if state == properties.ReferrersAPIUnsupported {
		// The repository is known to not support Referrers API, fallback to
		// referrers tag schema.
		return r.referrersByTagSchema(ctx, desc, artifactType, fn)
	}

	err := r.referrersByAPI(ctx, desc, artifactType, fn)
	if state == properties.ReferrersAPISupported {
		// The repository is known to support Referrers API, no fallback.
		return err
	}

	// The referrers state is unknown.
	if err != nil {
		if errors.Is(err, errdef.ErrUnsupported) {
			// Referrers API is not supported, fallback to referrers tag schema.
			r.SetReferrersCapability(false)
			return r.referrersByTagSchema(ctx, desc, artifactType, fn)
		}
		if errutil.IsErrorCode(err, errcode.ErrorCodeNameUnknown) {
			// The registry answered 404 NAME_UNKNOWN. The distribution spec
			// requires a fallback to the referrers tag schema on any 404 from
			// the Referrers API, but NAME_UNKNOWN may only mean that the
			// repository does not exist yet, so the Referrers API capability
			// stays unknown and is probed again next time.
			// Reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#unavailable-referrers-api
			return r.referrersByTagSchema(ctx, desc, artifactType, fn)
		}
		return err
	}

	r.SetReferrersCapability(true)
	return nil
}

// referrersByAPI lists the descriptors of manifests directly referencing
// the given manifest descriptor by requesting Referrers API.
// fn is called for the referrers result. If artifactType is not empty,
// only referrers of the same artifact type are fed to fn.
func (r *Repository) referrersByAPI(ctx context.Context, desc ocispec.Descriptor, artifactType string, fn func(referrers []ocispec.Descriptor) error) error {
	repoRef := r.reference()
	ref := repoRef
	ref.Digest = desc.Digest.String()
	ctx = auth.AppendRepositoryScope(ctx, ref, auth.ActionPull)

	url := buildReferrersURL(r.plainHTTP(), ref, artifactType)
	var err error
	maxPages := r.referrerListMaxPages()
	for page := 0; err == nil; page++ {
		if maxPages > 0 && page >= maxPages {
			return fmt.Errorf("referrer listing exceeded %d pages: %w", maxPages, errdef.ErrTooManyPages)
		}
		url, err = r.referrersPageByAPI(ctx, artifactType, fn, url)
	}
	if err == errNoLink {
		return nil
	}
	return err
}

// referrersPageByAPI lists a single page of the descriptors of manifests
// directly referencing the given manifest descriptor. fn is called for
// a page of referrersPageByAPI result.
// If artifactType is not empty, only referrersPageByAPI of the same
// artifact type are fed to fn.
// referrersPageByAPI returns the link url for the next page.
func (r *Repository) referrersPageByAPI(ctx context.Context, artifactType string, fn func(referrers []ocispec.Descriptor) error, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	pageSize := r.referrerListPageSize()
	if pageSize > 0 {
		q := req.URL.Query()
		q.Set("n", strconv.Itoa(pageSize))
		req.URL.RawQuery = q.Encode()
	}

	resp, err := r.do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		if errResp := errutil.ParseErrorResponse(resp); errutil.IsErrorCode(errResp, errcode.ErrorCodeNameUnknown) {
			// The repository is not found, Referrers API status is unknown
			return "", errResp
		}
		// Referrers API is not supported.
		return "", fmt.Errorf("failed to query referrers API: %w", errdef.ErrUnsupported)
	default:
		return "", errutil.ParseErrorResponse(resp)
	}

	// also check the content type
	if ct := resp.Header.Get("Content-Type"); !isImageIndexContentType(ct) {
		return "", fmt.Errorf("unknown content returned (%s), expecting image index: %w", ct, errdef.ErrUnsupported)
	}

	var index ocispec.Index
	lr := limitReader(resp.Body, r.maxMetadataBytes())
	if err := json.NewDecoder(lr).Decode(&index); err != nil {
		return "", fmt.Errorf("%s %q: failed to decode response: %w", resp.Request.Method, resp.Request.URL, err)
	}

	referrers := index.Manifests
	if artifactType != "" {
		// check both filters header and filters annotations for compatibility
		// latest spec for filters header: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#listing-referrers
		// older spec for filters annotations: https://github.com/opencontainers/distribution-spec/blob/v1.1.0-rc1/spec.md#listing-referrers
		filtersHeader := resp.Header.Get(headerOCIFiltersApplied)
		filtersAnnotation := index.Annotations[spec.AnnotationReferrersFiltersApplied]
		if !isReferrersFilterApplied(filtersHeader, filterTypeArtifactType) &&
			!isReferrersFilterApplied(filtersAnnotation, filterTypeArtifactType) {
			// perform client side filtering if the filter is not applied on the server side
			referrers = filterReferrers(referrers, artifactType)
		}
	}
	if len(referrers) > 0 {
		if err := fn(referrers); err != nil {
			return "", err
		}
	}
	return parseLink(resp)
}

// referrersByTagSchema lists the descriptors of manifests directly
// referencing the given manifest descriptor by requesting referrers tag.
// fn is called for the referrers result. If artifactType is not empty,
// only referrers of the same artifact type are fed to fn.
// reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#backwards-compatibility
func (r *Repository) referrersByTagSchema(ctx context.Context, desc ocispec.Descriptor, artifactType string, fn func(referrers []ocispec.Descriptor) error) error {
	referrersTag, err := buildReferrersTag(desc)
	if err != nil {
		return err
	}
	_, referrers, err := r.referrersFromIndex(ctx, referrersTag, desc)
	if err != nil {
		if errors.Is(err, errdef.ErrNotFound) {
			// no referrers to the manifest
			return nil
		}
		return err
	}

	filtered := filterReferrers(referrers, artifactType)
	if len(filtered) == 0 {
		return nil
	}
	return fn(filtered)
}

// referrersFromIndex queries the referrers index using the the given referrers
// tag. If Succeeded, returns the descriptor of referrers index and the
// referrers list.
func (r *Repository) referrersFromIndex(ctx context.Context, referrersTag string, subject ocispec.Descriptor) (ocispec.Descriptor, []ocispec.Descriptor, error) {
	// The referrers index is maintained by clients and never signed, so it is
	// not subject to the policy.
	desc, rc, err := r.manifests().fetchReference(ctx, referrersTag)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	defer rc.Close()

	// Some registries resolve the fallback tag to the subject itself. Its
	// children are not referrers, and the tag schema requires an OCI index.
	if desc.MediaType != ocispec.MediaTypeImageIndex || desc.Digest == subject.Digest {
		return ocispec.Descriptor{}, nil, fmt.Errorf("invalid referrers index from referrers tag %s: %w", referrersTag, errdef.ErrNotFound)
	}

	if err := limitSize(desc, r.maxMetadataBytes()); err != nil {
		return ocispec.Descriptor{}, nil, fmt.Errorf("failed to read referrers index from referrers tag %s: %w", referrersTag, err)
	}
	var index ocispec.Index
	if err := decodeJSON(rc, desc, &index); err != nil {
		return ocispec.Descriptor{}, nil, fmt.Errorf("failed to decode referrers index from referrers tag %s: %w", referrersTag, err)
	}

	return desc, index.Manifests, nil
}

// pingReferrers returns true if the Referrers API is available for r.
func (r *Repository) pingReferrers(ctx context.Context) (bool, error) {
	switch r.loadReferrersState() {
	case properties.ReferrersAPISupported:
		return true, nil
	case properties.ReferrersAPIUnsupported:
		return false, nil
	}

	// referrers state is unknown
	// limit the rate of pinging referrers API
	r.referrersPingLock.Lock()
	defer r.referrersPingLock.Unlock()

	switch r.loadReferrersState() {
	case properties.ReferrersAPISupported:
		return true, nil
	case properties.ReferrersAPIUnsupported:
		return false, nil
	}

	repoRef := r.reference()
	ref := repoRef
	ref.Digest = zeroDigest
	ctx = auth.AppendRepositoryScope(ctx, ref, auth.ActionPull)

	url := buildReferrersURL(r.plainHTTP(), ref, "")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	resp, err := r.do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		supported := isImageIndexContentType(resp.Header.Get("Content-Type"))
		r.SetReferrersCapability(supported)
		return supported, nil
	case http.StatusNotFound:
		if err := errutil.ParseErrorResponse(resp); errutil.IsErrorCode(err, errcode.ErrorCodeNameUnknown) {
			// repository not found
			return false, err
		}
		r.SetReferrersCapability(false)
		return false, nil
	default:
		return false, errutil.ParseErrorResponse(resp)
	}
}

// delete removes the content identified by the descriptor in the entity "blobs"
// or "manifests".
func (r *Repository) delete(ctx context.Context, target ocispec.Descriptor, isManifest bool) error {
	if err := checkDigestAlgorithm(digestSourceDescriptor, target.Digest); err != nil {
		return err
	}
	repoRef := r.reference()
	ref := repoRef
	ref.Digest = target.Digest.String()
	ctx = auth.AppendRepositoryScope(ctx, ref, auth.ActionDelete)
	buildURL := buildRepositoryBlobURL
	if isManifest {
		buildURL = buildRepositoryManifestURL
	}
	url := buildURL(r.plainHTTP(), ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	resp, err := r.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusAccepted:
		return verifyContentDigest(resp, target.Digest)
	case http.StatusNotFound:
		return fmt.Errorf("%s: %w", target.Digest, errdef.ErrNotFound)
	default:
		return errutil.ParseErrorResponse(resp)
	}
}

// blobStore accesses the blob part of the repository.
type blobStore struct {
	repo *Repository
}

// Fetch fetches the content identified by the descriptor.
func (s *blobStore) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	if err := s.repo.checkPolicy(ctx, ""); err != nil {
		return nil, err
	}
	return s.fetch(ctx, target)
}

// fetch is Fetch without the policy check, for callers that have already
// checked the operation it is part of.
func (s *blobStore) fetch(ctx context.Context, target ocispec.Descriptor) (rc io.ReadCloser, err error) {
	if err := checkDigestAlgorithm(digestSourceDescriptor, target.Digest); err != nil {
		return nil, err
	}
	repoRef := s.repo.reference()
	ref := repoRef
	ref.Digest = target.Digest.String()
	ctx = auth.AppendRepositoryScope(ctx, ref, auth.ActionPull)
	url := buildRepositoryBlobURL(s.repo.plainHTTP(), ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.repo.do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			resp.Body.Close()
		}
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		if size := resp.ContentLength; size != -1 && size != target.Size {
			return nil, fmt.Errorf("%s %q: mismatch Content-Length", resp.Request.Method, resp.Request.URL)
		}
		if err := verifyPullDigestHeader(resp, target.Digest); err != nil {
			return nil, err
		}

		// check server range request capability.
		// Docker spec allows range header form of "Range: bytes=<start>-<end>".
		// However, the remote server may still not RFC 7233 compliant.
		// Reference: https://distribution.github.io/distribution/spec/api/#blob
		if rangeUnit := resp.Header.Get("Accept-Ranges"); rangeUnit == "bytes" {
			return httputil.NewReadSeekCloser(s.repo.client(), req, resp.Body, target.Size), nil
		}
		return resp.Body, nil
	case http.StatusNotFound:
		return nil, fmt.Errorf("%s: %w", target.Digest, errdef.ErrNotFound)
	default:
		return nil, errutil.ParseErrorResponse(resp)
	}
}

// Mount mounts the given descriptor from fromRepo into s.
func (s *blobStore) Mount(ctx context.Context, desc ocispec.Descriptor, fromRepo string, getContent func() (io.ReadCloser, error)) error {
	if err := s.repo.checkPolicy(ctx, ""); err != nil {
		return err
	}
	if err := checkDigestAlgorithm(digestSourceDescriptor, desc.Digest); err != nil {
		return err
	}
	// pushing usually requires both pull and push actions.
	// Reference: https://github.com/distribution/distribution/blob/v2.7.1/registry/handlers/app.go#L921-L930
	repoRef := s.repo.reference()
	ctx = auth.AppendRepositoryScope(ctx, repoRef, auth.ActionPull, auth.ActionPush)

	// We also need pull access to the source repo. Scope hints are keyed by the
	// resource they apply to, so the source scope has to be registered against
	// the target repository: it is the request to the target that must carry it.
	ctx = auth.AppendScopesForResource(ctx, repoRef.Resource(), auth.ScopeRepository(fromRepo, auth.ActionPull))

	url := buildRepositoryBlobMountURL(s.repo.plainHTTP(), repoRef, desc.Digest, fromRepo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	resp, err := s.repo.do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusCreated {
		defer resp.Body.Close()
		// Check the server seems to be behaving.
		return verifyPushContentDigest(resp, desc.Digest, nil)
	}
	if resp.StatusCode != http.StatusAccepted {
		defer resp.Body.Close()
		return errutil.ParseErrorResponse(resp)
	}
	resp.Body.Close()
	// From the [spec]:
	//
	// "If a registry does not support cross-repository mounting
	// or is unable to mount the requested blob,
	// it SHOULD return a 202.
	// This indicates that the upload session has begun
	// and that the client MAY proceed with the upload."
	//
	// So we need to get the content from somewhere in order to
	// push it. If the caller has provided a getContent function, we
	// can use that, otherwise pull the content from the source repository.
	//
	// [spec]: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#mounting-a-blob-from-another-repository

	var r io.ReadCloser
	if getContent != nil {
		r, err = getContent()
	} else {
		r, err = s.sibling(fromRepo).fetch(ctx, desc)
	}
	if err != nil {
		return fmt.Errorf("cannot read source blob: %w", err)
	}
	defer r.Close()
	return s.completePushAfterInitialPost(ctx, req, resp, desc, r)
}

// sibling returns a blob store for another repository in the same
// registry.
func (s *blobStore) sibling(otherRepoName string) *blobStore {
	otherRepo := s.repo.clone()
	otherRepo.RepositoryName = otherRepoName
	return &blobStore{
		repo: otherRepo,
	}
}

// Push pushes the content, matching the expected descriptor.
// Existing content is not checked by Push() to minimize the number of out-going
// requests.
// By default, Push is done by conventional 2-step monolithic upload: a `POST`
// opens an upload session and a `PUT` sends the content. The content is not
// sent until the `POST` succeeds, so an authentication challenge is answered
// and an authentication error fails before any content is read. When
// [Repository.SingleRequestPushThreshold] is set, blobs no larger than it are
// read into memory and pushed in a single `POST` request instead.
//
// References:
//   - https://distribution.github.io/distribution/spec/api/#pushing-an-image
//   - https://distribution.github.io/distribution/spec/api/#initiate-blob-upload
//   - https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#pushing-a-blob-monolithically
func (s *blobStore) Push(ctx context.Context, expected ocispec.Descriptor, content io.Reader) error {
	if err := s.repo.checkPolicy(ctx, ""); err != nil {
		return err
	}
	return s.push(ctx, expected, content)
}

// push is Push without the policy check, for callers that have already
// checked the operation it is part of.
func (s *blobStore) push(ctx context.Context, expected ocispec.Descriptor, content io.Reader) error {
	if err := checkDigestAlgorithm(digestSourceDescriptor, expected.Digest); err != nil {
		return err
	}
	// start an upload
	// pushing usually requires both pull and push actions.
	// Reference: https://github.com/distribution/distribution/blob/v2.7.1/registry/handlers/app.go#L921-L930
	repoRef := s.repo.reference()
	ctx = auth.AppendRepositoryScope(ctx, repoRef, auth.ActionPull, auth.ActionPush)

	// Push in chunks when configured and the blob is larger than a single chunk.
	// A blob that fits in one chunk is pushed monolithically, since a lone PATCH
	// plus the closing PUT is strictly worse than a monolithic POST/PUT. If the
	// chunked upload cannot be started before any content is consumed, fall back
	// to the monolithic path so no registry regresses.
	if chunkSize := s.repo.maxChunkSize(); chunkSize > 0 && expected.Size > chunkSize {
		err := s.pushChunked(ctx, expected, content, chunkSize)
		if err == nil {
			return nil
		}
		if !errors.Is(err, errChunkedUploadNotStarted) {
			return err
		}
		// Pre-consumption failure: no bytes were read from content, so a
		// monolithic upload can safely take over.
	}

	if threshold := s.repo.singleRequestPushThreshold(); expected.Size > 0 && expected.Size <= threshold && !s.repo.singleRequestUnsupported.Load() {
		buf, err := io.ReadAll(io.LimitReader(content, expected.Size+1))
		if err != nil {
			return err
		}
		if int64(len(buf)) != expected.Size {
			return fmt.Errorf("mismatch content length %d: expect %d", len(buf), expected.Size)
		}
		if done, err := s.pushSingleRequest(ctx, expected, buf); done || err != nil {
			return err
		}
		// The registry does not support the single POST, so push the buffered
		// content with POST and PUT.
		content = bytes.NewReader(buf)
	}

	url := buildRepositoryBlobUploadURL(s.repo.plainHTTP(), repoRef)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}

	resp, err := s.repo.do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusAccepted {
		defer resp.Body.Close()
		return errutil.ParseErrorResponse(resp)
	}
	resp.Body.Close()
	return s.completePushAfterInitialPost(ctx, req, resp, expected, content)
}

// pushSingleRequest pushes a blob held in buf in a single POST request
// carrying the content. It reports whether the push is done; false with a nil
// error means the registry rejected the request with a 4xx status other than
// 401, 403 or 429, or with 501, so the caller should fall back to a POST
// followed by a PUT. A 202, or a 405, 415 or 501, marks the repository as not
// supporting it.
// Reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#single-post
func (s *blobStore) pushSingleRequest(ctx context.Context, expected ocispec.Descriptor, buf []byte) (bool, error) {
	url := buildRepositoryBlobUploadURL(s.repo.plainHTTP(), s.repo.reference())
	// a *bytes.Reader body makes net/http set ContentLength and GetBody, so the
	// request can be replayed after a 401 challenge or by a retrying transport.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	q := req.URL.Query()
	q.Set("digest", expected.Digest.String())
	req.URL.RawQuery = q.Encode()

	resp, err := s.repo.do(req)
	if err != nil {
		return false, err
	}
	switch code := resp.StatusCode; {
	case code == http.StatusCreated:
		defer resp.Body.Close()
		return true, verifyPushContentDigest(resp, expected.Digest, nil)
	case code == http.StatusAccepted:
		// The registry ignored the digest and opened an upload session.
		s.repo.singleRequestUnsupported.Store(true)
		resp.Body.Close()
		return true, s.completePushAfterInitialPost(ctx, req, resp, expected, bytes.NewReader(buf))
	case code == http.StatusUnauthorized, code == http.StatusForbidden, code == http.StatusTooManyRequests:
		// a credential or rate-limit problem, not a missing feature
	case code >= 400 && code < 500, code == http.StatusNotImplemented:
		resp.Body.Close()
		// Only these say the feature is missing; others, such as 400
		// DIGEST_INVALID or 404 NAME_UNKNOWN, are about this request.
		switch code {
		case http.StatusMethodNotAllowed, http.StatusUnsupportedMediaType, http.StatusNotImplemented:
			s.repo.singleRequestUnsupported.Store(true)
		}
		return false, nil
	}
	defer resp.Body.Close()
	return true, errutil.ParseErrorResponse(resp)
}

// sameUploadHost reports whether location and reqURL refer to the same host,
// normalizing implicit default ports (80 for http, 443 for https) so that
// e.g. "example.com" and "example.com:443" compare equal over HTTPS.
// Reference: https://github.com/oras-project/oras-go/security/advisories/GHSA-jxpm-75mh-9fp7
func sameUploadHost(location, reqURL *url.URL) bool {
	return urlutil.CanonicalHostPort(location) == urlutil.CanonicalHostPort(reqURL)
}

// resolveUploadLocation extracts the next blob-upload Location from resp,
// resolved against the request URL. It applies the port-443 work-around and
// rejects https scheme downgrades. Credentials stay on the registry host via
// the sameUploadHost checks at the call sites.
// Reference: https://github.com/oras-project/oras-go/issues/177
func resolveUploadLocation(resp *http.Response, req *http.Request) (*url.URL, error) {
	location, err := resp.Location()
	if err != nil {
		return nil, err
	}
	// work-around solution for https://github.com/oras-project/oras-go/issues/177
	// For some registries, if the port 443 is explicitly set to the hostname
	// like registry.wabbit-networks.io:443/myrepo, blob push will fail since
	// the hostname of the Location header in the response is set to
	// registry.wabbit-networks.io instead of registry.wabbit-networks.io:443.
	if req.URL.Port() == "443" && urlutil.SameHostname(location, req.URL) && location.Port() == "" {
		location.Host = net.JoinHostPort(location.Hostname(), "443")
	}
	if req.URL.Scheme == "https" && location.Scheme != "https" {
		return nil, fmt.Errorf("blob upload Location %q downgrades scheme from https", location.Host)
	}
	return location, nil
}

// completePushAfterInitialPost implements step 2 of the push protocol. This can be invoked either by
// Push or by Mount when the receiving repository does not implement the
// mount endpoint.
func (s *blobStore) completePushAfterInitialPost(ctx context.Context, req *http.Request, resp *http.Response, expected ocispec.Descriptor, content io.Reader) error {
	// monolithic upload
	location, err := resolveUploadLocation(resp, req)
	if err != nil {
		return err
	}
	url := location.String()
	// unwrap once; detect seekable/built-in on the unwrapped value.
	unwrapped := ioutil.UnwrapNopCloser(content)
	body := unwrapped
	if _, ok := body.(io.ReadCloser); ok {
		// undo unwrap if the nopCloser is intended.
		body = content
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodPut, url, body)
	if err != nil {
		return err
	}
	if req.GetBody != nil && req.ContentLength != expected.Size {
		// short circuit a size mismatch for built-in types.
		return fmt.Errorf("mismatch content length %d: expect %d", req.ContentLength, expected.Size)
	}
	// If the stdlib did not set GetBody (i.e. body is not a built-in type) but
	// the content is seekable, wire GetBody to re-seek the source so the request
	// body can be replayed without buffering. net/http relies on GetBody to
	// resend the body on a redirect or an HTTP/2 connection recycle, and a
	// retrying RoundTripper relies on it to resend after a failed attempt.
	if req.GetBody == nil {
		if seeker, ok := unwrapped.(io.Seeker); ok {
			if start, err := seeker.Seek(0, io.SeekCurrent); err == nil {
				if ra, ok := unwrapped.(io.ReaderAt); ok {
					// Give each attempt its own reader over the shared source.
					// io.ReaderAt.ReadAt is safe for concurrent use and carries no
					// shared offset, so a failed attempt still reading cannot race
					// the replay's read. Covers *os.File and *bytes.Reader, i.e. the
					// oci/file and memory stores.
					req.Body = io.NopCloser(io.NewSectionReader(ra, start, expected.Size))
					req.GetBody = func() (io.ReadCloser, error) {
						return io.NopCloser(io.NewSectionReader(ra, start, expected.Size)), nil
					}
				} else {
					// Non-ReaderAt seekable source: re-seek one shared reader. Shield
					// it from the transport's per-attempt Body.Close so a replay can
					// still seek it; the caller still owns closing content.
					req.Body = io.NopCloser(unwrapped)
					req.GetBody = func() (io.ReadCloser, error) {
						if _, err := seeker.Seek(start, io.SeekStart); err != nil {
							return nil, err
						}
						return io.NopCloser(unwrapped), nil
					}
				}
			}
		}
	}
	req.ContentLength = expected.Size
	// the expected media type is ignored as in the API doc.
	req.Header.Set("Content-Type", "application/octet-stream")
	q := req.URL.Query()
	q.Set("digest", expected.Digest.String())
	req.URL.RawQuery = q.Encode()

	// reuse credential from previous POST request
	if auth := resp.Request.Header.Get("Authorization"); auth != "" && sameUploadHost(location, resp.Request.URL) {
		req.Header.Set("Authorization", auth)
	}
	resp, err = s.repo.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return errutil.ParseErrorResponse(resp)
	}
	return verifyPushContentDigest(resp, expected.Digest, nil)
}

// Exists returns true if the described content exists.
func (s *blobStore) Exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	if err := s.repo.checkPolicy(ctx, ""); err != nil {
		return false, err
	}
	return s.exists(ctx, target)
}

// exists is Exists without the policy check, for callers that have already
// checked the operation it is part of.
func (s *blobStore) exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	_, err := s.resolve(ctx, target.Digest.String())
	if err == nil {
		return true, nil
	}
	if errors.Is(err, errdef.ErrNotFound) {
		return false, nil
	}
	return false, err
}

// Delete removes the content identified by the descriptor.
func (s *blobStore) Delete(ctx context.Context, target ocispec.Descriptor) error {
	if err := s.repo.checkPolicy(ctx, ""); err != nil {
		return err
	}
	return s.repo.delete(ctx, target, false)
}

// Resolve resolves a reference to a descriptor.
func (s *blobStore) Resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	if err := s.repo.checkPolicy(ctx, ""); err != nil {
		return ocispec.Descriptor{}, err
	}
	return s.resolve(ctx, reference)
}

// resolve is Resolve without the policy check, for callers that have already
// checked the operation it is part of.
func (s *blobStore) resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	ref, err := s.repo.ParseReference(reference)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	refDigest, err := parseSupportedDigest(digestSourceReference, ref.GetReference())
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	ctx = auth.AppendRepositoryScope(ctx, ref, auth.ActionPull)
	url := buildRepositoryBlobURL(s.repo.plainHTTP(), ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	resp, err := s.repo.do(req)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return generateBlobDescriptor(resp, refDigest)
	case http.StatusNotFound:
		return ocispec.Descriptor{}, fmt.Errorf("%s: %w", ref, errdef.ErrNotFound)
	default:
		return ocispec.Descriptor{}, errutil.ParseErrorResponse(resp)
	}
}

// FetchReference fetches the blob identified by the reference.
// The reference must be a digest.
func (s *blobStore) FetchReference(ctx context.Context, reference string) (desc ocispec.Descriptor, rc io.ReadCloser, err error) {
	if err := s.repo.checkPolicy(ctx, ""); err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	ref, err := s.repo.ParseReference(reference)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	refDigest, err := parseSupportedDigest(digestSourceReference, ref.GetReference())
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}

	ctx = auth.AppendRepositoryScope(ctx, ref, auth.ActionPull)
	url := buildRepositoryBlobURL(s.repo.plainHTTP(), ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}

	resp, err := s.repo.do(req)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	defer func() {
		if err != nil {
			resp.Body.Close()
		}
	}()

	switch resp.StatusCode {
	case http.StatusOK: // server does not support seek as `Range` was ignored.
		if resp.ContentLength == -1 {
			desc, err = s.resolve(ctx, reference)
		} else {
			desc, err = generateBlobDescriptor(resp, refDigest)
		}
		if err != nil {
			return ocispec.Descriptor{}, nil, err
		}

		// check server range request capability.
		// Docker spec allows range header form of "Range: bytes=<start>-<end>".
		// However, the remote server may still not RFC 7233 compliant.
		// Reference: https://distribution.github.io/distribution/spec/api/#blob
		if rangeUnit := resp.Header.Get("Accept-Ranges"); rangeUnit == "bytes" {
			return desc, httputil.NewReadSeekCloser(s.repo.client(), req, resp.Body, desc.Size), nil
		}
		return desc, resp.Body, nil
	case http.StatusNotFound:
		return ocispec.Descriptor{}, nil, fmt.Errorf("%s: %w", ref, errdef.ErrNotFound)
	default:
		return ocispec.Descriptor{}, nil, errutil.ParseErrorResponse(resp)
	}
}

// generateBlobDescriptor returns a descriptor generated from the response.
func generateBlobDescriptor(resp *http.Response, refDigest digest.Digest) (ocispec.Descriptor, error) {
	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}

	size := resp.ContentLength
	if size == -1 {
		return ocispec.Descriptor{}, fmt.Errorf("%s %q: unknown response Content-Length", resp.Request.Method, resp.Request.URL)
	}

	if err := verifyPullDigestHeader(resp, refDigest); err != nil {
		return ocispec.Descriptor{}, err
	}

	return ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    refDigest,
		Size:      size,
	}, nil
}

// manifestStore accesses the manifest part of the repository.
type manifestStore struct {
	repo *Repository
}

// Fetch fetches the content identified by the descriptor.
// If the manifest's Docker-Content-Digest uses a different algorithm, the
// response is verified and buffered up to MaxMetadataBytes before return.
// Each such in-flight fetch may hold that much data in memory.
func (s *manifestStore) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	if err := s.repo.checkManifestPolicy(ctx, target.Digest.String(), target); err != nil {
		return nil, err
	}
	return s.fetch(ctx, target)
}

// fetch is Fetch without the policy check, for callers that have already
// checked the operation it is part of.
func (s *manifestStore) fetch(ctx context.Context, target ocispec.Descriptor) (rc io.ReadCloser, err error) {
	if err := checkDigestAlgorithm(digestSourceDescriptor, target.Digest); err != nil {
		return nil, err
	}
	repoRef := s.repo.reference()
	ref := repoRef
	ref.Digest = target.Digest.String()
	ctx = auth.AppendRepositoryScope(ctx, ref, auth.ActionPull)
	url := buildRepositoryManifestURL(s.repo.plainHTTP(), ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", target.MediaType)

	resp, err := s.repo.do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			resp.Body.Close()
		}
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		// no-op
	case http.StatusNotFound:
		return nil, fmt.Errorf("%s: %w", target.Digest, errdef.ErrNotFound)
	default:
		return nil, errutil.ParseErrorResponse(resp)
	}
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("%s %q: invalid response Content-Type: %w", resp.Request.Method, resp.Request.URL, err)
	}
	if mediaType != target.MediaType {
		return nil, fmt.Errorf("%s %q: mismatch response Content-Type %q: expect %q", resp.Request.Method, resp.Request.URL, mediaType, target.MediaType)
	}
	// Content-Length is not validated here because some registries (e.g. Harbor)
	// return different values between HEAD (used by Resolve) and GET responses,
	// typically due to transparent compression. Integrity is guaranteed by
	// digest verification below.
	if err := s.verifyPullContentDigest(resp, target.Digest); err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Push pushes the content, matching the expected descriptor.
func (s *manifestStore) Push(ctx context.Context, expected ocispec.Descriptor, content io.Reader) error {
	if err := s.repo.checkWritePolicy(ctx, expected.Digest.String()); err != nil {
		return err
	}
	return s.pushWithIndexing(ctx, expected, content, expected.Digest.String())
}

// Exists returns true if the described content exists.
func (s *manifestStore) Exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	if err := s.repo.checkWritePolicy(ctx, target.Digest.String()); err != nil {
		return false, err
	}
	return s.exists(ctx, target)
}

// exists is Exists without the policy check, for callers that have already
// checked the operation it is part of.
func (s *manifestStore) exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	_, err := s.resolve(ctx, target.Digest.String())
	if err == nil {
		return true, nil
	}
	if errors.Is(err, errdef.ErrNotFound) {
		return false, nil
	}
	return false, err
}

// Delete removes the manifest content identified by the descriptor.
func (s *manifestStore) Delete(ctx context.Context, target ocispec.Descriptor) error {
	if err := s.repo.checkWritePolicy(ctx, target.Digest.String()); err != nil {
		return err
	}
	return s.deleteWithIndexing(ctx, target)
}

// deleteWithIndexing removes the manifest content identified by the descriptor,
// and indexes referrers for the manifest when needed.
func (s *manifestStore) deleteWithIndexing(ctx context.Context, target ocispec.Descriptor) error {
	switch target.MediaType {
	case spec.MediaTypeArtifactManifest, ocispec.MediaTypeImageManifest, ocispec.MediaTypeImageIndex:
		if state := s.repo.loadReferrersState(); state == properties.ReferrersAPISupported {
			// referrers API is available, no client-side indexing needed
			return s.repo.delete(ctx, target, true)
		}

		if err := limitSize(target, s.repo.maxMetadataBytes()); err != nil {
			return err
		}
		ctx = auth.AppendRepositoryScope(ctx, s.repo.reference(), auth.ActionPull, auth.ActionDelete)
		manifestJSON, err := content.FetchAll(ctx, content.FetcherFunc(s.fetch), target)
		if err != nil {
			return err
		}
		if err := s.indexReferrersForDelete(ctx, target, manifestJSON); err != nil {
			return err
		}
	}

	return s.repo.delete(ctx, target, true)
}

// indexReferrersForDelete indexes referrers for manifests with a subject field
// on manifest delete.
//
// References:
//   - Latest spec: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#deleting-manifests
//   - Compatible spec: https://github.com/opencontainers/distribution-spec/blob/v1.1.0-rc1/spec.md#deleting-manifests
func (s *manifestStore) indexReferrersForDelete(ctx context.Context, desc ocispec.Descriptor, manifestJSON []byte) error {
	var manifest struct {
		Subject *ocispec.Descriptor `json:"subject"`
	}
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		return fmt.Errorf("failed to decode manifest: %s: %s: %w", desc.Digest, desc.MediaType, err)
	}
	if manifest.Subject == nil {
		// no subject, no indexing needed
		return nil
	}

	subject := *manifest.Subject
	ok, err := s.repo.pingReferrers(ctx)
	if err != nil {
		return err
	}
	if ok {
		// referrers API is available, no client-side indexing needed
		return nil
	}
	return s.updateReferrersIndex(ctx, subject, referrerChange{desc, referrerOperationRemove})
}

// Resolve resolves a reference to a descriptor.
// See also `ManifestMediaTypes`.
func (s *manifestStore) Resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	if err := s.repo.checkPolicy(ctx, reference); err != nil {
		return ocispec.Descriptor{}, err
	}
	desc, err := s.resolve(ctx, reference)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	if err := s.repo.checkPolicyResolved(ctx, reference, desc); err != nil {
		return ocispec.Descriptor{}, err
	}
	return desc, nil
}

// resolve is Resolve without the policy check, for callers that have already
// checked the operation it is part of.
func (s *manifestStore) resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	ref, err := s.repo.ParseReference(reference)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	if err := checkReferenceDigestAlgorithm(ref); err != nil {
		return ocispec.Descriptor{}, err
	}
	ctx = auth.AppendRepositoryScope(ctx, ref, auth.ActionPull)
	url := buildRepositoryManifestURL(s.repo.plainHTTP(), ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	req.Header.Set("Accept", manifestAcceptHeader(s.repo.manifestMediaTypes()))

	resp, err := s.repo.do(req)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return s.generateDescriptor(resp, ref, req.Method)
	case http.StatusNotFound:
		return ocispec.Descriptor{}, fmt.Errorf("%s: %w", ref, errdef.ErrNotFound)
	default:
		return ocispec.Descriptor{}, errutil.ParseErrorResponse(resp)
	}
}

// FetchReference fetches the manifest identified by the reference.
// The reference can be a tag or digest.
func (s *manifestStore) FetchReference(ctx context.Context, reference string) (ocispec.Descriptor, io.ReadCloser, error) {
	if err := s.repo.checkPolicy(ctx, reference); err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	desc, rc, err := s.fetchReference(ctx, reference)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	if err := s.repo.checkPolicyResolved(ctx, reference, desc); err != nil {
		rc.Close()
		return ocispec.Descriptor{}, nil, err
	}
	return desc, rc, nil
}

// fetchReference is FetchReference without the policy check, for callers
// that have already checked the operation it is part of.
func (s *manifestStore) fetchReference(ctx context.Context, reference string) (desc ocispec.Descriptor, rc io.ReadCloser, err error) {
	ref, err := s.repo.ParseReference(reference)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	if err := checkReferenceDigestAlgorithm(ref); err != nil {
		return ocispec.Descriptor{}, nil, err
	}

	ctx = auth.AppendRepositoryScope(ctx, ref, auth.ActionPull)
	url := buildRepositoryManifestURL(s.repo.plainHTTP(), ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	req.Header.Set("Accept", manifestAcceptHeader(s.repo.manifestMediaTypes()))

	resp, err := s.repo.do(req)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	defer func() {
		if err != nil {
			resp.Body.Close()
		}
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		if resp.ContentLength == -1 {
			// Identity comes from Resolve's HEAD, which carries no body and so
			// cannot verify an alternate digest algorithm: a digest reference
			// using an algorithm the registry does not report fails here even
			// though Fetch accepts it. Go sets ContentLength to -1 for chunked
			// and transparently gunzipped responses.
			desc, err = s.resolve(ctx, reference)
			if err == nil {
				err = s.verifyPullContentDigest(resp, desc.Digest)
			}
		} else {
			desc, err = s.generateDescriptor(resp, ref, req.Method)
		}
		if err != nil {
			return ocispec.Descriptor{}, nil, err
		}
		return desc, resp.Body, nil
	case http.StatusNotFound:
		return ocispec.Descriptor{}, nil, fmt.Errorf("%s: %w", ref, errdef.ErrNotFound)
	default:
		return ocispec.Descriptor{}, nil, errutil.ParseErrorResponse(resp)
	}
}

// Tag tags a manifest descriptor with a reference string.
// A registry may return a Docker-Content-Digest header in a different
// allowlisted algorithm; the pushed bytes are verified against it. Tag pushes
// buffer the manifest up to MaxMetadataBytes (default 4MiB) for that check, so
// an oversized manifest or index fails with [errdef.ErrSizeExceedsLimit].
func (s *manifestStore) Tag(ctx context.Context, desc ocispec.Descriptor, reference string) error {
	if err := s.repo.checkWritePolicy(ctx, reference); err != nil {
		return err
	}
	ref, err := s.repo.ParseReference(reference)
	if err != nil {
		return err
	}

	ctx = auth.AppendRepositoryScope(ctx, ref, auth.ActionPull, auth.ActionPush)
	rc, err := s.fetch(ctx, desc)
	if err != nil {
		return err
	}
	defer rc.Close()

	return s.push(ctx, desc, rc, ref.GetReference())
}

// Untag removes the association between the given tag and the manifest it currently points to.
func (s *manifestStore) Untag(ctx context.Context, reference string) error {
	if err := s.repo.checkPolicy(ctx, reference); err != nil {
		return err
	}
	ref, err := s.ParseReference(reference)
	if err != nil {
		return err
	}
	if ref.Digest != "" {
		return fmt.Errorf("%w: invalid tag %q", errdef.ErrInvalidReference, ref.GetReference())
	}
	if err := ref.ValidateTag(); err != nil {
		return err
	}
	ctx = auth.AppendRepositoryScope(ctx, ref, auth.ActionDelete)
	url := buildRepositoryManifestURL(s.repo.plainHTTP(), ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := s.repo.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusAccepted, http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("%s: %w", ref, errdef.ErrNotFound)
	default:
		return errutil.ParseErrorResponse(resp)
	}
}

// PushReference pushes the manifest with a reference tag.
//
// When reference is a tag, the registry chooses the stored digest algorithm. A
// Docker-Content-Digest header in a different allowlisted algorithm is accepted
// only if it equals that algorithm's digest of the bytes pushed. Tag pushes
// buffer the manifest up to MaxMetadataBytes (default 4MiB) for that check, so
// an oversized manifest or index fails with [errdef.ErrSizeExceedsLimit].
// expected.Digest is not rewritten; on a registry that indexes only SHA-256,
// Resolve by the caller's digest may fail afterwards, and referrers naming that
// digest may not be found. A digest reference is strict: the URL names the
// identity.
func (s *manifestStore) PushReference(ctx context.Context, expected ocispec.Descriptor, content io.Reader, reference string) error {
	if err := s.repo.checkWritePolicy(ctx, reference); err != nil {
		return err
	}
	ref, err := s.repo.ParseReference(reference)
	if err != nil {
		return err
	}
	return s.pushWithIndexing(ctx, expected, content, ref.GetReference())
}

// verifiedManifest is a body already verified by [content.ReadAll] in
// [manifestStore.pushWithIndexing]. It implements [io.Reader] so it fits
// [manifestStore.push]; push unwraps body so the PUT gets GetBody and does
// not ReadAll again, for both tag and digest references.
type verifiedManifest struct {
	*bytes.Reader
	body []byte
}

// push pushes the manifest content, matching the expected descriptor.
// A tag-addressed push buffers the bytes (capped by MaxMetadataBytes) so a
// registry-chosen Docker-Content-Digest algorithm can be checked against those
// bytes; a digest-addressed push stays strict — see [verifyPushContentDigest].
func (s *manifestStore) push(ctx context.Context, expected ocispec.Descriptor, r io.Reader, reference string) error {
	repoRef := s.repo.reference()
	ref := repoRef
	var pushed []byte
	if verified, ok := r.(*verifiedManifest); ok {
		// Already ReadAll'd by pushWithIndexing; unwrap so the request gets
		// GetBody and is not buffered and verified again.
		pushed = verified.body
		if pushed == nil {
			pushed = []byte{}
		}
		r = bytes.NewReader(pushed)
	}
	if d, err := digest.Parse(reference); err == nil {
		ref.Digest = d.String()
		// digest-addressed pushes stay strict; see [verifyPushContentDigest].
		pushed = nil
	} else {
		ref.Tag = reference
		if pushed == nil {
			if err := limitSize(expected, s.repo.maxMetadataBytes()); err != nil {
				return err
			}
			pushed, err = content.ReadAll(r, expected)
			if err != nil {
				return err
			}
			if pushed == nil {
				pushed = []byte{}
			}
			r = bytes.NewReader(pushed)
		}
	}
	// pushing usually requires both pull and push actions.
	// Reference: https://github.com/distribution/distribution/blob/v2.7.1/registry/handlers/app.go#L921-L930
	ctx = auth.AppendRepositoryScope(ctx, ref, auth.ActionPull, auth.ActionPush)
	url := buildRepositoryManifestURL(s.repo.plainHTTP(), ref)
	// unwrap the content for optimizations of built-in types.
	body := ioutil.UnwrapNopCloser(r)
	if _, ok := body.(io.ReadCloser); ok {
		// undo unwrap if the nopCloser is intended.
		body = r
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, body)
	if err != nil {
		return err
	}
	if req.GetBody != nil && req.ContentLength != expected.Size {
		// short circuit a size mismatch for built-in types.
		return fmt.Errorf("mismatch content length %d: expect %d", req.ContentLength, expected.Size)
	}
	req.ContentLength = expected.Size
	req.Header.Set("Content-Type", expected.MediaType)

	// if the underlying client is an auth client, the content might be read
	// more than once for obtaining the auth challenge and the actual request.
	// To prevent double reading, the manifest is read and stored in the memory,
	// and serve from the memory.
	client := s.repo.client()
	if _, ok := client.(*auth.Client); ok && req.GetBody == nil {
		store := cas.NewMemory()
		err := store.Push(ctx, expected, r)
		if err != nil {
			return err
		}
		req.GetBody = func() (io.ReadCloser, error) {
			return store.Fetch(ctx, expected)
		}
		req.Body, err = req.GetBody()
		if err != nil {
			return err
		}
	}
	resp, err := s.repo.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return errutil.ParseErrorResponse(resp)
	}
	s.checkOCISubjectHeader(resp)
	return verifyPushContentDigest(resp, expected.Digest, pushed)
}

// checkOCISubjectHeader checks the "OCI-Subject" header in the response and
// sets referrers capability accordingly.
// Reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#pushing-manifests-with-subject
func (s *manifestStore) checkOCISubjectHeader(resp *http.Response) {
	// If the "OCI-Subject" header is set, it indicates that the registry
	// supports the Referrers API and has processed the subject of the manifest.
	if subjectHeader := resp.Header.Get(headerOCISubject); subjectHeader != "" {
		s.repo.SetReferrersCapability(true)
	}

	// If the "OCI-Subject" header is NOT set, it means that either the manifest
	// has no subject OR the referrers API is NOT supported by the registry.
	//
	// Since we don't know whether the pushed manifest has a subject or not,
	// we do not set the referrers capability to false at here.
}

// pushWithIndexing pushes the manifest content matching the expected descriptor,
// and indexes referrers for the manifest when needed.
func (s *manifestStore) pushWithIndexing(ctx context.Context, expected ocispec.Descriptor, r io.Reader, reference string) error {
	if err := checkDigestAlgorithm(digestSourceDescriptor, expected.Digest); err != nil {
		return err
	}
	switch expected.MediaType {
	case spec.MediaTypeArtifactManifest, ocispec.MediaTypeImageManifest, ocispec.MediaTypeImageIndex:
		if state := s.repo.loadReferrersState(); state == properties.ReferrersAPISupported {
			// referrers API is available, no client-side indexing needed
			return s.push(ctx, expected, r, reference)
		}

		if err := limitSize(expected, s.repo.maxMetadataBytes()); err != nil {
			return err
		}
		manifestJSON, err := content.ReadAll(r, expected)
		if err != nil {
			return err
		}
		if err := s.push(ctx, expected, &verifiedManifest{
			Reader: bytes.NewReader(manifestJSON),
			body:   manifestJSON,
		}, reference); err != nil {
			return err
		}
		// check referrers API availability again after push
		if state := s.repo.loadReferrersState(); state == properties.ReferrersAPISupported {
			// the subject has been processed the registry, no client-side
			// indexing needed
			return nil
		}
		return s.indexReferrersForPush(ctx, expected, manifestJSON)
	default:
		return s.push(ctx, expected, r, reference)
	}
}

// indexReferrersForPush indexes referrers for manifests with a subject field
// on manifest push.
//
// References:
//   - Latest spec: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#pushing-manifests-with-subject
//   - Compatible spec: https://github.com/opencontainers/distribution-spec/blob/v1.1.0-rc1/spec.md#pushing-manifests-with-subject
func (s *manifestStore) indexReferrersForPush(ctx context.Context, desc ocispec.Descriptor, manifestJSON []byte) error {
	var subject ocispec.Descriptor
	switch desc.MediaType {
	case spec.MediaTypeArtifactManifest:
		var manifest spec.Artifact
		if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
			return fmt.Errorf("failed to decode manifest: %s: %s: %w", desc.Digest, desc.MediaType, err)
		}
		if manifest.Subject == nil {
			// no subject, no indexing needed
			return nil
		}
		subject = *manifest.Subject
		desc.ArtifactType = manifest.ArtifactType
		desc.Annotations = manifest.Annotations
	case ocispec.MediaTypeImageManifest:
		var manifest ocispec.Manifest
		if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
			return fmt.Errorf("failed to decode manifest: %s: %s: %w", desc.Digest, desc.MediaType, err)
		}
		if manifest.Subject == nil {
			// no subject, no indexing needed
			return nil
		}
		subject = *manifest.Subject
		desc.ArtifactType = manifest.ArtifactType
		if desc.ArtifactType == "" {
			desc.ArtifactType = manifest.Config.MediaType
		}
		desc.Annotations = manifest.Annotations
	case ocispec.MediaTypeImageIndex:
		var manifest ocispec.Index
		if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
			return fmt.Errorf("failed to decode manifest: %s: %s: %w", desc.Digest, desc.MediaType, err)
		}
		if manifest.Subject == nil {
			// no subject, no indexing needed
			return nil
		}
		subject = *manifest.Subject
		desc.ArtifactType = manifest.ArtifactType
		desc.Annotations = manifest.Annotations
	default:
		return nil
	}

	// if the manifest has a subject but the remote registry does not process it,
	// it means that the Referrers API is not supported by the registry.
	s.repo.SetReferrersCapability(false)
	return s.updateReferrersIndex(ctx, subject, referrerChange{desc, referrerOperationAdd})
}

// updateReferrersIndex updates the referrers index for desc referencing subject
// on manifest push and manifest delete.
// References:
//   - https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#pushing-manifests-with-subject
//   - https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#deleting-manifests
func (s *manifestStore) updateReferrersIndex(ctx context.Context, subject ocispec.Descriptor, change referrerChange) (err error) {
	referrersTag, err := buildReferrersTag(subject)
	if err != nil {
		return err
	}

	var oldIndexDesc *ocispec.Descriptor
	var oldReferrers []ocispec.Descriptor
	prepare := func() error {
		// 1. pull the original referrers list using the referrers tag schema
		indexDesc, referrers, err := s.repo.referrersFromIndex(ctx, referrersTag, subject)
		if err != nil {
			if errors.Is(err, errdef.ErrNotFound) {
				// valid case: no old referrers index
				return nil
			}
			return err
		}
		oldIndexDesc = &indexDesc
		oldReferrers = referrers
		return nil
	}
	update := func(referrerChanges []referrerChange) error {
		// 2. apply the referrer changes on the referrers list
		updatedReferrers, err := applyReferrerChanges(oldReferrers, referrerChanges)
		if err != nil {
			if err == errNoReferrerUpdate {
				return nil
			}
			return err
		}

		// 3. push the updated referrers list using referrers tag schema
		if len(updatedReferrers) > 0 || s.repo.skipReferrersGC() {
			// push a new index in either case:
			// 1. the referrers list has been updated with a non-zero size
			// 2. OR the updated referrers list is empty but referrers GC
			//    is skipped, in this case an empty index should still be pushed
			//    as the old index won't get deleted
			newIndexDesc, newIndex, err := generateIndex(updatedReferrers, subject.Digest.Algorithm())
			if err != nil {
				return fmt.Errorf("failed to generate referrers index for referrers tag %s: %w", referrersTag, err)
			}
			if err := s.push(ctx, newIndexDesc, bytes.NewReader(newIndex), referrersTag); err != nil {
				return fmt.Errorf("failed to push referrers index tagged by %s: %w", referrersTag, err)
			}
		}

		// 4. delete the dangling original referrers index, if applicable
		if s.repo.skipReferrersGC() || oldIndexDesc == nil {
			return nil
		}
		if err := s.repo.delete(ctx, *oldIndexDesc, true); err != nil {
			return &ReferrersError{
				Op:      opDeleteReferrersIndex,
				Err:     fmt.Errorf("failed to delete dangling referrers index %s for referrers tag %s: %w", oldIndexDesc.Digest.String(), referrersTag, err),
				Subject: subject,
			}
		}
		return nil
	}

	merge, done := s.repo.referrersMergePool.Get(referrersTag)
	defer done()
	return merge.Do(change, prepare, update)
}

// ParseReference parses a reference to a fully qualified reference.
func (s *manifestStore) ParseReference(reference string) (properties.Reference, error) {
	return s.repo.ParseReference(reference)
}

// generateDescriptor returns a descriptor generated from the response.
// See the truth table at the top of `repository_test.go`
func (s *manifestStore) generateDescriptor(resp *http.Response, ref properties.Reference, httpMethod string) (ocispec.Descriptor, error) {
	// 1. Validate Content-Type
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf(
			"%s %q: invalid response `Content-Type` header; %w",
			resp.Request.Method,
			resp.Request.URL,
			err,
		)
	}

	// 2. Validate Size
	if resp.ContentLength == -1 {
		return ocispec.Descriptor{}, fmt.Errorf(
			"%s %q: unknown response Content-Length",
			resp.Request.Method,
			resp.Request.URL,
		)
	}

	// 3. Validate Client Reference
	var refDigest digest.Digest
	if d, err := ref.GetDigest(); err == nil {
		refDigest = d
	}
	// resolve and fetchReference reject a reference digest in an unsupported
	// algorithm before issuing a request; this keeps the check for any other
	// caller and matches the pull verification below.
	if err := checkDigestAlgorithm(digestSourceReference, refDigest); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("%s %q: %w", resp.Request.Method, resp.Request.URL, err)
	}

	// 4. Validate Server Digest (if present)
	var serverHeaderDigest digest.Digest
	if serverHeaderDigestStr := resp.Header.Get(headerDockerContentDigest); serverHeaderDigestStr != "" {
		serverHeaderDigest, err = parseSupportedDigest(digestSourceHeader, serverHeaderDigestStr)
		if err != nil {
			if errors.Is(err, errdef.ErrUnsupported) {
				return ocispec.Descriptor{}, fmt.Errorf("%s %q: %w", resp.Request.Method, resp.Request.URL, err)
			}
			return ocispec.Descriptor{}, fmt.Errorf(
				"%s %q: invalid response header value: `%s: %s`; %w",
				resp.Request.Method,
				resp.Request.URL,
				headerDockerContentDigest,
				serverHeaderDigestStr,
				err,
			)
		}
	}

	/* 5. Now, look for specific error conditions; see truth table in method docstring */
	var contentDigest digest.Digest

	if len(serverHeaderDigest) == 0 {
		if httpMethod == http.MethodHead {
			if len(refDigest) == 0 {
				// HEAD without server `Docker-Content-Digest` header is an
				// immediate fail
				return ocispec.Descriptor{}, fmt.Errorf(
					"HTTP %s request missing required header %q",
					httpMethod, headerDockerContentDigest,
				)
			}
			// Otherwise, just trust the client-supplied digest
			contentDigest = refDigest
		} else {
			// GET without server `Docker-Content-Digest` header forces the
			// expensive calculation
			var calculatedDigest digest.Digest
			algorithm := digest.Canonical
			if len(refDigest) > 0 {
				algorithm = refDigest.Algorithm()
			}
			if calculatedDigest, err = calculateDigestFromResponse(resp, s.repo.maxMetadataBytes(), algorithm); err != nil {
				return ocispec.Descriptor{}, fmt.Errorf("failed to calculate digest on response body; %w", err)
			}
			contentDigest = calculatedDigest
		}
	} else {
		contentDigest = serverHeaderDigest
	}

	if httpMethod == http.MethodGet && len(refDigest) > 0 && len(serverHeaderDigest) > 0 && serverHeaderDigest.Algorithm() != refDigest.Algorithm() {
		// A registry may report a canonical digest using a different algorithm.
		// Preserve the requested identity, verifying it against the body.
		if err := s.verifyPullContentDigest(resp, refDigest); err != nil {
			return ocispec.Descriptor{}, err
		}
		contentDigest = refDigest
	}
	if len(refDigest) > 0 && refDigest != contentDigest {
		return ocispec.Descriptor{}, fmt.Errorf(
			"%s %q: invalid response; digest mismatch in %s: received %q when expecting %q; %w",
			resp.Request.Method, resp.Request.URL,
			headerDockerContentDigest, contentDigest,
			refDigest,
			content.ErrMismatchedDigest,
		)
	}

	// 6. Finally, if we made it this far, then all is good; return.
	return ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    contentDigest,
		Size:      resp.ContentLength,
	}, nil
}

// verifyPullContentDigest permits a different canonical digest algorithm on
// manifest pulls, but verifies the returned bytes against the requested digest.
// Digest-addressed manifest pushes and all blob pushes use
// [verifyPushContentDigest] with a nil body, which is strict: a header
// algorithm that differs from the requested one wraps [errdef.ErrUnsupported].
// Tag-addressed manifest pushes pass the bytes that were sent, so a different
// allowlisted algorithm is accepted only when it hashes those bytes.
// Alternate algorithms buffer at most MaxMetadataBytes before returning the
// verified body to the caller. Headerless responses, regardless of
// Content-Length, are verified only once the body is read to EOF: a caller that
// stops after desc.Size bytes never triggers the check. Resolve and Exists use
// HEAD, so remain strict. A digest algorithm oras-go does not support is
// rejected with errdef.ErrUnsupported.
func (s *manifestStore) verifyPullContentDigest(resp *http.Response, expected digest.Digest) error {
	if err := checkDigestAlgorithm(digestSourceReference, expected); err != nil {
		return fmt.Errorf("%s %q: %w", resp.Request.Method, resp.Request.URL, err)
	}
	// checkDigestAlgorithm leaves a malformed digest to the caller, but a
	// descriptor passed to Fetch may come unparsed from a registry, for example
	// a referrer. go-digest panics on Algorithm and Verifier when the digest
	// has no "algorithm:" prefix, so reject that format here.
	if err := expected.Validate(); errors.Is(err, digest.ErrDigestInvalidFormat) {
		return fmt.Errorf("%s %q: invalid digest %q: %w", resp.Request.Method, resp.Request.URL, expected, err)
	}
	headerStr := resp.Header.Get(headerDockerContentDigest)
	if headerStr == "" {
		// Manifests fetched without a header were never bounded by
		// MaxMetadataBytes. Verify the body as the caller reads it.
		resp.Body = newVerifyReadCloser(resp.Body, expected, resp.Request.Method, resp.Request.URL)
		return nil
	}
	canonical, err := digest.Parse(headerStr)
	if err != nil || canonical.Algorithm() == expected.Algorithm() {
		return verifyPullDigestHeader(resp, expected)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(limitReader(resp.Body, s.repo.maxMetadataBytes()))
	if err != nil {
		return fmt.Errorf("%s %q: failed to read response body: %w", resp.Request.Method, resp.Request.URL, err)
	}
	var extra [1]byte
	if n, err := io.ReadFull(resp.Body, extra[:]); n > 0 {
		return fmt.Errorf("%s %q: %w", resp.Request.Method, resp.Request.URL, errdef.ErrSizeExceedsLimit)
	} else if err != nil && err != io.EOF {
		return fmt.Errorf("%s %q: failed to read response body: %w", resp.Request.Method, resp.Request.URL, err)
	}
	if actual := expected.Algorithm().FromBytes(body); actual != expected {
		return fmt.Errorf("%s %q: invalid response; content digest mismatch: received %q when expecting %q; %w",
			resp.Request.Method, resp.Request.URL, actual, expected, content.ErrMismatchedDigest)
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return nil
}

// verifyReadCloser checks the digest at EOF without buffering the response.
type verifyReadCloser struct {
	io.ReadCloser
	verifier digest.Verifier
	expected digest.Digest
	method   string
	url      *url.URL
	checked  bool
}

func newVerifyReadCloser(rc io.ReadCloser, expected digest.Digest, method string, url *url.URL) io.ReadCloser {
	return &verifyReadCloser{
		ReadCloser: rc,
		verifier:   expected.Verifier(),
		expected:   expected,
		method:     method,
		url:        url,
	}
}

func (v *verifyReadCloser) Read(p []byte) (int, error) {
	n, err := v.ReadCloser.Read(p)
	if n > 0 {
		if _, werr := v.verifier.Write(p[:n]); werr != nil {
			return n, werr
		}
	}
	if err == io.EOF && !v.checked {
		v.checked = true
		if !v.verifier.Verified() {
			return n, fmt.Errorf("%s %q: invalid response; content digest mismatch: expecting %q; %w",
				v.method, v.url, v.expected, content.ErrMismatchedDigest)
		}
	}
	return n, err
}

// calculateDigestFromResponse calculates the actual digest of the response body
// taking care not to destroy it in the process.
func calculateDigestFromResponse(resp *http.Response, maxMetadataBytes int64, algorithm digest.Algorithm) (digest.Digest, error) {
	defer resp.Body.Close()

	body := limitReader(resp.Body, maxMetadataBytes)
	content, err := io.ReadAll(body)
	if err != nil {
		return "", fmt.Errorf("%s %q: failed to read response body: %w", resp.Request.Method, resp.Request.URL, err)
	}
	// the digest of a truncated body would describe different content
	var extra [1]byte
	if n, err := io.ReadFull(resp.Body, extra[:]); n > 0 {
		return "", fmt.Errorf("%s %q: %w", resp.Request.Method, resp.Request.URL, errdef.ErrSizeExceedsLimit)
	} else if err != nil && err != io.EOF {
		return "", fmt.Errorf("%s %q: failed to read response body: %w", resp.Request.Method, resp.Request.URL, err)
	}
	resp.Body = io.NopCloser(bytes.NewReader(content))

	return algorithm.FromBytes(content), nil
}

// verifyContentDigest verifies "Docker-Content-Digest" header if present.
// OCI distribution-spec states the Docker-Content-Digest header is optional.
// Reference: https://github.com/opencontainers/distribution-spec/blob/v1.0.1/spec.md#legacy-docker-support-http-headers
// The header is compared for equality with expected. Pull paths use this
// helper; push paths use [verifyPushContentDigest].
// A header whose algorithm is not on the oras-go allowlist is rejected with
// [errdef.ErrUnsupported] before any comparison, so the digest never becomes an
// identity this package will not act on.
func verifyContentDigest(resp *http.Response, expected digest.Digest) error {
	digestStr := resp.Header.Get(headerDockerContentDigest)

	if len(digestStr) == 0 {
		return nil
	}

	contentDigest, err := parseSupportedDigest(digestSourceHeader, digestStr)
	if err != nil {
		if errors.Is(err, errdef.ErrUnsupported) {
			return fmt.Errorf("%s %q: %w", resp.Request.Method, resp.Request.URL, err)
		}
		return fmt.Errorf(
			"%s %q: invalid response header: `%s: %s`",
			resp.Request.Method, resp.Request.URL,
			headerDockerContentDigest, digestStr,
		)
	}

	if contentDigest != expected {
		return &digestHeaderMismatchError{
			method:   resp.Request.Method,
			url:      resp.Request.URL,
			received: contentDigest,
			expected: expected,
		}
	}

	return nil
}

// verifyPushContentDigest is [verifyContentDigest] for push, mount and similar
// write paths. The header is never trusted as an identity on its own.
//
// If the header equals expected, it is accepted without rehashing: on tag
// pushes [content.ReadAll] already verified expected against the buffered
// bytes. If the header algorithm matches expected, a mismatch is the existing
// [digestHeaderMismatchError] and does not wrap [content.ErrMismatchedDigest].
// If the algorithms differ and pushed is non-nil (tag-addressed manifest push),
// the header is rehashed from pushed and accepted only when it equals that
// algorithm's digest of those bytes. If the algorithms differ and pushed is
// nil (digest-addressed manifest push or any blob path), the error names both
// algorithms and wraps [errdef.ErrUnsupported].
func verifyPushContentDigest(resp *http.Response, expected digest.Digest, pushed []byte) error {
	err := verifyContentDigest(resp, expected)
	if err == nil {
		return nil
	}
	var mismatch *digestHeaderMismatchError
	if !errors.As(err, &mismatch) {
		return err
	}
	if mismatch.received.Algorithm() == expected.Algorithm() {
		return err
	}
	if pushed != nil {
		if mismatch.received == mismatch.received.Algorithm().FromBytes(pushed) {
			return nil
		}
		return err
	}
	return fmt.Errorf("%s %q: %s: digest algorithm %q does not match requested %q: %w",
		mismatch.method, mismatch.url, digestSourceHeader,
		mismatch.received.Algorithm(), expected.Algorithm(), errdef.ErrUnsupported)
}

// digestHeaderMismatchError is returned by [verifyContentDigest] when the
// "Docker-Content-Digest" header disagrees with the expected digest.
//
// It deliberately does not wrap [content.ErrMismatchedDigest]: on push, mount
// and delete the registry is disagreeing about content the client sent, not
// serving bad content. Pull paths add the sentinel through
// [verifyPullDigestHeader].
type digestHeaderMismatchError struct {
	method   string
	url      *url.URL
	received digest.Digest
	expected digest.Digest
}

func (e *digestHeaderMismatchError) Error() string {
	return fmt.Sprintf("%s %q: invalid response; digest mismatch in %s: received %q when expecting %q",
		e.method, e.url, headerDockerContentDigest, e.received, e.expected)
}

// verifyPullDigestHeader is [verifyContentDigest] for pull paths: a digest
// mismatch also satisfies errors.Is(err, [content.ErrMismatchedDigest]), since
// the registry served content that does not match the requested digest. A
// malformed header is returned unwrapped.
func verifyPullDigestHeader(resp *http.Response, expected digest.Digest) error {
	err := verifyContentDigest(resp, expected)
	var mismatch *digestHeaderMismatchError
	if errors.As(err, &mismatch) {
		return fmt.Errorf("%w; %w", err, content.ErrMismatchedDigest)
	}
	return err
}

// generateIndex generates an image index containing the given manifests list.
// The index descriptor is digested using alg. [digest.Canonical] is used if alg
// is empty; an error is returned if alg is unsupported or unavailable.
func generateIndex(manifests []ocispec.Descriptor, alg digest.Algorithm) (ocispec.Descriptor, []byte, error) {
	if manifests == nil {
		manifests = []ocispec.Descriptor{} // make it an empty array to prevent potential server-side bugs
	}
	index := ocispec.Index{
		Versioned: specs.Versioned{
			SchemaVersion: 2, // historical value. does not pertain to OCI or docker version
		},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: manifests,
	}
	indexJSON, err := json.Marshal(index)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	indexDesc, err := content.NewDescriptorFromBytesWithAlgorithm(index.MediaType, indexJSON, alg)
	if err != nil {
		// alg is unsupported or unavailable. A silent fallback to
		// digest.Canonical would restore the referrers tag/index algorithm
		// mismatch this function exists to avoid, so the error is surfaced.
		return ocispec.Descriptor{}, nil, err
	}
	return indexDesc, indexJSON, nil
}
