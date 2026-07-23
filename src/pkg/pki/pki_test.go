// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package pki

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

func TestCheckForExpiredCert1(t *testing.T) {
	tests := []struct {
		name           string
		timeAtCreation time.Time
		certExpiration time.Time
		timeAtCheck    time.Time
		expectedLog    string
		expectedErr    string
	}{
		{
			name:           "20% left exactly -> no warning",
			timeAtCreation: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			timeAtCheck:    time.Date(2025, 1, 1, 0, 4, 0, 0, time.UTC),
			certExpiration: time.Date(2025, 1, 1, 0, 5, 0, 0, time.UTC),
		},
		{
			name:           "just under 20% -> warning",
			timeAtCreation: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			timeAtCheck:    time.Date(2025, 1, 1, 0, 4, 1, 0, time.UTC),
			certExpiration: time.Date(2025, 1, 1, 0, 5, 0, 0, time.UTC),
			expectedLog:    "the Zarf agent certificate is expiring soon, run `zarf tools update-creds agent` to update",
		},
		{
			name:           "already expired -> error",
			timeAtCreation: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			timeAtCheck:    time.Date(2025, 1, 1, 0, 5, 0, 0, time.UTC),
			certExpiration: time.Date(2025, 1, 1, 0, 4, 0, 0, time.UTC),
			expectedErr:    "the Zarf agent certificate is expired as of 2025-01-01 00:04:00 +0000 UTC, run `zarf tools update-creds agent` to update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup logger so we can capture logs
			buf := &bytes.Buffer{}
			cfg := logger.Config{
				Level:       logger.Info,
				Format:      logger.FormatConsole,
				Destination: buf,
			}
			l, err := logger.New(cfg)
			require.NoError(t, err)
			ctx := logger.WithContext(context.Background(), l)

			// Create cert with fixed time
			now = func() time.Time { return tt.timeAtCreation }
			pki, err := generatePKI("localhost", tt.certExpiration)
			require.NoError(t, err)

			// Check cert with fixed time
			now = func() time.Time { return tt.timeAtCheck }
			err = CheckForExpiredCert(ctx, pki)
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)

			if tt.expectedLog != "" {
				require.Contains(t, buf.String(), tt.expectedLog)
			}
		})
	}
}

func TestGeneratePKIWithOptions(t *testing.T) {
	originalNow := now
	defer func() { now = originalNow }()

	creationTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	now = func() time.Time { return creationTime }

	duration := 10 * 365 * 24 * time.Hour // ~10 years
	pki, err := GeneratePKIWithOptions("test.example.com", GenerateOptions{Duration: duration})
	require.NoError(t, err)
	require.NotEmpty(t, pki.CA)
	require.NotEmpty(t, pki.Cert)
	require.NotEmpty(t, pki.Key)

	cert, err := ParseCertFromPEM(pki.Cert)
	require.NoError(t, err)
	expectedExpiry := creationTime.Add(duration)
	require.Equal(t, expectedExpiry, cert.NotAfter)
}

func TestTransportWithKeyFollowsRedirectToSeparatelyTrustedEndpoint(t *testing.T) {
	originalNow := now
	now = time.Now
	t.Cleanup(func() { now = originalNow })

	registryServerPKI, registryClientPKI, err := GenerateMTLSCerts("registry CA", nil, "registry", "registry-client")
	require.NoError(t, err)

	externalPKI, err := GeneratePKI("external")
	require.NoError(t, err)

	externalCert, err := tls.X509KeyPair(externalPKI.Cert, externalPKI.Key)
	require.NoError(t, err)
	externalServer := newTLSServer(t, &tls.Config{Certificates: []tls.Certificate{externalCert}}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer externalServer.Close()

	registryCert, err := tls.X509KeyPair(registryServerPKI.Cert, registryServerPKI.Key)
	require.NoError(t, err)
	registryClientCAs := x509.NewCertPool()
	require.True(t, registryClientCAs.AppendCertsFromPEM(registryClientPKI.CA))
	registryServer := newTLSServer(t, &tls.Config{
		Certificates: []tls.Certificate{registryCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    registryClientCAs,
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, externalServer.URL, http.StatusTemporaryRedirect)
	}))
	defer registryServer.Close()

	externalRootCAs := x509.NewCertPool()
	require.True(t, externalRootCAs.AppendCertsFromPEM(externalPKI.CA))
	transport, err := transportWithKey(registryClientPKI, externalRootCAs)
	require.NoError(t, err)

	client := &http.Client{Transport: transport}
	response, err := client.Get(registryServer.URL)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, response.Body.Close())
	}()
	require.Equal(t, http.StatusNoContent, response.StatusCode)
}

func newTLSServer(t *testing.T, tlsConfig *tls.Config, handler http.Handler) *httptest.Server {
	t.Helper()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)

	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.TLS = tlsConfig
	server.StartTLS()
	return server
}

func TestGetRemainingCertLifePercentage(t *testing.T) {
	// Reset time function after tests
	originalNow := now
	defer func() {
		now = originalNow
	}()

	creationTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name               string
		certLifetime       time.Duration
		checkOffset        time.Duration
		expectedPercentage float64
	}{
		{
			name:               "certificate with 100% life remaining",
			certLifetime:       10 * 24 * time.Hour, // 10 days
			checkOffset:        0,                   // Check immediately
			expectedPercentage: 100.0,
		},
		{
			name:               "certificate with 50% life remaining",
			certLifetime:       10 * 24 * time.Hour, // 10 days
			checkOffset:        5 * 24 * time.Hour,  // 5 days later
			expectedPercentage: 50.0,
		},
		{
			name:               "certificate with 20% life remaining",
			certLifetime:       10 * 24 * time.Hour, // 10 days
			checkOffset:        8 * 24 * time.Hour,  // 8 days later
			expectedPercentage: 20.0,
		},
		{
			name:               "certificate with 25% life remaining (short lifetime)",
			certLifetime:       1 * time.Hour,    // 1 hour
			checkOffset:        45 * time.Minute, // 45 minutes later
			expectedPercentage: 25.0,
		},
		{
			name:               "expired certificate returns 0%",
			certLifetime:       10 * 24 * time.Hour, // 10 days
			checkOffset:        15 * 24 * time.Hour, // 15 days later (expired)
			expectedPercentage: 0.0,
		},
		{
			name:               "certificate exactly at expiration returns 0%",
			certLifetime:       10 * 24 * time.Hour, // 10 days
			checkOffset:        10 * 24 * time.Hour, // Exactly at expiration
			expectedPercentage: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate certificate
			expirationTime := creationTime.Add(tt.certLifetime)
			now = func() time.Time { return creationTime }
			pki, err := generatePKI("test.local", expirationTime)
			require.NoError(t, err)

			// Set check time
			checkTime := creationTime.Add(tt.checkOffset)
			now = func() time.Time { return checkTime }

			// Test the function
			percentage, err := GetRemainingCertLifePercentage(pki.Cert)
			require.NoError(t, err)
			require.InDelta(t, tt.expectedPercentage, percentage, 0.01)
		})
	}
}
