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

// Package remote provides a client to the remote registry.
// Reference: https://github.com/distribution/distribution
package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/registry"
	"github.com/oras-project/oras-go/v3/registry/remote/auth"
	"github.com/oras-project/oras-go/v3/registry/remote/internal/errutil"
	"github.com/oras-project/oras-go/v3/registry/remote/policy"
	"github.com/oras-project/oras-go/v3/registry/remote/properties"
)

// Registry is an HTTP client to a remote registry.
type Registry struct {
	// Client is the underlying HTTP client used to access the remote registry.
	// If nil, auth.DefaultClient is used.
	Client Client

	// Reference contains registry host information.
	Reference properties.Reference

	// PlainHTTP signals the transport to access the registry via HTTP
	// instead of HTTPS.
	PlainHTTP bool

	// HandleWarning handles the warning returned by the remote server.
	// Callers SHOULD deduplicate warnings from multiple associated responses.
	//
	// References:
	//   - https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#warnings
	//   - https://www.rfc-editor.org/rfc/rfc7234#section-5.5
	HandleWarning func(warning Warning)

	// Policy is an optional policy evaluator for allow/deny decisions.
	// If nil, no policy enforcement is performed.
	// Reference: https://man.archlinux.org/man/containers-policy.json.5.en
	Policy *policy.Evaluator

	// MaxMetadataBytes specifies a limit on how many response bytes are allowed
	// in the server's response to the metadata APIs, such as catalog list, tag
	// list, and referrers list.
	// If less than or equal to zero, a default (currently 4MiB) is used.
	MaxMetadataBytes int64

	// RepositoryListPageSize specifies the page size when invoking the catalog
	// API.
	// If zero, the page size is determined by the remote registry.
	// Reference: https://distribution.github.io/distribution/spec/api/#catalog
	RepositoryListPageSize int

	// ManifestMediaTypes is the default for repositories.
	// Used in `Accept` header for resolving manifests from references.
	// It is also used in identifying manifests and blobs from descriptors.
	// If an empty list is present, default manifest media types are used.
	ManifestMediaTypes []string

	// TagListPageSize is the default for repositories.
	// If zero, the page size is determined by the remote registry.
	TagListPageSize int

	// ReferrerListPageSize is the default for repositories.
	// If zero, the page size is determined by the remote registry.
	ReferrerListPageSize int

	// RepositoryListMaxPages is the maximum number of pages to fetch during
	// catalog listing. Zero means unlimited.
	RepositoryListMaxPages int

	// TagListMaxPages is the default maximum number of pages to fetch during
	// tag listing. Zero means unlimited.
	TagListMaxPages int

	// ReferrerListMaxPages is the default maximum number of pages to fetch
	// during referrer listing. Zero means unlimited.
	ReferrerListMaxPages int

	// MaxChunkSize is the default target size in bytes for each PATCH chunk
	// when pushing blobs to repositories. If less than or equal to zero,
	// blobs are pushed monolithically unless a repository overrides it.
	MaxChunkSize int64

	// SingleRequestPushThreshold is the default size limit in bytes for
	// pushing blobs to repositories in a single POST request. If less than or
	// equal to zero, blobs are pushed with a POST followed by a PUT unless a
	// repository overrides it.
	SingleRequestPushThreshold int64

	// MaxUnsafePushBytes is the default maximum number of bytes UnsafePush
	// accepts for both known and unknown sizes when pushing blobs to
	// repositories. If less than or equal to zero, a default (currently 4 GiB)
	// is used unless a repository overrides it.
	MaxUnsafePushBytes int64

	// SkipReferrersGC is the default for repositories.
	// If false, the old referrers index will be deleted after the new one
	// is successfully uploaded.
	SkipReferrersGC bool
}

// NewRegistry creates a client to the remote registry with the specified domain
// name.
// Example: localhost:5000
func NewRegistry(name string) (*Registry, error) {
	ref := properties.Reference{
		Registry: name,
	}
	if err := ref.ValidateRegistry(); err != nil {
		return nil, err
	}
	return &Registry{
		Reference: ref,
	}, nil
}

// client returns an HTTP client used to access the remote registry.
// A default HTTP client is returned if the client is not configured.
func (r *Registry) client() Client {
	if r.Client == nil {
		return auth.DefaultClient
	}
	return r.Client
}

// maxMetadataBytes returns the maximum metadata bytes limit.
func (r *Registry) maxMetadataBytes() int64 {
	return r.MaxMetadataBytes
}

// do sends an HTTP request and returns an HTTP response using the HTTP client
// returned by r.client().
func (r *Registry) do(req *http.Request) (*http.Response, error) {
	if r.HandleWarning == nil {
		return r.client().Do(req)
	}

	resp, err := r.client().Do(req)
	if err != nil {
		return nil, err
	}
	handleWarningHeaders(resp.Header.Values(headerWarning), r.HandleWarning)
	return resp, nil
}

// Ping checks whether or not the registry implement Docker Registry API V2 or
// OCI Distribution Specification.
// Ping can be used to check authentication when an auth client is configured.
//
// References:
//   - https://distribution.github.io/distribution/spec/api/#base
//   - https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#api
func (r *Registry) Ping(ctx context.Context) error {
	ref := properties.Reference{Registry: r.Reference.Registry}
	url := buildRegistryBaseURL(r.PlainHTTP, ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := r.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return errdef.ErrNotFound
	default:
		return errutil.ParseErrorResponse(resp)
	}
}

// Repositories lists the name of repositories available in the registry.
// See also `RepositoryListPageSize`.
//
// If `last` is NOT empty, the entries in the response start after the
// repo specified by `last`. Otherwise, the response starts from the top
// of the Repositories list.
//
// Reference: https://distribution.github.io/distribution/spec/api/#catalog
func (r *Registry) Repositories(ctx context.Context, last string, fn func(repos []string) error) error {
	ref := properties.Reference{Registry: r.Reference.Registry}
	ctx = auth.AppendScopesForResource(ctx, ref.Resource(), auth.ScopeRegistryCatalog())
	url := buildRegistryCatalogURL(r.PlainHTTP, ref)
	var err error
	maxPages := r.RepositoryListMaxPages
	for page := 0; err == nil; page++ {
		if maxPages > 0 && page >= maxPages {
			return fmt.Errorf("repository listing exceeded %d pages: %w", maxPages, errdef.ErrTooManyPages)
		}
		url, err = r.repositories(ctx, last, fn, url)
		// clear `last` for subsequent pages
		last = ""
	}
	if err != errNoLink {
		return err
	}
	return nil
}

// repositories returns a single page of repository list with the next link.
func (r *Registry) repositories(ctx context.Context, last string, fn func(repos []string) error, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if r.RepositoryListPageSize > 0 || last != "" {
		q := req.URL.Query()
		if r.RepositoryListPageSize > 0 {
			q.Set("n", strconv.Itoa(r.RepositoryListPageSize))
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
		Repositories []string `json:"repositories"`
	}
	lr := limitReader(resp.Body, r.maxMetadataBytes())
	if err := json.NewDecoder(lr).Decode(&page); err != nil {
		return "", fmt.Errorf("%s %q: failed to decode response: %w", resp.Request.Method, resp.Request.URL, err)
	}
	if err := fn(page.Repositories); err != nil {
		return "", err
	}

	return parseLink(resp)
}

// Repository returns a repository reference by the given name.
func (r *Registry) Repository(ctx context.Context, name string) (registry.Repository, error) {
	return r.newRepository(name)
}

// newRepository creates a new Repository with the given name.
func (r *Registry) newRepository(name string) (*Repository, error) {
	ref := properties.Reference{
		Registry:   r.Reference.Registry,
		Repository: name,
	}
	if err := ref.ValidateRepository(); err != nil {
		return nil, err
	}
	return &Repository{
		Registry:       r,
		RepositoryName: name,
	}, nil
}
