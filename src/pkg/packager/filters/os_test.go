// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"testing"

	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
)

func TestLocalOSFilter(t *testing.T) {

	pkg := types.ZarfPackage{}
	for _, os := range validate.SupportedOS() {
		pkg.Components = append(pkg.Components, types.ZarfComponent{
			Only: types.ZarfComponentOnlyTarget{
				LocalOS: os,
			},
		})
	}

	for _, os := range validate.SupportedOS() {
		filter := ByLocalOS(os)
		result, err := filter.Apply(pkg)
		if os == "" {
			require.ErrorIs(t, err, ErrLocalOSRequired)
		} else {
			require.NoError(t, err)
		}
		for _, component := range result {
			if component.Only.LocalOS != "" {
				require.Equal(t, os, component.Only.LocalOS)
			}
		}
	}
}
