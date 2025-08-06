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
	return fmt.Sprintf("localhost:%d", port)
}
