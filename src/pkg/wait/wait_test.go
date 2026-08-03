// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package wait provides functions for waiting on Kubernetes resources and network endpoints.
package wait

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/stretchr/testify/require"
)

func TestIsJSONPathWaitType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		waitType string
		expected bool
	}{
		{
			name:     "JSONPath with availableReplicas",
			waitType: "{.status.availableReplicas}=1",
			expected: true,
		},
		{
			name:     "JSONPath with container ready status",
			waitType: "{.status.containerStatuses[0].ready}=true",
			expected: true,
		},
		{
			name:     "JSONPath with container port",
			waitType: "{.spec.containers[0].ports[0].containerPort}=80",
			expected: true,
		},
		{
			name:     "JSONPath with nodeName",
			waitType: "{.spec.nodeName}=knode0",
			expected: true,
		},
		{
			name:     "condition type Ready",
			waitType: "Ready",
			expected: false,
		},
		{
			name:     "condition type delete",
			waitType: "delete",
			expected: false,
		},
		{
			name:     "empty string",
			waitType: "",
			expected: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isJSONPathWaitType(tt.waitType)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestProbeNetworkHTTP(t *testing.T) {
	t.Parallel()
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(successServer.Close)

	hangingServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(hangingServer.Close)

	notFoundServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(notFoundServer.Close)

	successServerURL := strings.TrimPrefix(successServer.URL, "http://")
	notFoundServerURL := strings.TrimPrefix(notFoundServer.URL, "http://")
	hangingServerURL := strings.TrimPrefix(hangingServer.URL, "http://")
	closedTCPAddress := closedLocalTCPAddress(t)

	tests := []struct {
		name      string
		host      string
		condition string
		wantOK    bool
		expectErr bool
	}{
		{
			name:      "success condition accepts 2xx",
			host:      successServerURL,
			condition: "success",
			wantOK:    true,
			expectErr: false,
		},
		{
			name:      "success condition rejects 404",
			host:      notFoundServerURL,
			condition: "success",
			wantOK:    false,
			expectErr: false,
		},
		{
			name:      "status code condition accepts matching code",
			host:      notFoundServerURL,
			condition: "404",
			wantOK:    true,
			expectErr: false,
		},
		{
			name:      "status code condition rejects non-matching code",
			host:      notFoundServerURL,
			condition: "200",
			wantOK:    false,
			expectErr: false,
		},
		{
			name:      "closed port returns error",
			host:      closedTCPAddress,
			condition: "success",
			wantOK:    false,
			expectErr: true,
		},
		{
			name:      "hanging server returns error",
			host:      hangingServerURL,
			condition: "success",
			wantOK:    false,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ok, err := probeNetwork(t.Context(), "http", tt.host, tt.condition, 100*time.Millisecond)
			if tt.expectErr {
				require.Error(t, err)
				require.False(t, ok)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantOK, ok)
		})
	}
}

func TestProbeNetworkTCP(t *testing.T) {
	t.Parallel()

	addr := startFakeTCPServer(t)
	ok, err := probeNetwork(t.Context(), "tcp", addr, "", 100*time.Millisecond)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = probeNetwork(t.Context(), "tcp", closedLocalTCPAddress(t), "", 100*time.Millisecond)
	require.Error(t, err)
	require.False(t, ok)
}

func TestWaitForNetwork(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		cancelContext  bool
		contextTimeout time.Duration
		probe          func(context.Context, string, string, string, time.Duration) (bool, error)
		timeout        time.Duration
		interval       time.Duration
		wantErr        string
		wantIs         error
		wantNotIs      []error
		attempts       int
	}{
		{
			name: "success on first probe",
			probe: func(context.Context, string, string, string, time.Duration) (bool, error) {
				return true, nil
			},
			timeout:  time.Second,
			interval: 10 * time.Millisecond,
			attempts: 1,
		},
		{
			name: "success after retry",
			probe: func() func(context.Context, string, string, string, time.Duration) (bool, error) {
				attempts := 0
				return func(context.Context, string, string, string, time.Duration) (bool, error) {
					attempts++
					return attempts == 2, nil
				}
			}(),
			timeout:  time.Second,
			interval: 10 * time.Millisecond,
			attempts: 2,
		},
		{
			name:          "context cancelled",
			cancelContext: true,
			probe:         neverReadyProbe,
			timeout:       time.Second,
			interval:      time.Second,
			wantErr:       "wait cancelled: context canceled",
			wantIs:        context.Canceled,
			wantNotIs:     []error{context.DeadlineExceeded},
		},
		{
			name:           "context deadline exceeded",
			contextTimeout: 100 * time.Millisecond,
			probe:          neverReadyProbe,
			timeout:        time.Second,
			interval:       time.Second,
			wantErr:        "wait cancelled: context deadline exceeded",
			wantIs:         context.DeadlineExceeded,
			wantNotIs:      []error{context.Canceled},
		},
		{
			name:      "internal timeout",
			probe:     neverReadyProbe,
			timeout:   100 * time.Millisecond,
			interval:  10 * time.Millisecond,
			wantErr:   "wait timed out",
			wantNotIs: []error{context.DeadlineExceeded, context.Canceled},
		},
		{
			name: "retryable probe errors are retried",
			probe: func() func(context.Context, string, string, string, time.Duration) (bool, error) {
				attempts := 0
				return func(context.Context, string, string, string, time.Duration) (bool, error) {
					attempts++
					if attempts < 2 {
						return false, errors.New("not ready")
					}
					return true, nil
				}
			}(),
			timeout:  time.Second,
			interval: 10 * time.Millisecond,
			attempts: 2,
		},
		{
			name: "unrecoverable probe errors return immediately",
			probe: func(context.Context, string, string, string, time.Duration) (bool, error) {
				return false, retry.Unrecoverable(errors.New("invalid condition"))
			},
			timeout:  time.Second,
			interval: 10 * time.Millisecond,
			wantErr:  "invalid condition",
			attempts: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			if tt.contextTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tt.contextTimeout)
				t.Cleanup(cancel)
			}
			if tt.cancelContext {
				cancelCtx, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelCtx
				t.Cleanup(cancel)
			}
			attempts := 0
			probe := func(ctx context.Context, protocol string, address string, condition string, waitInterval time.Duration) (bool, error) {
				attempts++
				return tt.probe(ctx, protocol, address, condition, waitInterval)
			}

			start := time.Now()
			err := waitForNetwork(ctx, "test", "unused", "", tt.timeout, tt.interval, probe)
			elapsed := time.Since(start)

			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErr)
			}
			if tt.wantIs != nil {
				require.ErrorIs(t, err, tt.wantIs)
			}
			for _, notIs := range tt.wantNotIs {
				require.NotErrorIs(t, err, notIs)
			}
			if tt.attempts > 0 {
				require.Equal(t, tt.attempts, attempts)
			}
			require.Less(t, elapsed, time.Second, "forNetwork should return promptly")
		})
	}
}

func startFakeTCPServer(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() }) //nolint:errcheck

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close() //nolint:errcheck
		}
	}()

	return ln.Addr().String()
}

func closedLocalTCPAddress(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())
	return addr
}

func neverReadyProbe(context.Context, string, string, string, time.Duration) (bool, error) {
	return false, nil
}
