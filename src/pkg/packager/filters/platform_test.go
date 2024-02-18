// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package filters

import (
	"testing"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
)

func archOSMatrix() [][2]string {
	// Create a matrix of all possible combinations
	matrix := [][2]string{}
	for _, os := range allowedOs {
		for _, arch := range allowedArch {
			matrix = append(matrix, [2]string{arch, os})
		}
	}
	return matrix
}

func TestArchAndOSFilter(t *testing.T) {
	// Create a test package
	pkg := types.ZarfPackage{}
	for _, platform := range archOSMatrix() {
		pkg.Components = append(pkg.Components, types.ZarfComponent{
			Only: types.ZarfComponentOnlyTarget{
				LocalOS: platform[1],
				Cluster: types.ZarfComponentOnlyCluster{Architecture: platform[0]},
			},
		})
	}

	for _, platform := range archOSMatrix() {
		filter := ByArchAndOS(platform[0], platform[1])
		result, err := filter.Apply(pkg)
		require.NoError(t, err)
		for _, component := range result {
			if component.Only.Cluster.Architecture != "" {
				require.Equal(t, platform[0], component.Only.Cluster.Architecture)
			}
			if component.Only.LocalOS != "" {
				require.Equal(t, platform[1], component.Only.LocalOS)
			}
		}
	}
}
