// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/oci"
)

func TestParseReference(t *testing.T) {
	t.Parallel()

	digest := "sha256:" + strings.Repeat("a", 64)
	for _, tc := range []struct {
		name string
		raw  string
		want oci.Reference
	}{
		{
			name: "repository",
			raw:  "registry.example.com/team/package",
			want: oci.Reference{Registry: "registry.example.com", Repository: "team/package"},
		},
		{
			name: "tag",
			raw:  "registry.example.com/team/package:release",
			want: oci.Reference{Registry: "registry.example.com", Repository: "team/package", Tag: "release"},
		},
		{
			name: "digest",
			raw:  "registry.example.com/team/package@" + digest,
			want: oci.Reference{Registry: "registry.example.com", Repository: "team/package", Digest: digest},
		},
		{
			name: "tag and digest",
			raw:  "oci://REGISTRY.EXAMPLE.COM/team/package:release@" + digest,
			want: oci.Reference{Registry: "registry.example.com", Repository: "team/package", Tag: "release", Digest: digest},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ref, err := oci.ParseReference(tc.raw)
			require.NoError(t, err)
			require.Equal(t, tc.want, ref)
			require.NoError(t, ref.Validate())
			selected := tc.want.Tag
			if tc.want.Digest != "" {
				selected = tc.want.Digest
			}
			require.Equal(t, selected, ref.GetReference())
			roundTrip, err := oci.ParseReference(ref.String())
			require.NoError(t, err)
			require.Equal(t, ref, roundTrip)
		})
	}

	t.Run("invalid tag with valid digest", func(t *testing.T) {
		_, err := oci.ParseReference("registry.example.com/team/package:bad tag@" + digest)
		require.Error(t, err)
	})
}
