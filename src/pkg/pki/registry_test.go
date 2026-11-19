// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package pki_test

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/pki"
	"github.com/zarf-dev/zarf/src/types"
)

func TestRegistryTransport(t *testing.T) {
	serverPKI, clientPKI, err := pki.GenerateMTLSCerts("registry", []string{"localhost"}, "localhost", "client")
	require.NoError(t, err)
	serverCert, err := tls.X509KeyPair(serverPKI.Cert, serverPKI.Key)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(serverPKI.CA))
	dir := t.TempDir()
	opts := types.RegistryTLSOptions{
		CAFile: filepath.Join(dir, "ca.pem"), CertFile: filepath.Join(dir, "client.pem"), KeyFile: filepath.Join(dir, "client-key.pem"),
	}
	require.NoError(t, os.WriteFile(opts.CAFile, serverPKI.CA, 0o600))
	require.NoError(t, os.WriteFile(opts.CertFile, clientPKI.Cert, 0o600))
	require.NoError(t, os.WriteFile(opts.KeyFile, clientPKI.Key, 0o600))

	for _, tt := range []struct {
		name       string
		opts       types.RegistryTLSOptions
		clientAuth tls.ClientAuthType
		trusted    bool
		wantErr    bool
	}{
		{name: "mTLS with private CA", opts: opts, clientAuth: tls.RequireAndVerifyClientCert},
		{name: "mTLS with existing trust", opts: types.RegistryTLSOptions{CertFile: opts.CertFile, KeyFile: opts.KeyFile}, clientAuth: tls.RequireAndVerifyClientCert, trusted: true},
		{name: "private CA without client authentication", opts: types.RegistryTLSOptions{CAFile: opts.CAFile}},
		{name: "existing trust without certificate flags", trusted: true},
		{name: "missing client identity", opts: types.RegistryTLSOptions{CAFile: opts.CAFile}, clientAuth: tls.RequireAndVerifyClientCert, wantErr: true},
		{name: "untrusted server", opts: types.RegistryTLSOptions{CertFile: opts.CertFile, KeyFile: opts.KeyFile}, clientAuth: tls.RequireAndVerifyClientCert, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, err := io.WriteString(w, "authenticated")
				assert.NoError(t, err)
			}))
			server.TLS = &tls.Config{Certificates: []tls.Certificate{serverCert}, ClientCAs: roots, ClientAuth: tt.clientAuth, MinVersion: tls.VersionTLS12}
			server.StartTLS()
			t.Cleanup(server.Close)
			var base *http.Transport
			if tt.trusted {
				base = &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots}}
			}
			transport, err := pki.RegistryTransport(base, tt.opts)
			require.NoError(t, err)
			t.Cleanup(transport.CloseIdleConnections)
			resp, err := (&http.Client{Transport: transport}).Get(server.URL)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, "authenticated", string(body))
		})
	}
}

func TestRegistryTransportInvalidConfiguration(t *testing.T) {
	dir := t.TempDir()
	badCA := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(badCA, []byte("not a certificate"), 0o600))
	for _, opts := range []types.RegistryTLSOptions{
		{CertFile: "client.pem"},
		{KeyFile: "client-key.pem"},
		{CAFile: badCA},
		{CAFile: filepath.Join(dir, "missing.pem")},
		{CertFile: badCA, KeyFile: badCA},
	} {
		_, err := pki.RegistryTransport(nil, opts)
		require.Error(t, err)
	}
}
