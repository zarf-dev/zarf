// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

func TestLocalOSFilter(t *testing.T) {
	pkg := v1alpha1.ZarfPackage{}
	for _, os := range v1alpha1.SupportedOS() {
		pkg.Components = append(pkg.Components, v1alpha1.ZarfComponent{
			Only: v1alpha1.ZarfComponentOnlyTarget{
				LocalOS: os,
			},
		})
	}

	for _, os := range v1alpha1.SupportedOS() {
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
