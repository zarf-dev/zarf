// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	legacyerrdef "oras.land/oras-go/v2/errdef"
)

func TestReferenceFromMetadataLegacy(t *testing.T) {
	t.Parallel()

	pkg := api.Package{Metadata: api.PackageMetadata{Name: "test", Version: "1.0.0"}}
	ref, err := ReferenceFromMetadata("registry.example.com/team", pkg)
	require.NoError(t, err)
	require.Equal(t, "1.0.0", ref.Reference)
	require.Equal(t, "registry.example.com/team/test:1.0.0", ref.String())

	ref, err = ReferenceFromMetadataWithOptions("registry.example.com/team", pkg, ReferenceFromMetadataOptions{Tag: "release"})
	require.NoError(t, err)
	require.Equal(t, "release", ref.Reference)
	require.Equal(t, "registry.example.com/team/test:release", ref.String())

	_, err = ReferenceFromMetadataWithOptions("registry.example.com/team", pkg, ReferenceFromMetadataOptions{Tag: "invalid tag"})
	require.ErrorIs(t, err, legacyerrdef.ErrInvalidReference)
}

func TestNormalizeOCISource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		source   string
		expected string
	}{
		{
			name:     "registry domain",
			source:   "ghcr.io/zarf-dev/packages/init:v0.84.0",
			expected: "oci://ghcr.io/zarf-dev/packages/init:v0.84.0",
		},
		{
			name:     "localhost registry",
			source:   "localhost:5000/packages/init:v0.84.0",
			expected: "oci://localhost:5000/packages/init:v0.84.0",
		},
		{
			name:     "IPv6 registry",
			source:   "[::1]:5000/packages/init:v0.84.0",
			expected: "oci://[::1]:5000/packages/init:v0.84.0",
		},
		{
			name:     "explicit OCI URL",
			source:   "oci://ghcr.io/zarf-dev/packages/init:v0.84.0",
			expected: "oci://ghcr.io/zarf-dev/packages/init:v0.84.0",
		},
		{
			name:     "cluster package name",
			source:   "my-package",
			expected: "my-package",
		},
		{
			name:     "local tarball",
			source:   "packages/init.tar.zst",
			expected: "packages/init.tar.zst",
		},
		{
			name:     "short registry hostname",
			source:   "packages/init:v0.84.0",
			expected: "oci://packages/init:v0.84.0",
		},
		{
			name:     "invalid OCI reference",
			source:   "packages/Init:v0.84.0",
			expected: "packages/Init:v0.84.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, NormalizeOCISource(tt.source))
		})
	}
}
