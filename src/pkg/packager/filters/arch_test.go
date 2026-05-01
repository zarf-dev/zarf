// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package filters_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
)

func TestClusterArchitectureFilter(t *testing.T) {
	t.Parallel()
	pkg := v1alpha1.ZarfPackage{
		Components: []v1alpha1.ZarfComponent{
			{Name: "no-arch"},
			{Name: "amd64", Only: v1alpha1.ZarfComponentOnlyTarget{Cluster: v1alpha1.ZarfComponentOnlyCluster{Architecture: "amd64"}}},
			{Name: "arm64", Only: v1alpha1.ZarfComponentOnlyTarget{Cluster: v1alpha1.ZarfComponentOnlyCluster{Architecture: "arm64"}}},
		},
	}

	tests := []struct {
		name     string
		hostArch string
		want     []string
		err      error
	}{
		{name: "missing host arch errors", hostArch: "", err: filters.ErrHostArchRequired},
		{name: "host amd64 keeps unpinned and amd64", hostArch: "amd64", want: []string{"no-arch", "amd64"}},
		{name: "host arm64 keeps unpinned and arm64", hostArch: "arm64", want: []string{"no-arch", "arm64"}},
		{name: "unrelated host arch keeps only unpinned", hostArch: "ppc64le", want: []string{"no-arch"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := filters.ByArchitecture(tt.hostArch).Apply(pkg)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			names := make([]string, len(got))
			for i, c := range got {
				names[i] = c.Name
			}
			require.Equal(t, tt.want, names)
		})
	}
}
