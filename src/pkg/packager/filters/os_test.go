// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package filters_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
)

func TestLocalOSFilter(t *testing.T) {
	supportedOS := []string{"linux", "darwin", "windows", ""}
	pkg := api.Package{}
	for _, os := range supportedOS {
		pkg.Components = append(pkg.Components, api.Component{
			Target: api.ComponentTarget{OS: os},
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
		for _, component := range result {
			if component.Target.OS != "" {
				require.Equal(t, os, component.Target.OS)
			}
		}
	}
}
