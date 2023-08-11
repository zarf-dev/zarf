// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var rootManifest = ZarfOCIManifest{}

func TestFetch(t *testing.T) {
	remote, reg, shutdown := setup(t, 555)
	go reg.ListenAndServe()
	defer shutdown()

	root, err := remote.FetchRoot()
	require.NoError(t, err)
	require.NotNil(t, root)
	// TODO: test the contents of the root descriptor

	manifest, err := remote.FetchRoot()
	require.NoError(t, err)
	require.Equal(t, rootManifest, manifest)

	desc, err := remote.ResolveRoot()
	require.NoError(t, err)
	manifest, err = remote.FetchManifest(desc)
	require.NoError(t, err)
	require.Equal(t, rootManifest, manifest)

	// TODO: finish me
}
