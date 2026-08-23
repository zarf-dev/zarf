// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package filters_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
)

func TestLocalOSFilter(t *testing.T) {
	supportedOS := []string{"linux", "darwin", "windows", ""}
	pkg := filters.PackageView{}
	for _, os := range supportedOS {
		pkg.Components = append(pkg.Components, filters.ComponentView{
			OnlyLocalOS: os,
		})
	}

	for _, os := range supportedOS {
		filter := filters.ByLocalOS(os)
		result, err := filter.Apply(pkg)
		if os == "" {
			require.ErrorIs(t, err, filters.ErrLocalOSRequired)
		} else {
			require.NoError(t, err)
		}
		for _, idx := range result {
			component := pkg.Components[idx]
			if component.OnlyLocalOS != "" {
				require.Equal(t, os, component.OnlyLocalOS)
			}
		}
	}
}
