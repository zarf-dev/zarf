// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package testutil

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory" // used for docker test registry
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/pki"
)

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
	config := &configuration.Configuration{}
	config.HTTP.Addr = fmt.Sprintf(":%d", port)
	config.Log.AccessLog.Disabled = true
	config.Log.Level = "error"
	logrus.SetOutput(io.Discard)
	config.HTTP.DrainTimeout = 10 * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
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
	return addr
}

type registryCredentials struct {
	username string
	password string
}

var testRegistryAuth = struct {
	sync.RWMutex
	credentials map[string]registryCredentials
}{
	credentials: map[string]registryCredentials{},
}

var registerTestRegistryAuthMiddleware sync.Once

// SetupInMemoryRegistryTLSAuth starts a TLS-enabled, basic-authenticated in-memory registry and returns its address.
func SetupInMemoryRegistryTLSAuth(ctx context.Context, t *testing.T, username, password string) string {
	t.Helper()

	registerTestRegistryAuthMiddleware.Do(func() {
		registry.RegisterHandler(func(_ *configuration.Configuration, next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				testRegistryAuth.RLock()
				credentials, configured := testRegistryAuth.credentials[r.Host]
				testRegistryAuth.RUnlock()

				if configured {
					requestUsername, requestPassword, ok := r.BasicAuth()
					if !ok || requestUsername != credentials.username || requestPassword != credentials.password {
						w.Header().Set("WWW-Authenticate", `Basic realm="test registry"`)
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
				}

				next.ServeHTTP(w, r)
			})
		})
	})

	port, err := helpers.GetAvailablePort()
	require.NoError(t, err)
	address := fmt.Sprintf("localhost:%d", port)

	certs, err := pki.GeneratePKI("localhost")
	require.NoError(t, err)
	certPath := filepath.Join(t.TempDir(), "registry.crt")
	keyPath := filepath.Join(t.TempDir(), "registry.key")
	require.NoError(t, os.WriteFile(certPath, certs.Cert, 0o600))
	require.NoError(t, os.WriteFile(keyPath, certs.Key, 0o600))

	testRegistryAuth.Lock()
	testRegistryAuth.credentials[address] = registryCredentials{username: username, password: password}
	testRegistryAuth.Unlock()
	t.Cleanup(func() {
		testRegistryAuth.Lock()
		delete(testRegistryAuth.credentials, address)
		testRegistryAuth.Unlock()
	})

	config := &configuration.Configuration{}
	config.HTTP.Addr = fmt.Sprintf(":%d", port)
	config.HTTP.TLS.Certificate = certPath
	config.HTTP.TLS.Key = keyPath
	config.HTTP.HTTP2.Disabled = true
	config.Log.AccessLog.Disabled = true
	config.Log.Level = "error"
	config.HTTP.DrainTimeout = 10 * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	logrus.SetOutput(io.Discard)

	reg, err := registry.NewRegistry(ctx, config)
	require.NoError(t, err)
	//nolint:errcheck // ignore
	go reg.ListenAndServe()
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", address, 50*time.Millisecond)
		if err != nil {
			return false
		}
		require.NoError(t, conn.Close())
		return true
	}, 5*time.Second, 10*time.Millisecond, "registry did not start in time")

	return address
}
