// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry"
)

func TestParseZarfPackageReference(t *testing.T) {
	cases := map[string]ZarfPackageReference{
		"oci://localhost:5000/pkg/0:0.0.1-amd64": {
			Reference: registry.Reference{
				Registry:   "localhost:5000",
				Repository: "pkg/0",
				Reference:  "0.0.1-amd64",
			},
			Arch:        "amd64",
			Version:     "0.0.1",
			PackageName: "0",
		},
		"oci://localhost:5000/pkg/0:0.0.1-skeleton": {
			Reference: registry.Reference{
				Registry:   "localhost:5000",
				Repository: "pkg/0",
				Reference:  "0.0.1-skeleton",
			},
			Arch:        "skeleton",
			Version:     "0.0.1",
			PackageName: "0",
		},
		"oci://localhost:5000/pkg/1:0.0.1-amd64@sha256:" + strings.Repeat("a", 64): {
			Reference: registry.Reference{
				Registry:   "localhost:5000",
				Repository: "pkg/1",
				Reference:  "sha256:" + strings.Repeat("a", 64),
			},
			Arch:        "amd64",
			Version:     "0.0.1",
			PackageName: "1",
		},
	}
	for input, expected := range cases {
		actual, err := ParseZarfPackageReference(input)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
}
