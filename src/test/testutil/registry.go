// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package testutil

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory" // used for docker test registry
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func startInMemoryRegistry(ctx context.Context, t *testing.T, port int, certFile, keyFile string) (*registry.Registry, string) {
	t.Helper()
	config := &configuration.Configuration{}
	config.HTTP.Addr = fmt.Sprintf(":%d", port)
	config.Log.AccessLog.Disabled = true
	config.Log.Level = "error"
	logrus.SetOutput(io.Discard)
	config.HTTP.DrainTimeout = 10 * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	if certFile != "" {
		config.HTTP.TLS.Certificate = certFile
		config.HTTP.TLS.Key = keyFile
	}
	ref, err := registry.NewRegistry(ctx, config)
	require.NoError(t, err)
	//nolint:errcheck // ignore
	go ref.ListenAndServe()
	addr := fmt.Sprintf("localhost:%d", port)
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err != nil {
			return false
		}
		require.NoError(t, conn.Close())
		return true
	}, 5*time.Second, 10*time.Millisecond, "registry did not start in time")
	return ref, addr
}

// SetupInMemoryRegistryDynamic starts an in-memory registry on an auto-allocated port.
func SetupInMemoryRegistryDynamic(ctx context.Context, t *testing.T) string {
	t.Helper()
	port, err := helpers.GetAvailablePort()
	require.NoError(t, err)
	return SetupInMemoryRegistry(ctx, t, port)
}

// SetupInMemoryRegistry sets up an in-memory registry on localhost and returns the address.
func SetupInMemoryRegistry(ctx context.Context, t *testing.T, port int) string {
	t.Helper()
	_, addr := startInMemoryRegistry(ctx, t, port, "", "")
	return addr
}

// SetupInMemoryRegistryStoppable starts an in-memory registry on an auto-allocated port
// and returns its address plus a function to stop it, for tests that need to simulate
// the registry becoming completely unreachable.
func SetupInMemoryRegistryStoppable(ctx context.Context, t *testing.T) (string, func()) {
	t.Helper()
	port, err := helpers.GetAvailablePort()
	require.NoError(t, err)
	ref, addr := startInMemoryRegistry(ctx, t, port, "", "")
	stop := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ref.Shutdown(shutdownCtx) //nolint:errcheck // best-effort shutdown in test cleanup
	}
	return addr, stop
}

// SetupInMemoryRegistryTLSOnPort starts an in-memory registry with TLS enabled on
// the given port, using the given certificate/key files, and returns its address.
// Used by tests simulating a registry that migrates from plain HTTP to HTTPS on the
// exact same address (pair with SetupInMemoryRegistryStoppable and SelfSignedCert).
func SetupInMemoryRegistryTLSOnPort(ctx context.Context, t *testing.T, port int, certFile, keyFile string) string {
	t.Helper()
	_, addr := startInMemoryRegistry(ctx, t, port, certFile, keyFile)
	return addr
}

// SelfSignedCert generates a self-signed TLS certificate and key for host, writes
// them as PEM to files in a t.TempDir(), and returns their paths.
func SelfSignedCert(t *testing.T, host string) (certFile, keyFile string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{host}
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	dir := t.TempDir()
	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")

	certOut, err := os.Create(certFile)
	require.NoError(t, err)
	require.NoError(t, pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der}))
	require.NoError(t, certOut.Close())

	keyBytes, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyOut, err := os.Create(keyFile)
	require.NoError(t, err)
	require.NoError(t, pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}))
	require.NoError(t, keyOut.Close())

	return certFile, keyFile
}
