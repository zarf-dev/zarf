// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/defenseunicorns/pkg/oci"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/types"
)

func TestNewRemoteDeprecatedModifiers(t *testing.T) {
	remote, err := NewRemote(context.Background(), "example.com/example:latest", oci.PlatformForArch("amd64"), oci.WithPlainHTTP(true))
	require.NoError(t, err)
	require.True(t, remote.Repo().PlainHTTP)
}

func TestNewRemoteWithOptionsTransportNegotiation(t *testing.T) {
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

			remote, err := NewRemoteWithOptions(context.Background(), "oci://"+strings.TrimPrefix(strings.TrimPrefix(server.URL, "https://"), "http://")+"/test:latest", oci.PlatformForArch("amd64"), RemoteClientOptions{
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
