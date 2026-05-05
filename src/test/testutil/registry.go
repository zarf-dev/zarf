// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package testutil

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory" // used for docker test registry
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
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
