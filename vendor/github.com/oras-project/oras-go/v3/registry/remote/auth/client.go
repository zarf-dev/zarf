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

// Package auth provides authentication for a client to a remote registry.
package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/oras-project/oras-go/v3/registry/remote/credentials"
	"github.com/oras-project/oras-go/v3/registry/remote/errcode"
	"github.com/oras-project/oras-go/v3/registry/remote/internal/urlutil"
	"github.com/oras-project/oras-go/v3/registry/remote/properties"
	"github.com/oras-project/oras-go/v3/registry/remote/retry"
)

// HTTP header names used in authentication.
const (
	headerAuthorization   = "Authorization"
	headerContentType     = "Content-Type"
	headerUserAgent       = "User-Agent"
	headerWWWAuthenticate = "Www-Authenticate"
)

// ErrBasicCredentialNotFound is returned  when the credential is not found for
// basic auth.
var ErrBasicCredentialNotFound = errors.New("basic credential not found")

// DefaultClient is the default auth-decorated client.
var DefaultClient = &Client{
	Client: retry.DefaultClient,
	Header: http.Header{
		headerUserAgent: {"oras-go"},
	},
	Cache: DefaultCache,
}

// maxResponseBytes specifies the default limit on how many response bytes are
// allowed in the server's response from authorization service servers.
// A typical response message from authorization service servers is around 1 to
// 4 KiB. Since the size of a token must be smaller than the HTTP header size
// limit, which is usually 16 KiB. As specified by the distribution, the
// response may contain 2 identical tokens, that is, 16 x 2 = 32 KiB.
// Hence, 128 KiB should be sufficient.
// References: https://distribution.github.io/distribution/spec/auth/token/
var maxResponseBytes int64 = 128 * 1024 // 128 KiB

// defaultMaxRedirects mirrors the limit enforced by net/http when
// http.Client.CheckRedirect is nil. Installing a CheckRedirect replaces that
// default policy, so redirectSafeClient must enforce the limit itself.
const defaultMaxRedirects = 10

// defaultClientID specifies the default client ID used in OAuth2.
// See also ClientID.
var defaultClientID = "oras-go"

// Client is an auth-decorated HTTP client.
// Its zero value is a usable client that uses http.DefaultClient with no cache.
type Client struct {
	// Client is the underlying HTTP client used to access the remote
	// server.
	// If nil, http.DefaultClient is used.
	// It is possible to use the default retry client from the package
	// `github.com/oras-project/oras-go/v3/registry/remote/retry`. That client
	// is already available in the DefaultClient.
	// It is also possible to use a custom client. For example, github.com/hashicorp/go-retryablehttp
	// is a popular HTTP client that supports retries.
	//
	// The Client field and the *http.Client it points to are both read once,
	// on the first request sent through this Client, to build the
	// redirect-safe wrapper used for every subsequent request. After that
	// first request, neither reassigning Client nor mutating the fields of
	// the http.Client it points to (Transport, Timeout, Jar, CheckRedirect)
	// has any effect. Configure the client fully before first use, or derive
	// a separately-configured one with Clone.
	Client *http.Client

	// Header contains the custom headers to be added to each request.
	Header http.Header

	// CredentialFunc specifies the function for resolving the credential for the
	// given registry resource.
	// It is called for the host of every challenged request, including blob
	// upload locations on a host other than the registry, so it must only
	// return a credential for the hosts that credential belongs to.
	// EmptyCredential is a valid return value and should not be considered as
	// an error.
	// If nil, the credential is always resolved to EmptyCredential.
	CredentialFunc credentials.CredentialFunc

	// Cache caches credentials for direct accessing the remote registry.
	// If nil, no cache is used.
	Cache Cache

	// ClientID used in fetching OAuth2 token as a required field.
	// If empty, a default client ID is used.
	// Reference: https://distribution.github.io/distribution/spec/auth/oauth/#getting-a-token
	ClientID string

	// TokenFetcher determines how a bearer token is acquired when the registry
	// issues a Bearer challenge.
	//
	// If nil, the client fetches an anonymous token via the distribution spec
	// when there is no credential, and otherwise uses OAuth2 with password
	// grant. If the token server rejects the password grant with 400, 401, 404
	// or 405, which registries without an OAuth2 token endpoint do, the client
	// retries with the distribution spec and remembers that for the realm.
	// To follow the legacy distribution spec from the start — the approach used
	// in oras-go v1 and v2 — set a composite fetcher in legacy mode:
	//
	//	client.TokenFetcher = auth.NewCompositeTokenFetcher(
	//		client.Client, client.Header, client.ClientID, true)
	//
	// To use OAuth2 only, without the retry, set StrictOAuth2 on a composite
	// fetcher.
	//
	// Note that in every case the registry itself is authenticated with a
	// bearer token; they differ only in how that token is obtained. The
	// authentication scheme itself is not configurable — it is whatever the
	// registry advertises in its WWW-Authenticate challenge.
	//
	// References:
	// - https://distribution.github.io/distribution/spec/auth/jwt/
	// - https://distribution.github.io/distribution/spec/auth/oauth/
	TokenFetcher TokenFetcher

	// BlockedRealmPrefixes is the denylist of address prefixes a bearer realm
	// host must not be, or resolve to. A realm is rejected if any of its
	// addresses falls in a listed prefix.
	//
	// nil uses DefaultBlockedRealmPrefixes (loopback, link-local,
	// unspecified, and cloud metadata). An empty non-nil slice blocks
	// nothing. A non-empty slice is used exactly, without merging defaults.
	// The scheme check and the https-to-http downgrade check always apply.
	// RFC 1918, IPv6 unique-local, and carrier-grade NAT addresses are
	// allowed by default. Strict mode is
	// append(DefaultBlockedRealmPrefixes(), PrivateRealmPrefixes()...).
	BlockedRealmPrefixes []netip.Prefix

	// redirectSafeClientOnce guards the lazy construction of
	// redirectSafeClientValue below.
	redirectSafeClientOnce sync.Once

	// redirectSafeClientValue caches the client returned by
	// redirectSafeClient(c.client()). The wrapper is stateless with respect
	// to the request being sent, so it only needs to be built once per
	// Client rather than on every send.
	redirectSafeClientValue *http.Client

	// distributionRealms remembers the token realms for which the default
	// token fetcher had to fall back from OAuth2 to the distribution spec.
	distributionRealms realmSet
}

// Clone returns a shallow copy of c's exported configuration. Assigning to the
// clone's fields does not affect c — for example to attach a different
// CredentialFunc before use — but the Header map and the Cache and Client
// values it references are shared with c, so mutating those through the clone
// is visible to c as well. BlockedRealmPrefixes is copied so index or append
// mutations on the clone do not change c.
//
// The clone does not inherit c's cached, lazily-built redirect-safe client or
// its record of the realms that needed the distribution spec token flow;
// it builds and caches its own the first time it sends a request. This makes
// Clone the supported way to derive an independent Client: assigning a
// Client value directly (dst := *c) also copies its internal
// synchronization state and is unsafe once c has sent a request.
func (c *Client) Clone() *Client {
	return &Client{
		Client:               c.Client,
		Header:               c.Header,
		CredentialFunc:       c.CredentialFunc,
		Cache:                c.Cache,
		ClientID:             c.ClientID,
		TokenFetcher:         c.TokenFetcher,
		BlockedRealmPrefixes: slices.Clone(c.BlockedRealmPrefixes),
	}
}

// client returns an HTTP client used to access the remote registry.
// http.DefaultClient is return if the client is not configured.
func (c *Client) client() *http.Client {
	if c.Client == nil {
		return http.DefaultClient
	}
	return c.Client
}

// cachedRedirectSafeClient returns the redirect-safe wrapper around c.client(),
// building it on the first call and reusing it for every subsequent call. Safe
// for concurrent use.
func (c *Client) cachedRedirectSafeClient() *http.Client {
	c.redirectSafeClientOnce.Do(func() {
		c.redirectSafeClientValue = redirectSafeClient(c.client())
	})
	return c.redirectSafeClientValue
}

// send adds headers to the request and sends the request to the remote server.
func (c *Client) send(req *http.Request) (*http.Response, error) {
	for key, values := range c.Header {
		req.Header[key] = append(req.Header[key], values...)
	}
	return c.cachedRedirectSafeClient().Do(req)
}

// redirectSafeClient returns a shallow copy of client whose CheckRedirect drops
// the Authorization header once a redirect chain has left the HTTP origin
// (scheme, host, or port) of its first request, or when a redirect moves
// between repositories under the distribution API. The standard library only
// strips sensitive headers when the hostname changes, so a redirect to a
// different port or repository on the same host would otherwise forward
// credentials outside their intended scope. Any caller-provided CheckRedirect
// is preserved.
//
// The check is made against the first request rather than the previous hop:
// net/http rebuilds the headers of every redirected request from the first
// request, so a header deleted on one hop is restored on the next.
//
// This applies to token endpoint requests as well as registry requests: the
// distribution spec flow sends the credential to the realm as HTTP Basic.
//
// Reference: https://github.com/oras-project/oras-go/security/advisories/GHSA-vh4v-2xq2-g5cg
func redirectSafeClient(client *http.Client) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	clientCopy := *client
	checkRedirect := client.CheckRedirect
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 0 && (leftOrigin(via, req) || crossesRepository(via[0].URL, req.URL)) {
			req.Header.Del(headerAuthorization)
		}
		if checkRedirect != nil {
			return checkRedirect(req, via)
		}
		if len(via) >= defaultMaxRedirects {
			return fmt.Errorf("stopped after %d redirects", defaultMaxRedirects)
		}
		return nil
	}
	return &clientCopy
}

// leftOrigin reports whether the redirect chain via, followed by req, has left
// the HTTP origin of its first request, which is the request the credential
// was attached for. Once the chain leaves that origin it is treated as having
// left for good, even if a later hop returns to it.
//
// Reference: https://github.com/oras-project/oras-go/security/advisories/GHSA-xc97-9f2f-9hrx
func leftOrigin(via []*http.Request, req *http.Request) bool {
	for _, r := range via[1:] {
		if !urlutil.SameOrigin(via[0].URL, r.URL) {
			return true
		}
	}
	return !urlutil.SameOrigin(via[0].URL, req.URL)
}

// isHTTPSUpgrade reports whether a redirect upgrades the same host from the
// standard HTTP port to the standard HTTPS port.
func isHTTPSUpgrade(from, to *url.URL) bool {
	return from.Scheme == "http" && to.Scheme == "https" &&
		urlutil.SameHostname(from, to) &&
		(from.Port() == "" || from.Port() == "80") &&
		(to.Port() == "" || to.Port() == "443")
}

// safeHTTPSUpgrade checks every hop before accepting an upgraded challenge.
func safeHTTPSUpgrade(original, last *http.Request) bool {
	if !isHTTPSUpgrade(original.URL, last.URL) {
		return false
	}
	if original.Host != "" && !strings.EqualFold(original.Host, original.URL.Host) {
		return false
	}
	for req := last; ; {
		if crossesRepository(original.URL, req.URL) || req.URL.User != nil {
			return false
		}
		if req.Response == nil {
			return urlutil.SameOrigin(original.URL, req.URL)
		}
		previous := req.Response.Request
		if previous == nil || (!urlutil.SameOrigin(previous.URL, req.URL) && !isHTTPSUpgrade(previous.URL, req.URL)) {
			return false
		}
		req = previous
	}
}

// credential resolves the credential for the given registry resource.
func (c *Client) credential(ctx context.Context, resource properties.Resource) (credentials.Credential, error) {
	if c.CredentialFunc == nil {
		return credentials.EmptyCredential, nil
	}
	return c.CredentialFunc(ctx, resource)
}

// cache resolves the cache.
// noCache is return if the cache is not configured.
func (c *Client) cache() Cache {
	if c.Cache == nil {
		return noCache{}
	}
	return c.Cache
}

// lookupIPAddr resolves a host name to its IP addresses. It is a variable so
// that tests can inject a fake resolver.
var lookupIPAddr = net.DefaultResolver.LookupIPAddr

var (
	nat64Prefix  = netip.MustParsePrefix("64:ff9b::/96")
	sixTo4Prefix = netip.MustParsePrefix("2002::/16")

	defaultBlockedRealmPrefixes = []netip.Prefix{
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("::1/128"),
		netip.MustParsePrefix("169.254.0.0/16"),
		netip.MustParsePrefix("fe80::/10"),
		netip.MustParsePrefix("224.0.0.0/24"),
		netip.MustParsePrefix("ff02::/16"),
		netip.MustParsePrefix("0.0.0.0/8"),
		netip.MustParsePrefix("::/128"),
		netip.MustParsePrefix("100.100.100.200/32"),
		netip.MustParsePrefix("fd00:ec2::254/128"),
		netip.MustParsePrefix("fd00:ec2::23/128"),
		netip.MustParsePrefix("fd20:ce::254/128"),
		netip.MustParsePrefix("168.63.129.16/32"),
	}

	privateRealmPrefixes = []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.168.0.0/16"),
		netip.MustParsePrefix("fc00::/7"),
		netip.MustParsePrefix("100.64.0.0/10"),
	}
)

// DefaultBlockedRealmPrefixes returns a fresh copy of the default realm
// address denylist: loopback, link-local (including IMDS), link-local
// multicast, unspecified, and cloud metadata addresses outside link-local.
// RFC 1918, IPv6 unique-local, and carrier-grade NAT are not included.
func DefaultBlockedRealmPrefixes() []netip.Prefix {
	return slices.Clone(defaultBlockedRealmPrefixes)
}

// PrivateRealmPrefixes returns a fresh copy of RFC 1918, IPv6 unique-local
// (fc00::/7), and carrier-grade NAT (100.64.0.0/10) prefixes. Append them to
// DefaultBlockedRealmPrefixes for a strict denylist.
func PrivateRealmPrefixes() []netip.Prefix {
	return slices.Clone(privateRealmPrefixes)
}

// unwrapRealmAddr returns the IPv4 address embedded in an IPv4-mapped,
// NAT64, or 6to4 IPv6 address. Other addresses are returned unmapped.
// The IPv6 zone is stripped so Prefix.Contains can match scoped literals
// such as [::1%lo0].
func unwrapRealmAddr(addr netip.Addr) netip.Addr {
	addr = addr.Unmap().WithZone("")
	if !addr.Is6() {
		return addr
	}
	b := addr.As16()
	if nat64Prefix.Contains(addr) {
		return netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]})
	}
	if sixTo4Prefix.Contains(addr) {
		return netip.AddrFrom4([4]byte{b[2], b[3], b[4], b[5]})
	}
	return addr
}

// blockedPrefix reports whether addr, after unwrapping, falls in any prefix.
func blockedPrefix(addr netip.Addr, prefixes []netip.Prefix) (netip.Prefix, bool) {
	check := unwrapRealmAddr(addr)
	for _, p := range prefixes {
		if p.Contains(check) || p.Contains(addr) {
			return p, true
		}
	}
	return netip.Prefix{}, false
}

// resolveBlockedPrefixes returns the denylist to apply. nil means defaults;
// a non-nil slice (including empty) is used as given.
func resolveBlockedPrefixes(prefixes []netip.Prefix) []netip.Prefix {
	if prefixes == nil {
		return defaultBlockedRealmPrefixes
	}
	return prefixes
}

// validateRealm rejects bearer token realm URLs that would have the client
// forward credentials to obviously unsafe destinations:
//
//   - schemes other than http or https,
//   - http realms when the registry was contacted over https (TLS downgrade),
//   - hosts that are, or resolve to, an address in prefixes (nil uses
//     DefaultBlockedRealmPrefixes; an empty non-nil slice blocks nothing).
//
// Cross-host realms with a public hostname are permitted, because the
// distribution spec allows a separate token endpoint (e.g. Docker Hub's
// auth.docker.io). When the registry itself is reached at the same hostname
// as the realm, the address check is skipped so loopback and in-cluster
// deployments continue to work.
//
// IPv4-mapped, NAT64, and 6to4 addresses are unwrapped before matching.
// The scheme and downgrade checks always apply.
//
// A realm host that is not an IP literal is resolved with ctx, and the realm
// is rejected if any of its A or AAAA records is in a blocked prefix. If the
// lookup fails, the realm is allowed: behind an HTTP proxy the client may be
// unable to resolve names that the proxy can, and a bogus name fails at dial
// time anyway. The check does not defend against DNS rebinding, where the
// name resolves to a public address here and to a blocked prefix when the
// request is dialed.
//
// validateRealm is also applied to every redirect hop of a token request.
//
// References:
//   - https://github.com/oras-project/oras-go/security/advisories/GHSA-xf85-363p-868w
//   - https://github.com/oras-project/oras-go/security/advisories/GHSA-mm27-wq9c-mxxr
func validateRealm(ctx context.Context, realm string, registryURL *url.URL, prefixes []netip.Prefix) error {
	if realm == "" {
		return nil
	}
	realmURL, err := url.Parse(realm)
	if err != nil {
		return fmt.Errorf("failed to parse bearer realm %q: %w", realm, err)
	}
	switch realmURL.Scheme {
	case "https":
		// always allowed
	case "http":
		if registryURL != nil && registryURL.Scheme == "https" {
			return fmt.Errorf("bearer realm %q uses http but registry was contacted over https", realm)
		}
	default:
		return fmt.Errorf("bearer realm %q uses unsupported scheme %q", realm, realmURL.Scheme)
	}
	host := realmURL.Hostname()
	if registryURL != nil && urlutil.SameHostname(realmURL, registryURL) {
		return nil
	}
	prefixes = resolveBlockedPrefixes(prefixes)
	if len(prefixes) == 0 {
		return nil
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if p, ok := blockedPrefix(addr, prefixes); ok {
			return fmt.Errorf("bearer realm host %q is %s, in blocked prefix %s", host, unwrapRealmAddr(addr), p)
		}
		return nil
	}
	addrs, err := lookupIPAddr(ctx, host)
	if err != nil {
		// Do not reject on lookup failure; see the function comment.
		return nil
	}
	for _, ipAddr := range addrs {
		addr, ok := netip.AddrFromSlice(ipAddr.IP)
		if !ok {
			continue
		}
		if p, ok := blockedPrefix(addr, prefixes); ok {
			return fmt.Errorf("bearer realm host %q resolves to %s, in blocked prefix %s", host, unwrapRealmAddr(addr), p)
		}
	}
	return nil
}

// realmRedirectGuard returns a shallow copy of client whose CheckRedirect
// applies validateRealm to every redirect hop of a token request, measured
// against registryURL, before delegating to the CheckRedirect of client. This
// keeps a validated realm from redirecting the token request into a blocked
// prefix or from https to http. prefixes is the same denylist used for the
// initial realm so redirect hops are evaluated consistently.
// Reference: https://github.com/oras-project/oras-go/security/advisories/GHSA-mm27-wq9c-mxxr
func realmRedirectGuard(client *http.Client, registryURL *url.URL, prefixes []netip.Prefix) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	clientCopy := *client
	checkRedirect := client.CheckRedirect
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := validateRealm(req.Context(), req.URL.String(), registryURL, prefixes); err != nil {
			return fmt.Errorf("redirect of token request rejected: %w", err)
		}
		if checkRedirect != nil {
			return checkRedirect(req, via)
		}
		if len(via) >= defaultMaxRedirects {
			return fmt.Errorf("stopped after %d redirects", defaultMaxRedirects)
		}
		return nil
	}
	return &clientCopy
}

// SetUserAgent sets the user agent for all out-going requests.
func (c *Client) SetUserAgent(userAgent string) {
	if c.Header == nil {
		c.Header = http.Header{}
	}
	c.Header.Set(headerUserAgent, userAgent)
}

// Do sends the request to the remote server, attempting to resolve
// authentication if 'Authorization' header is not set.
//
// A same-host redirect from the standard HTTP port to the standard HTTPS port
// can initiate authentication at the HTTPS endpoint. Other origin changes,
// repository changes, and HTTPS downgrades cannot initiate authentication there.
// Credentials obtained after an upgrade are not cached, to prevent their reuse
// on a later HTTP request.
//
// On authentication failure due to bad credential,
//   - Do returns error if it fails to fetch token for bearer auth.
//   - Do returns the registry response without error for basic auth.
func (c *Client) Do(originalReq *http.Request) (*http.Response, error) {
	if auth := originalReq.Header.Get(headerAuthorization); auth != "" {
		return c.send(originalReq)
	}

	ctx := originalReq.Context()
	req := originalReq.Clone(ctx)

	// attempt cached auth token
	var attemptedKey string
	var cachedBearer bool
	cache := c.cache()
	resource := requestResource(originalReq)
	// Credentials may be namespaced, so a token fetched for one repository
	// path must not be replayed for another path on the same host. The scope
	// hints are keyed by the same resource, so that the repositories of one
	// namespace are not disclosed on a token request made for another.
	cacheKey := resource.String()
	scheme, err := cache.GetScheme(ctx, cacheKey)
	if err == nil {
		switch scheme {
		case SchemeBasic:
			token, err := cache.GetToken(ctx, cacheKey, SchemeBasic, "")
			if err == nil {
				req.Header.Set(headerAuthorization, "Basic "+token)
			}
		case SchemeBearer:
			attemptedKey = bearerCacheKey(GetScopesForResource(ctx, resource))
			token, err := cache.GetToken(ctx, cacheKey, SchemeBearer, attemptedKey)
			if err == nil {
				req.Header.Set(headerAuthorization, "Bearer "+token)
				cachedBearer = true
			}
		}
	}

	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	// Some registries (notably Amazon ECR Public) reject an expired or
	// invalid bearer token with 400 DENIED and no WWW-Authenticate header
	// instead of 401. When the token came from the cache, resend the
	// request once without Authorization so the registry issues a fresh
	// 401 challenge.
	if cachedBearer && isInvalidTokenResponse(resp) {
		req = originalReq.Clone(ctx)
		if err := rewindRequestBody(req); err == nil {
			resp.Body.Close()
			resp, err = c.send(req)
			if err != nil {
				return nil, err
			}
		}
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}
	// Resolve credentials only for the original origin and repository, or a
	// same-host HTTPS upgrade. Authorization is still stripped on redirects.
	// Reference: https://github.com/oras-project/oras-go/security/advisories/GHSA-vh4v-2xq2-g5cg
	authReq := originalReq
	if resp.Request != nil {
		responseURL := resp.Request.URL
		if crossesRepository(originalReq.URL, responseURL) {
			return resp, nil
		}
		if !urlutil.SameOrigin(originalReq.URL, responseURL) {
			if !safeHTTPSUpgrade(originalReq, resp.Request) {
				return resp, nil
			}
			// Retry at the HTTPS endpoint without replaying credentials over HTTP.
			authReq = resp.Request.Clone(ctx)
			authReq.Header = originalReq.Header.Clone()
			if authReq.Method == originalReq.Method {
				authReq.GetBody = originalReq.GetBody
			}
			// Cache keys do not distinguish HTTP from HTTPS. Do not cache an
			// upgraded credential where a later HTTP request could reuse it.
			cache = noCache{}
		}
	}

	// attempt again with credentials for recognized schemes
	challenge := resp.Header.Get(headerWWWAuthenticate)
	scheme, params := parseChallenge(challenge)
	switch scheme {
	case SchemeBasic:
		resp.Body.Close()

		token, err := cache.Set(ctx, cacheKey, SchemeBasic, "", func(ctx context.Context) (string, error) {
			return c.fetchBasicAuth(ctx, resource)
		})
		if err != nil {
			return nil, fmt.Errorf("%s %q: %w", resp.Request.Method, resp.Request.URL, err)
		}

		req = authReq.Clone(ctx)
		req.Header.Set(headerAuthorization, "Basic "+token)
	case SchemeBearer:
		resp.Body.Close()

		scopes := GetScopesForResource(ctx, resource)
		if paramScope := params["scope"]; paramScope != "" {
			// merge hinted scopes with challenged scopes
			challenged, err := parseChallengeScopes(paramScope)
			if err != nil {
				return nil, fmt.Errorf("%s %q: bearer challenge from %s: %w", resp.Request.Method, resp.Request.URL, resource.Registry, err)
			}
			scopes = CleanScopes(append(scopes, challenged...))
		}
		key := bearerCacheKey(scopes)

		// attempt the cache again if there is a scope change
		if key != attemptedKey {
			if token, err := cache.GetToken(ctx, cacheKey, SchemeBearer, key); err == nil {
				req = authReq.Clone(ctx)
				req.Header.Set(headerAuthorization, "Bearer "+token)
				if err := rewindRequestBody(req); err != nil {
					return nil, err
				}

				resp, err := c.send(req)
				if err != nil {
					return nil, err
				}
				if resp.StatusCode != http.StatusUnauthorized && !isInvalidTokenResponse(resp) {
					return resp, nil
				}
				resp.Body.Close()
			}
		}

		// attempt with credentials
		realm := params["realm"]
		if err := validateRealm(ctx, realm, authReq.URL, c.BlockedRealmPrefixes); err != nil {
			return nil, fmt.Errorf("%s %q: %w", resp.Request.Method, resp.Request.URL, err)
		}
		service := params["service"]
		token, err := cache.Set(ctx, cacheKey, SchemeBearer, key, func(ctx context.Context) (string, error) {
			tokenCtx := withTokenBlockedPrefixes(withTokenRegistryURL(ctx, authReq.URL), c.BlockedRealmPrefixes)
			return c.fetchBearerToken(tokenCtx, resource, realm, service, scopes)
		})
		if err != nil {
			return nil, fmt.Errorf("%s %q: %w", resp.Request.Method, resp.Request.URL, err)
		}

		req = authReq.Clone(ctx)
		req.Header.Set(headerAuthorization, "Bearer "+token)
	default:
		return resp, nil
	}
	if err := rewindRequestBody(req); err != nil {
		return nil, err
	}

	return c.send(req)
}

// fetchBasicAuth fetches a basic auth token for the basic challenge.
func (c *Client) fetchBasicAuth(ctx context.Context, resource properties.Resource) (string, error) {
	cred, err := c.credential(ctx, resource)
	if err != nil {
		return "", fmt.Errorf("failed to resolve credential: %w", err)
	}
	if cred == credentials.EmptyCredential {
		return "", ErrBasicCredentialNotFound
	}
	if cred.Username == "" || cred.Password == "" {
		return "", errors.New("missing username or password for basic auth")
	}
	auth := cred.Username + ":" + cred.Password
	return base64.StdEncoding.EncodeToString([]byte(auth)), nil
}

// fetchBearerToken fetches an access token for the bearer challenge.
//
// The acquisition strategy is delegated to TokenFetcher. When none is
// configured, a CompositeTokenFetcher is used: anonymous access goes through
// the distribution spec token endpoint and credentialed access uses OAuth2.
func (c *Client) fetchBearerToken(ctx context.Context, resource properties.Resource, realm, service string, scopes []Scope) (string, error) {
	cred, err := c.credential(ctx, resource)
	if err != nil {
		return "", err
	}

	fetcher := c.TokenFetcher
	if fetcher == nil {
		composite := NewCompositeTokenFetcher(c.Client, c.Header, c.ClientID, false)
		composite.distributionRealms = &c.distributionRealms
		fetcher = composite
	}
	params := TokenParams{
		Resource: resource,
		Realm:    realm,
		Service:  service,
		Scopes:   scopes,
	}
	return fetcher.FetchToken(ctx, params, cred)
}

// parseChallengeScopes parses the space-separated scopes of the `scope`
// parameter of a WWW-Authenticate challenge.
//
// A scope outside the documented grammar is kept opaque by [ParseScope] and
// forwarded to the token server as it arrived, so a registry with its own scope
// form does not fail the request. Only a scope that cannot round trip through
// the wire form is an error — one carrying a tab, CR or LF, which the split on
// spaces above does not separate — and it fails the request naming the registry
// and quoting the offending scope, since the user cannot fix what they cannot
// see.
func parseChallengeScopes(paramScope string) ([]Scope, error) {
	var scopes []Scope
	for s := range strings.SplitSeq(paramScope, " ") {
		if s == "" {
			continue
		}
		scope, err := ParseScope(s)
		if err != nil {
			return nil, err
		}
		scopes = append(scopes, scope)
	}
	return scopes, nil
}

// bearerCacheKey returns the token cache key for scopes.
//
// The scopes are cleaned first, so that the key does not depend on their order
// or on duplication among them, and so that the key derivation cannot drift from
// the sort order of [CleanScopes].
func bearerCacheKey(scopes []Scope) string {
	return joinScopes(CleanScopes(scopes))
}

// rewindRequestBody tries to rewind the request body if exists.
func rewindRequestBody(req *http.Request) error {
	if req.Body == nil || req.Body == http.NoBody {
		return nil
	}
	if req.GetBody == nil {
		return fmt.Errorf("%s %q: request body is not rewindable", req.Method, req.URL)
	}
	body, err := req.GetBody()
	if err != nil {
		return fmt.Errorf("%s %q: failed to get request body: %w", req.Method, req.URL, err)
	}
	req.Body = body
	return nil
}

// isInvalidTokenResponse reports whether resp is a 400 Bad Request carrying a
// DENIED error code, which registries such as Amazon ECR Public return instead
// of a 401 challenge when a bearer token is invalid or expired.
//
// The response body is read up to maxResponseBytes and restored on resp.Body so
// that callers still see the original body when the response is returned.
func isInvalidTokenResponse(resp *http.Response) bool {
	if resp.StatusCode != http.StatusBadRequest || resp.Body == nil {
		return false
	}
	buf, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	resp.Body = struct {
		io.Reader
		io.Closer
	}{
		Reader: io.MultiReader(bytes.NewReader(buf), resp.Body),
		Closer: resp.Body,
	}
	if err != nil {
		return false
	}
	var body struct {
		Errors errcode.Errors `json:"errors"`
	}
	if err := json.Unmarshal(buf, &body); err != nil {
		return false
	}
	for _, e := range body.Errors {
		if e.Code == errcode.ErrorCodeDenied {
			return true
		}
	}
	return false
}
