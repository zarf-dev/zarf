// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package api_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

func TestPackageGetAPIVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		apiVersion string
		want       string
	}{
		{
			name: "defaults omitted version to v1alpha1",
			want: v1alpha1.APIVersion,
		},
		{
			name:       "preserves specified version",
			apiVersion: v1beta1.APIVersion,
			want:       v1beta1.APIVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, api.Package{APIVersion: tt.apiVersion}.GetAPIVersion())
		})
	}
}
