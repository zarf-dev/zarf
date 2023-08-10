// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"testing"

	"github.com/defenseunicorns/zarf/src/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
)

func TestOCIUtils(t *testing.T) {
	t.Run("ReferenceFromMetadata", testReferenceFromMetadata)
	t.Run("IsEmptyDescriptor", testIsEmptyDescriptor)
}

func testReferenceFromMetadata(t *testing.T) {
	reg := "registry.example.com"
	suffix := "test"
	meta := types.ZarfMetadata{
		Name:    "test",
		Version: "1.0.0",
	}
	ref, err := ReferenceFromMetadata(reg, &meta, suffix)
	require.NoError(t, err)

	expected := "registry.example.com/test:1.0.0-test"
	require.Equal(t, expected, ref)

	reg = "oci://registry.example.com/"
	ref, err = ReferenceFromMetadata(reg, &meta, suffix)
	require.NoError(t, err)
	require.Equal(t, expected, ref)
}

func testIsEmptyDescriptor(t *testing.T) {
	good := ocispec.Descriptor{
		Size: 1,
	}
	require.False(t, IsEmptyDescriptor(good))

	bad := ocispec.Descriptor{}
	require.True(t, IsEmptyDescriptor(bad))
}
