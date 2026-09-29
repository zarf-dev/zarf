// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

func TestSBOMResourcePathRoundTrip(t *testing.T) {
	t.Parallel()

	for _, key := range []string{
		"component:metrics",
		"image:registry.example/app:1.2.3/linux/amd64",
	} {
		t.Run(key, func(t *testing.T) {
			path := SBOMResourcePath(key)
			actual, ok := SBOMResourceKey(path)
			require.True(t, ok)
			require.Equal(t, key, actual)
		})
	}
}

func TestUsesGranularResourceLayout(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		apiVersion string
		want       bool
	}{
		"omitted version defaults to archive resources": {want: false},
		"v1alpha1 uses archive resources":               {apiVersion: v1alpha1.APIVersion, want: false},
		"v1beta1 uses granular resources":               {apiVersion: v1beta1.APIVersion, want: true},
		"future versions use granular resources":        {apiVersion: "zarf.dev/v1gamma1", want: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tt.want, UsesGranularResourceLayout(api.Package{APIVersion: tt.apiVersion}))
		})
	}
}
