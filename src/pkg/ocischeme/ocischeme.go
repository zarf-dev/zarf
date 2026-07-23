// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package ocischeme decides whether an OCI registry should be reached over plain
// HTTP or HTTPS.
//
// Zarf's --plain-http flag is a global, process-wide setting, but many registries
// Zarf talks to during a single command are not the registry the user meant that
// flag for — a third-party Helm chart URL, a container image reference, or a chart's
// own OCI dependency, all discovered by reading package data rather than named
// directly on the command line. Forcing the global flag onto those registries is
// wrong in both directions: it can force plain HTTP onto a registry that only speaks
// HTTPS (breaking the fetch), or leave a registry that only speaks plain HTTP
// unreachable because the flag wasn't set for an unrelated reason.
//
// This package answers the question per host instead: it probes the host directly
// and only falls back to plain HTTP on definitive proof — either the same port
// answers plaintext HTTP underneath a failed TLS handshake, or (only for a bare
// hostname with no explicit port, where HTTPS and HTTP resolve to different default
// ports) a real HTTP response on the conventional HTTP port once the HTTPS default
// port has proven completely unreachable. It never falls back on a TLS certificate
// error or an ordinary non-2xx status code — those prove something is listening and
// speaking a protocol, which is not evidence the correct scheme is different.
package ocischeme

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// probeTimeout bounds a single scheme-probe request using the package's default
// transport, which makes one attempt with no retries so decide can reliably detect
// http.ErrSchemeMismatch. A caller-supplied ProbeOptions.Transport that retries
// internally is only bounded by probeTimeout in aggregate, not per attempt.
const probeTimeout = 5 * time.Second

// negativeCacheTTL bounds how long a failed negotiation is cached before the next
// call re-probes. This is separate from, and always shorter than, Options.TTL: even
// a CLI invocation (TTL 0, decisions otherwise never expire) must not hammer an
// unreachable host with a fresh probeTimeout on every single call made during one
// command.
const negativeCacheTTL = 30 * time.Second

// ProbeOptions configures a single scheme-negotiation probe.
type ProbeOptions struct {
	// InsecureSkipTLSVerify disables TLS certificate verification during the HTTPS
	// probe. A certificate error is never, by itself, a reason to fall back to plain
	// HTTP; this only affects whether the HTTPS probe accepts a self-signed/invalid
	// certificate as a successful connection. Ignored if Transport is set.
	InsecureSkipTLSVerify bool
	// Transport, when set, is used for the probe instead of the package's default
	// transport. Provide this when reaching the host requires something a generic
	// transport can't do, like presenting a client certificate to an mTLS-secured
	// registry. If it retries internally, probing is slower and less precise than
	// the package's no-retry default; see probeTimeout.
	Transport http.RoundTripper
}

// Options configures a Negotiator.
type Options struct {
	// TTL controls how long a negotiated decision is cached before it is re-probed.
	// Zero means decisions never expire, which is correct for a Zarf CLI invocation
	// (a short-lived process) but not for a long-running process such as the Zarf
	// Agent admission webhook, which should set a positive TTL.
	TTL time.Duration
}

type cacheEntry struct {
	plainHTTP bool
	// err is non-nil for a cached failure, which expires after negativeCacheTTL
	// instead of the Negotiator's configured TTL; see lookup.
	err error
	at  time.Time
}

// Negotiator decides, per host, whether Zarf should speak plain HTTP or HTTPS to a
// registry reference that was discovered by reading package data rather than named
// explicitly on the command line. Decisions are cached per host, and concurrent
// callers negotiating the same host share a single in-flight probe.
//
// A Negotiator is safe for concurrent use.
type Negotiator struct {
	group singleflight.Group

	mu    sync.RWMutex
	cache map[string]cacheEntry
	ttl   time.Duration
	now   func() time.Time
}

// New creates a Negotiator.
func New(o Options) *Negotiator {
	return &Negotiator{
		cache: make(map[string]cacheEntry),
		ttl:   o.TTL,
		now:   time.Now,
	}
}

// UsePlainHTTP returns true if host should be reached over plain HTTP rather than
// HTTPS, deciding by probing the host directly (see decide). A cached decision is
// reused until it expires; see Options.TTL. Failures are cached too, but only for
// negativeCacheTTL.
func (n *Negotiator) UsePlainHTTP(ctx context.Context, host string, opts ProbeOptions) (bool, error) {
	if v, ok, cachedErr := n.lookup(host); ok {
		return v, cachedErr
	}

	v, err, _ := n.group.Do(host, func() (any, error) {
		// Re-check under the singleflight in case a concurrent call just populated it.
		if v, ok, cachedErr := n.lookup(host); ok {
			return v, cachedErr
		}
		// Decoupled from ctx: this probe is shared across every concurrent caller
		// negotiating host, so one caller's cancellation must not fail it for the
		// others. probeTimeout still bounds it.
		plainHTTP, decideErr := decide(context.WithoutCancel(ctx), host, opts)
		n.mu.Lock()
		n.cache[host] = cacheEntry{plainHTTP: plainHTTP, err: decideErr, at: n.now()}
		n.mu.Unlock()
		return plainHTTP, decideErr
	})
	if err != nil {
		return false, err
	}
	plainHTTP, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("unexpected type %T from negotiation for %q", v, host)
	}
	return plainHTTP, nil
}

func (n *Negotiator) lookup(host string) (plainHTTP bool, ok bool, cachedErr error) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	entry, found := n.cache[host]
	if !found {
		return false, false, nil
	}
	ttl := n.ttl
	if entry.err != nil {
		ttl = negativeCacheTTL
	}
	if ttl > 0 && n.now().Sub(entry.at) > ttl {
		return false, false, nil
	}
	return entry.plainHTTP, true, entry.err
}

// Invalidate drops any cached decision for host, forcing the next UsePlainHTTP call
// to re-probe.
//
// Callers should only invalidate in response to a transport-level failure that
// plausibly means the cached scheme is now wrong — e.g. a TLS handshake failure when
// a request was sent over HTTPS, or a connection reset/refused when a request was
// sent over the cached plain-HTTP scheme.
func (n *Negotiator) Invalidate(host string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.cache, host)
}

// decide performs the actual negotiation: try HTTPS first, and only fall back to
// plain HTTP when the HTTPS attempt fails with definitive proof that the endpoint
// speaks plaintext HTTP on that port.
func decide(ctx context.Context, host string, opts ProbeOptions) (bool, error) {
	httpsErr := probe(ctx, "https", host, opts)
	if httpsErr == nil {
		return false, nil
	}

	// net/http itself special-cases the scenario where a TLS ClientHello is sent to a
	// server speaking plaintext HTTP on that port: crypto/tls first surfaces this as a
	// tls.RecordHeaderError (the server's plaintext response line, "HTTP/1.1 ...", is
	// misread as a garbled TLS record whose first 5 bytes spell "HTTP/"), and
	// net/http's Client normalizes that into the exported http.ErrSchemeMismatch
	// sentinel.
	if errors.Is(httpsErr, http.ErrSchemeMismatch) {
		httpErr := probe(ctx, "http", host, opts)
		if httpErr != nil {
			return false, errors.Join(httpsErr, httpErr)
		}
		return true, nil
	}

	// A bare hostname with no explicit port defaults to a different port per scheme
	// (443 vs 80), so a plaintext responder on the conventional HTTP port can never
	// produce the same-port proof above. If the HTTPS attempt didn't get far enough to
	// even prove something is listening — no TCP connection, not a certificate error —
	// a real HTTP response on the conventional HTTP port counts as proof instead.
	// decide is only ever reached when the caller already opted into plain HTTP for
	// this negotiation, so this doesn't downgrade anything silently.
	if !hasExplicitPort(host) && isConnectionFailure(httpsErr) {
		if httpErr := probe(ctx, "http", host, opts); httpErr == nil {
			return true, nil
		}
	}

	return false, fmt.Errorf("registry %q did not respond over HTTPS and did not present definitive proof of a plain HTTP endpoint; refusing to downgrade to plain HTTP: %w", host, httpsErr)
}

// hasExplicitPort reports whether host includes an explicit port (e.g. "host:5000"
// or "[::1]:5000"), as opposed to a bare hostname or IP that resolves to a
// scheme-dependent default port.
func hasExplicitPort(host string) bool {
	_, _, err := net.SplitHostPort(host)
	return err == nil
}

// isConnectionFailure reports whether err means the probe never got far enough to
// prove anything is listening — connection refused, network/host unreachable, DNS
// failure, or the probe's own timeout — as opposed to a TLS or certificate error,
// which proves something is listening and speaking a protocol on that port.
func isConnectionFailure(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return opErr.Op == "dial"
	}
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr) || errors.Is(err, context.DeadlineExceeded)
}

// probe sends an anonymous GET to <scheme>://<host>/v2/. Any completed HTTP response
// counts as proof the scheme is correct, regardless of status code: a 401 or 403
// over TLS still proves the endpoint speaks HTTPS, so there is no need to
// authenticate merely to determine transport. Redirects are not followed, for the
// same reason: a redirect response is already proof enough, and following one risks
// judging a different host's scheme instead of host's.
func probe(ctx context.Context, scheme, host string, opts ProbeOptions) error {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	url := fmt.Sprintf("%s://%s/v2/", scheme, host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	rt := opts.Transport
	if rt == nil {
		baseTransport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return errors.New("could not get default transport")
		}
		baseTransport = baseTransport.Clone()
		baseTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: opts.InsecureSkipTLSVerify} //nolint:gosec // explicit, narrowly-scoped opt-in via ProbeOptions
		rt = baseTransport
	}

	client := &http.Client{
		Transport: rt,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// ctxKey provides a location to store a Negotiator in a context.
type ctxKey struct{}

var defaultCtxKey = ctxKey{}

// WithNegotiator returns a copy of ctx carrying n, retrievable with From.
func WithNegotiator(ctx context.Context, n *Negotiator) context.Context {
	return context.WithValue(ctx, defaultCtxKey, n)
}

// defaultNegotiator backs From when no Negotiator has been installed in the
// context — notably, any SDK consumer of Zarf's packages that never runs through
// the zarf CLI's root command (which is what installs a per-invocation Negotiator;
// see cmd/root.go) never populates the context key at all. A shared, lazily built
// singleton means that caller still gets real caching and singleflight dedup across
// its own calls, rather than a fresh, empty-cache Negotiator every time.
var defaultNegotiator = sync.OnceValue(func() *Negotiator {
	return New(Options{TTL: 5 * time.Minute})
})

// From returns the Negotiator carried by ctx. If none is present — e.g. because ctx
// was never passed through the zarf CLI's root command, or in a unit test — it
// returns a shared default Negotiator (see defaultNegotiator) so callers still get
// caching, just not scoped to a single command invocation.
func From(ctx context.Context) *Negotiator {
	if ctx != nil {
		if n, ok := ctx.Value(defaultCtxKey).(*Negotiator); ok && n != nil {
			return n
		}
	}
	return defaultNegotiator()
}
