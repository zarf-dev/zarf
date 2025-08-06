// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package testutil

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory" // used for docker test registry
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// SetupInMemoryRegistry sets up an in-memory registry on localhost and returns the address.
func SetupInMemoryRegistry(ctx context.Context, t *testing.T, port int) string {
	return SetupInMemoryRegistryWithAuth(ctx, t, port, "")
}

// SetupInMemoryRegistryWithAuth sets up an in-memory registry on localhost and returns the address.
// If the parameter `htpassword` is not empty, the registry will use that as the auth for accessing it.
func SetupInMemoryRegistryWithAuth(ctx context.Context, t *testing.T, port int, htpassword string) string {
	t.Helper()
	config := &configuration.Configuration{}
	config.HTTP.Addr = fmt.Sprintf(":%d", port)
	config.Log.AccessLog.Disabled = true
	config.Log.Level = "error"
	logrus.SetOutput(io.Discard)
	config.HTTP.DrainTimeout = 10 * time.Second
	if htpassword != "" {
		config.HTTP.Secret = htpassword
	}
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}
	ref, err := registry.NewRegistry(ctx, config)
	require.NoError(t, err)
	//nolint:errcheck // ignore
	go ref.ListenAndServe()
	return fmt.Sprintf("localhost:%d", port)
}
