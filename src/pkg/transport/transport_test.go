// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package transport

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func hostOf(t *testing.T, rawURL string) string {
	t.Helper()
	// httptest server URLs are already "http://127.0.0.1:PORT"; strip the scheme.
	host := strings.TrimPrefix(rawURL, "http://")
	host = strings.TrimPrefix(host, "https://")
	return host
}

func TestUsePlainHTTP_HTTPSSuccess(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := New(Options{})
	got, err := n.UsePlainHTTP(context.Background(), hostOf(t, srv.URL), ProbeOptions{InsecureSkipTLSVerify: true})
	require.NoError(t, err)
	require.False(t, got, "a reachable HTTPS endpoint must resolve to HTTPS")
}

func TestUsePlainHTTP_HTTPSNonSuccessStatusStillMeansHTTPS(t *testing.T) {
	// The core anti-downgrade-oracle assertion: any delivered response over a
	// completed TLS handshake proves the endpoint speaks HTTPS, regardless of status
	// code. Falling back on a 401/403/404/500 would make Zarf a downgrade oracle.
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			}))
			defer srv.Close()

			n := New(Options{})
			got, err := n.UsePlainHTTP(context.Background(), hostOf(t, srv.URL), ProbeOptions{InsecureSkipTLSVerify: true})
			require.NoError(t, err)
			require.False(t, got, "status %d over TLS must still resolve to HTTPS, never downgrade", status)
		})
	}
}

func TestUsePlainHTTP_PlaintextOnTLSPortFallsBackToHTTP(t *testing.T) {
	var httpRequests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httpRequests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := New(Options{})
	got, err := n.UsePlainHTTP(context.Background(), hostOf(t, srv.URL), ProbeOptions{})
	require.NoError(t, err)
	require.True(t, got, "a plain-HTTP server on the probed port must resolve to plain HTTP")
	require.Equal(t, int32(1), httpRequests.Load())
}

func TestUsePlainHTTP_ConnectionRefusedDoesNotFallBack(t *testing.T) {
	// Bind and immediately close a listener to get a port nothing is listening on.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	host := l.Addr().String()
	require.NoError(t, l.Close())

	n := New(Options{})
	got, err := n.UsePlainHTTP(context.Background(), host, ProbeOptions{})
	require.Error(t, err)
	require.False(t, got)
	require.Contains(t, err.Error(), "refusing to downgrade")
}

func TestUsePlainHTTP_TimeoutDoesNotFallBack(t *testing.T) {
	// A server that accepts the connection but never responds within the probe
	// timeout must not be treated as "try plain HTTP" — a timeout proves nothing
	// about which scheme is correct.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close() //nolint:errcheck

	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			// Accept the connection and never write anything back, until the test ends.
			go func() {
				<-done
				conn.Close() //nolint:errcheck
			}()
		}
	}()

	n := New(Options{})
	ctx, cancel := context.WithTimeout(context.Background(), 2*probeTimeout)
	defer cancel()
	got, err := n.UsePlainHTTP(ctx, l.Addr().String(), ProbeOptions{})
	require.Error(t, err)
	require.False(t, got)
	require.Contains(t, err.Error(), "refusing to downgrade")
}

func TestUsePlainHTTP_BothSchemesFail(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	host := l.Addr().String()
	require.NoError(t, l.Close())

	n := New(Options{})
	_, err = n.UsePlainHTTP(context.Background(), host, ProbeOptions{})
	require.Error(t, err)
}

func TestUsePlainHTTP_InsecureSkipTLSVerify(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	host := hostOf(t, srv.URL)

	t.Run("without skip verify, a self-signed cert fails and does not downgrade", func(t *testing.T) {
		n := New(Options{})
		got, err := n.UsePlainHTTP(context.Background(), host, ProbeOptions{InsecureSkipTLSVerify: false})
		require.Error(t, err)
		require.False(t, got)
		require.Contains(t, err.Error(), "refusing to downgrade")
	})

	t.Run("with skip verify, the self-signed cert is accepted and resolves to HTTPS", func(t *testing.T) {
		n := New(Options{})
		got, err := n.UsePlainHTTP(context.Background(), host, ProbeOptions{InsecureSkipTLSVerify: true})
		require.NoError(t, err)
		require.False(t, got)
	})
}

func TestUsePlainHTTP_CachesAndDedupesConcurrentCalls(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	host := hostOf(t, srv.URL)

	n := New(Options{})
	const goroutines = 50
	results := make(chan bool, goroutines)
	errs := make(chan error, goroutines)
	for range goroutines {
		go func() {
			got, err := n.UsePlainHTTP(context.Background(), host, ProbeOptions{InsecureSkipTLSVerify: true})
			results <- got
			errs <- err
		}()
	}
	for range goroutines {
		require.NoError(t, <-errs)
		require.False(t, <-results)
	}
	require.Equal(t, int32(1), requests.Load(), "concurrent negotiation of the same host must collapse into one probe")

	// A subsequent, sequential call should also hit the cache, not the server.
	_, err := n.UsePlainHTTP(context.Background(), host, ProbeOptions{InsecureSkipTLSVerify: true})
	require.NoError(t, err)
	require.Equal(t, int32(1), requests.Load(), "a cached decision must not re-probe")
}

func TestUsePlainHTTP_TTLExpiryReProbes(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	host := hostOf(t, srv.URL)

	n := New(Options{TTL: time.Minute})
	fakeNow := time.Now()
	n.now = func() time.Time { return fakeNow }

	_, err := n.UsePlainHTTP(context.Background(), host, ProbeOptions{InsecureSkipTLSVerify: true})
	require.NoError(t, err)
	require.Equal(t, int32(1), requests.Load())

	// Still within TTL: cached.
	fakeNow = fakeNow.Add(30 * time.Second)
	_, err = n.UsePlainHTTP(context.Background(), host, ProbeOptions{InsecureSkipTLSVerify: true})
	require.NoError(t, err)
	require.Equal(t, int32(1), requests.Load())

	// Past TTL: re-probes.
	fakeNow = fakeNow.Add(2 * time.Minute)
	_, err = n.UsePlainHTTP(context.Background(), host, ProbeOptions{InsecureSkipTLSVerify: true})
	require.NoError(t, err)
	require.Equal(t, int32(2), requests.Load())
}

func TestUsePlainHTTP_NegativeCacheDoesNotReprobeWithinTTL(t *testing.T) {
	var accepts atomic.Int32
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close() //nolint:errcheck

	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			accepts.Add(1)
			conn.Close() //nolint:errcheck
		}
	}()

	n := New(Options{})
	fakeNow := time.Now()
	n.now = func() time.Time { return fakeNow }

	_, err = n.UsePlainHTTP(context.Background(), l.Addr().String(), ProbeOptions{})
	require.Error(t, err)
	first := accepts.Load()
	require.GreaterOrEqual(t, first, int32(1))

	// Still within negativeCacheTTL: the cached failure is returned without probing again.
	_, err = n.UsePlainHTTP(context.Background(), l.Addr().String(), ProbeOptions{})
	require.Error(t, err)
	require.Equal(t, first, accepts.Load(), "a cached failure must not re-probe within negativeCacheTTL")

	// Past negativeCacheTTL: re-probes.
	fakeNow = fakeNow.Add(negativeCacheTTL + time.Second)
	_, err = n.UsePlainHTTP(context.Background(), l.Addr().String(), ProbeOptions{})
	require.Error(t, err)
	require.Greater(t, accepts.Load(), first, "past negativeCacheTTL, the next call must re-probe")
}

func TestUsePlainHTTP_ProbeContextDecoupledFromCaller(t *testing.T) {
	entered := make(chan struct{})
	unblock := make(chan struct{})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-unblock
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	host := hostOf(t, srv.URL)

	n := New(Options{})

	aCtx, aCancel := context.WithCancel(context.Background())
	aErr := make(chan error, 1)
	bErr := make(chan error, 1)

	go func() {
		_, err := n.UsePlainHTTP(aCtx, host, ProbeOptions{InsecureSkipTLSVerify: true})
		aErr <- err
	}()
	<-entered // A's probe request has reached the server and is now blocked there.

	go func() {
		_, err := n.UsePlainHTTP(context.Background(), host, ProbeOptions{InsecureSkipTLSVerify: true})
		bErr <- err
	}()
	time.Sleep(50 * time.Millisecond) // give B time to join A's in-flight singleflight call

	aCancel()
	close(unblock)

	require.NoError(t, <-aErr, "the shared probe must not fail even for the caller whose context was canceled")
	require.NoError(t, <-bErr, "a concurrent caller's healthy context must not be affected by another caller's cancellation")
}

func TestNegotiator_Invalidate(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	host := hostOf(t, srv.URL)

	n := New(Options{})
	_, err := n.UsePlainHTTP(context.Background(), host, ProbeOptions{InsecureSkipTLSVerify: true})
	require.NoError(t, err)
	require.Equal(t, int32(1), requests.Load())

	n.Invalidate(host)

	_, err = n.UsePlainHTTP(context.Background(), host, ProbeOptions{InsecureSkipTLSVerify: true})
	require.NoError(t, err)
	require.Equal(t, int32(2), requests.Load(), "invalidating a host must force a re-probe")
}

func TestWithNegotiatorAndFrom(t *testing.T) {
	t.Run("returns a usable negotiator when none is installed", func(t *testing.T) {
		n := From(context.Background())
		require.NotNil(t, n)
	})

	t.Run("falls back to a shared singleton, not a fresh instance per call", func(t *testing.T) {
		// This is the behavior an SDK consumer relies on: code that calls Zarf's
		// packages directly, without going through the zarf CLI's root command
		// (which is what installs a per-invocation Negotiator into the context),
		// still gets real caching across its own calls instead of a fresh,
		// empty-cache Negotiator every time.
		first := From(context.Background())
		second := From(context.Background())
		require.Same(t, first, second)
	})

	t.Run("round-trips the installed negotiator", func(t *testing.T) {
		want := New(Options{})
		ctx := WithNegotiator(context.Background(), want)
		require.Same(t, want, From(ctx))
	})
}
