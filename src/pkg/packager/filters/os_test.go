// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package filters

import (
	"testing"

	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
)

func TestArchAndOSFilter(t *testing.T) {

	pkg := types.ZarfPackage{}
	for _, os := range validate.SupportedOS() {
		pkg.Components = append(pkg.Components, types.ZarfComponent{
			Only: types.ZarfComponentOnlyTarget{
				LocalOS: os,
			},
		})
	}

	for _, os := range validate.SupportedOS() {
		filter := ByLocalOS()
		result, err := filter.Apply(pkg)
		require.NoError(t, err)
		require.Len(t, result, 2)
		for _, component := range result {
			if component.Only.LocalOS != "" {
				require.Equal(t, os, component.Only.LocalOS)
			}
		}
	}
}
