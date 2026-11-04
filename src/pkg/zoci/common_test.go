// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/oci"
	"github.com/zarf-dev/zarf/src/types"
)

func TestNewRemoteTransportNegotiation(t *testing.T) {
	tests := []struct {
		name        string
		newServer   func(http.Handler) *httptest.Server
		remoteOpts  types.RemoteOptions
		expectError bool
		expectHTTP  bool
	}{
		{
			name:      "uses HTTP for a plaintext registry",
			newServer: httptest.NewServer,
			remoteOpts: types.RemoteOptions{
				PlainHTTP: true,
			},
			expectHTTP: true,
		},
		{
			name:      "retains HTTPS for a self-signed registry when TLS verification is skipped",
			newServer: httptest.NewTLSServer,
			remoteOpts: types.RemoteOptions{
				PlainHTTP:             true,
				InsecureSkipTLSVerify: true,
			},
		},
		{
			name:      "rejects an untrusted HTTPS registry instead of downgrading",
			newServer: httptest.NewTLSServer,
			remoteOpts: types.RemoteOptions{
				PlainHTTP: true,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.newServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(server.Close)

			remote, err := NewRemote(context.Background(), "oci://"+strings.TrimPrefix(strings.TrimPrefix(server.URL, "https://"), "http://")+"/test:latest", oci.PlatformForArch("amd64"), RemoteClientOptions{
				RemoteOptions: tt.remoteOpts,
			})
			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectHTTP, remote.Repo().PlainHTTP)
		})
	}
}

func TestNewRemoteUsesCustomTransportForNegotiation(t *testing.T) {
	newServer := func(t *testing.T) *httptest.Server {
		t.Helper()
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		t.Cleanup(server.Close)
		return server
	}

	t.Run("trusted TLS configuration", func(t *testing.T) {
		server := newServer(t)
		transport, ok := server.Client().Transport.(*http.Transport)
		require.True(t, ok)

		remote, err := NewRemote(context.Background(), "oci://"+strings.TrimPrefix(server.URL, "https://")+"/test:latest", oci.PlatformForArch("amd64"), RemoteClientOptions{
			Transport: transport,
			RemoteOptions: types.RemoteOptions{
				PlainHTTP: true,
			},
		})
		require.NoError(t, err)
		require.False(t, remote.Repo().PlainHTTP)
	})

	t.Run("insecure TLS option", func(t *testing.T) {
		server := newServer(t)
		defaultTransport, ok := http.DefaultTransport.(*http.Transport)
		require.True(t, ok)
		transport := defaultTransport.Clone()

		remote, err := NewRemote(context.Background(), "oci://"+strings.TrimPrefix(server.URL, "https://")+"/test:latest", oci.PlatformForArch("amd64"), RemoteClientOptions{
			Transport: transport,
			RemoteOptions: types.RemoteOptions{
				PlainHTTP:             true,
				InsecureSkipTLSVerify: true,
			},
		})
		require.NoError(t, err)
		require.False(t, remote.Repo().PlainHTTP)
	})
}
