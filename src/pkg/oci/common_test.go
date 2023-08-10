// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	"github.com/stretchr/testify/require"

	// _ "github.com/distribution/distribution/v3/registry/auth/htpasswd"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory"
)

func setup(t *testing.T, port int) (*OrasRemote, *registry.Registry) {
	ctx := context.TODO()

	config := &configuration.Configuration{}
	config.HTTP.Addr = fmt.Sprintf(":%d", port)
	config.HTTP.DrainTimeout = time.Duration(10) * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}

	reg, err := registry.NewRegistry(ctx, config)
	require.NoError(t, err)

	url := fmt.Sprintf("oci://localhost:%d", port)
	remote, err := NewOrasRemote(url)
	require.NoError(t, err)

	return remote, reg
}

func Test_NewOrasRemote(t *testing.T) {
	// this is purposefully a basic test, as this functionality is
	// extensively tested in registry.ParseReference

	// should error with non-existent repository
	_, err := NewOrasRemote("oci://localhost:555")
	require.Error(t, err)

	// should error with a bad reference
	_, err = NewOrasRemote("oci://localhost:555/foo:bar/baz")
	require.Error(t, err)

	// should not error with a valid reference that does not exist
	remote, err := NewOrasRemote("oci://localhost:555/foo")
	require.NoError(t, err)

	todo := context.TODO()
	withCancel, cancel := context.WithCancel(todo)
	defer cancel()
	remote.WithContext(withCancel)
	require.Equal(t, withCancel, remote.ctx)
}
