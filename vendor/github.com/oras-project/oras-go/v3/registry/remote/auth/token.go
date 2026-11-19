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

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"

	"github.com/oras-project/oras-go/v3/registry/remote/credentials"
	"github.com/oras-project/oras-go/v3/registry/remote/errcode"
	"github.com/oras-project/oras-go/v3/registry/remote/internal/errutil"
	"github.com/oras-project/oras-go/v3/registry/remote/properties"
)

// TokenParams contains parameters for token acquisition.
type TokenParams struct {
	// Resource is the registry resource being accessed, including any namespace
	// or repository path when available.
	Resource properties.Resource
	// Realm is the token endpoint URL.
	Realm string
	// Service is the service parameter from the WWW-Authenticate header.
	Service string
	// Scopes are the requested scopes for the token. They are serialized into
	// the wire form by the fetcher, so an implementation does not parse or
	// re-serialize scope strings.
	Scopes []Scope
}

// TokenFetcher abstracts the token acquisition strategy.
type TokenFetcher interface {
	// FetchToken fetches an access token for the given parameters and credential.
	FetchToken(ctx context.Context, params TokenParams, cred credentials.Credential) (string, error)
}

// tokenRegistryURLKey is the context key for the URL the registry was
// contacted at, carried from Client.Do to the token request.
type tokenRegistryURLKey struct{}

// tokenBlockedPrefixesKey is the context key for the realm address denylist
// carried from Client.Do to redirect hops of the token request.
type tokenBlockedPrefixesKey struct{}

// withTokenRegistryURL returns a copy of ctx carrying registryURL, against
// which sendRequest validates every redirect hop of a token request.
func withTokenRegistryURL(ctx context.Context, registryURL *url.URL) context.Context {
	return context.WithValue(ctx, tokenRegistryURLKey{}, registryURL)
}

// tokenRegistryURL returns the registry URL carried by ctx, if any.
func tokenRegistryURL(ctx context.Context) *url.URL {
	registryURL, _ := ctx.Value(tokenRegistryURLKey{}).(*url.URL)
	return registryURL
}

// withTokenBlockedPrefixes returns a copy of ctx carrying prefixes, applied
// to every redirect hop of a token request alongside the registry URL.
func withTokenBlockedPrefixes(ctx context.Context, prefixes []netip.Prefix) context.Context {
	return context.WithValue(ctx, tokenBlockedPrefixesKey{}, prefixes)
}

// tokenBlockedPrefixes returns the realm address denylist carried by ctx, or
// nil (defaults) if none is set.
func tokenBlockedPrefixes(ctx context.Context) []netip.Prefix {
	prefixes, _ := ctx.Value(tokenBlockedPrefixesKey{}).([]netip.Prefix)
	return prefixes
}

// sendRequest adds the custom headers to the request and sends it using the
// given client. If client is nil, http.DefaultClient is used.
//
// The client is wrapped by redirectSafeClient so that a token endpoint
// redirect crossing an HTTP origin does not carry the credential along. A
// request body is not resent across origins either: on a 307 or 308 redirect
// net/http replays the body, which for an OAuth2 token request holds the
// password or refresh token.
//
// When the request context carries the registry URL (set by Client.Do), every
// redirect hop is also validated by validateRealm against that URL, so that a
// validated realm cannot redirect the token request into a blocked prefix
// or downgrade it from https to http.
//
// References:
//   - https://github.com/oras-project/oras-go/security/advisories/GHSA-mm27-wq9c-mxxr
//   - https://github.com/oras-project/oras-go/security/advisories/GHSA-xc97-9f2f-9hrx
func sendRequest(client *http.Client, header http.Header, req *http.Request) (*http.Response, error) {
	for key, values := range header {
		req.Header[key] = append(req.Header[key], values...)
	}
	if registryURL := tokenRegistryURL(req.Context()); registryURL != nil {
		client = realmRedirectGuard(client, registryURL, tokenBlockedPrefixes(req.Context()))
	}
	safeClient := redirectSafeClient(client)
	checkRedirect := safeClient.CheckRedirect
	safeClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.Body != nil && req.Body != http.NoBody && leftOrigin(via, req) {
			return fmt.Errorf("%s %q: refusing to resend a token request body to another origin", req.Method, req.URL.Redacted())
		}
		return checkRedirect(req, via)
	}
	return safeClient.Do(req)
}

// decodeTokenResponse decodes a token endpoint response body and returns the
// access token.
//
// As specified in https://distribution.github.io/distribution/spec/auth/token/
// section "Token Response Fields", the token is carried in either the `token`
// or `access_token` field; if both are present they are identical. OAuth2
// responses use `access_token` (see
// https://distribution.github.io/distribution/spec/auth/oauth/).
func decodeTokenResponse(resp *http.Response) (string, error) {
	var result struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	lr := io.LimitReader(resp.Body, maxResponseBytes)
	if err := json.NewDecoder(lr).Decode(&result); err != nil {
		return "", fmt.Errorf("%s %q: failed to decode response: %w", resp.Request.Method, resp.Request.URL, err)
	}
	if result.AccessToken != "" {
		return result.AccessToken, nil
	}
	if result.Token != "" {
		return result.Token, nil
	}
	return "", fmt.Errorf("%s %q: empty token returned", resp.Request.Method, resp.Request.URL)
}

// DistributionTokenFetcher implements distribution spec token endpoint (GET).
// This fetches tokens using the legacy distribution specification.
//
// References:
//   - https://distribution.github.io/distribution/spec/auth/jwt/
//   - https://distribution.github.io/distribution/spec/auth/token/
type DistributionTokenFetcher struct {
	// Client is the underlying HTTP client used to send requests.
	// If nil, http.DefaultClient is used.
	Client *http.Client
	// Header contains custom headers to be added to each request.
	Header http.Header
}

// FetchToken fetches an access token using the distribution spec token endpoint.
// It fetches anonymous tokens if no credential is provided.
func (f *DistributionTokenFetcher) FetchToken(ctx context.Context, params TokenParams, cred credentials.Credential) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, params.Realm, nil)
	if err != nil {
		return "", err
	}
	if cred.Username != "" || cred.Password != "" {
		req.SetBasicAuth(cred.Username, cred.Password)
	}
	q := req.URL.Query()
	if params.Service != "" {
		q.Set("service", params.Service)
	}
	for _, scope := range params.Scopes {
		if s := scope.String(); s != "" {
			q.Add("scope", s)
		}
	}
	req.URL.RawQuery = q.Encode()

	resp, err := sendRequest(f.Client, f.Header, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errutil.ParseErrorResponse(resp)
	}
	return decodeTokenResponse(resp)
}

// OAuth2TokenFetcher implements OAuth2 password/refresh_token grants (POST).
// This fetches tokens using the OAuth2 specification.
//
// Reference: https://distribution.github.io/distribution/spec/auth/oauth/
type OAuth2TokenFetcher struct {
	// Client is the underlying HTTP client used to send requests.
	// If nil, http.DefaultClient is used.
	Client *http.Client
	// Header contains custom headers to be added to each request.
	Header http.Header
	// ClientID is used in fetching OAuth2 token as a required field.
	// If empty, a default client ID is used.
	ClientID string
}

// FetchToken fetches an OAuth2 access token.
func (f *OAuth2TokenFetcher) FetchToken(ctx context.Context, params TokenParams, cred credentials.Credential) (string, error) {
	form := url.Values{}
	if cred.RefreshToken != "" {
		form.Set("grant_type", "refresh_token")
		form.Set("refresh_token", cred.RefreshToken)
	} else if cred.Username != "" && cred.Password != "" {
		form.Set("grant_type", "password")
		form.Set("username", cred.Username)
		form.Set("password", cred.Password)
	} else {
		return "", errors.New("missing username or password for bearer auth")
	}
	form.Set("service", params.Service)
	clientID := f.ClientID
	if clientID == "" {
		clientID = defaultClientID
	}
	form.Set("client_id", clientID)
	if scope := joinScopes(params.Scopes); scope != "" {
		form.Set("scope", scope)
	}
	body := strings.NewReader(form.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, params.Realm, body)
	if err != nil {
		return "", err
	}
	req.Header.Set(headerContentType, "application/x-www-form-urlencoded")

	resp, err := sendRequest(f.Client, f.Header, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errutil.ParseErrorResponse(resp)
	}
	return decodeTokenResponse(resp)
}

// CompositeTokenFetcher selects strategy based on credential type and legacy mode.
// It delegates to either DistributionTokenFetcher or OAuth2TokenFetcher based
// on the credential and configuration.
type CompositeTokenFetcher struct {
	// Distribution is the fetcher for distribution spec tokens.
	Distribution TokenFetcher
	// OAuth2 is the fetcher for OAuth2 tokens.
	OAuth2 TokenFetcher
	// LegacyMode controls whether to use the legacy distribution spec
	// instead of OAuth2 with password grant when authenticating using
	// username and password.
	LegacyMode bool
	// StrictOAuth2 disables the fallback to the distribution spec. By default,
	// when the token server rejects the OAuth2 password grant in a way that
	// means it does not implement OAuth2, the request is retried as a
	// distribution spec token request. If StrictOAuth2 is true, the OAuth2
	// error is returned instead.
	StrictOAuth2 bool

	// distributionRealms remembers the realms that fell back to the
	// distribution spec, so that later fetches do not pay for the OAuth2
	// attempt again. A nil value remembers nothing.
	distributionRealms *realmSet
}

// realmSet is a set of token realms, safe for concurrent use. The zero value
// is an empty set and a nil *realmSet is a set that never remembers.
type realmSet struct {
	realms sync.Map
}

func (s *realmSet) has(realm string) bool {
	if s == nil {
		return false
	}
	_, ok := s.realms.Load(realm)
	return ok
}

func (s *realmSet) add(realm string) {
	if s != nil {
		s.realms.Store(realm, struct{}{})
	}
}

// FetchToken selects the appropriate fetcher and delegates the token acquisition.
// The selection logic is:
//   - If credential has an access token, return it directly.
//   - If credential is empty or (has no refresh token and legacy mode is enabled),
//     use the distribution fetcher.
//   - Otherwise, use the OAuth2 fetcher. If the token server rejects a
//     username/password grant with 400, 401, 404 or 405, which registries
//     without an OAuth2 token endpoint answer, retry with the distribution
//     fetcher unless StrictOAuth2 is set. If that fails too, both errors are
//     returned. Refresh tokens are never retried, as they cannot be used with
//     the distribution spec.
func (f *CompositeTokenFetcher) FetchToken(ctx context.Context, params TokenParams, cred credentials.Credential) (string, error) {
	// Return access token directly if provided
	if cred.AccessToken != "" {
		return cred.AccessToken, nil
	}

	// Use distribution fetcher for empty credentials or legacy mode without refresh token
	if cred == credentials.EmptyCredential || (cred.RefreshToken == "" && f.LegacyMode) {
		return f.Distribution.FetchToken(ctx, params, cred)
	}

	canFallback := cred.RefreshToken == "" && !f.StrictOAuth2
	if canFallback && f.distributionRealms.has(params.Realm) {
		return f.Distribution.FetchToken(ctx, params, cred)
	}

	// Use OAuth2 fetcher otherwise
	token, oauth2Err := f.OAuth2.FetchToken(ctx, params, cred)
	if oauth2Err == nil || !canFallback || !isOAuth2Unsupported(oauth2Err) {
		return token, oauth2Err
	}
	token, err := f.Distribution.FetchToken(ctx, params, cred)
	if err != nil {
		return "", errors.Join(err, oauth2Err)
	}
	f.distributionRealms.add(params.Realm)
	return token, nil
}

// isOAuth2Unsupported reports whether err is the response of a token server
// that does not implement the OAuth2 token endpoint. Registries are known to
// answer the OAuth2 POST with 400 (ACR), 401 (JFrog Artifactory), 404 (GCR,
// GitLab) or 405 (GHCR).
func isOAuth2Unsupported(err error) bool {
	var errResp *errcode.ErrorResponse
	if !errors.As(err, &errResp) {
		return false
	}
	switch errResp.StatusCode {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusMethodNotAllowed:
		return true
	}
	return false
}

// NewCompositeTokenFetcher creates a new CompositeTokenFetcher with default
// Distribution and OAuth2 fetchers using the provided client and header.
func NewCompositeTokenFetcher(client *http.Client, header http.Header, clientID string, legacyMode bool) *CompositeTokenFetcher {
	return &CompositeTokenFetcher{
		Distribution: &DistributionTokenFetcher{
			Client: client,
			Header: header,
		},
		OAuth2: &OAuth2TokenFetcher{
			Client:   client,
			Header:   header,
			ClientID: clientID,
		},
		LegacyMode:         legacyMode,
		distributionRealms: &realmSet{},
	}
}
