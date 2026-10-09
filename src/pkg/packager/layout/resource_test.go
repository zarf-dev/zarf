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
