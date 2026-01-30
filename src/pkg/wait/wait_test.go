// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package wait provides functions for waiting on Kubernetes resources and network endpoints.
package wait

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isJSONPathWaitType(tt.waitType)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestForNetwork(t *testing.T) {
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

	tests := []struct {
		name      string
		host      string
		condition string
		timeout   time.Duration
		interval  time.Duration
		expectErr bool
	}{
		{
			name:      "Wait for success, get success",
			host:      successServerURL,
			condition: "success",
			timeout:   time.Millisecond * 500,
			interval:  time.Millisecond * 10,
			expectErr: false,
		},
		{
			name:      "Wait for success, get not found",
			host:      notFoundServerURL,
			condition: "success",
			timeout:   time.Millisecond * 500,
			interval:  time.Millisecond * 10,
			expectErr: true,
		},
		{
			name:      "Wait for not found, get not found",
			host:      notFoundServerURL,
			condition: "404",
			timeout:   time.Millisecond * 500,
			interval:  time.Millisecond * 10,
			expectErr: false,
		},
		{
			name:      "Wait for success, non-existent server",
			host:      "localhost:1",
			condition: "success",
			timeout:   time.Millisecond * 500,
			interval:  time.Millisecond * 10,
			expectErr: true,
		},
		{
			name:      "Wait for success, hanging server should timeout not hang",
			host:      hangingServerURL,
			condition: "success",
			timeout:   time.Millisecond * 500,
			interval:  time.Millisecond * 100,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := forNetwork(t.Context(), "http", tt.host, tt.condition, tt.timeout, tt.interval)
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
