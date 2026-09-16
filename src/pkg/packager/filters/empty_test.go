// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
)

func TestEmptyFilter_Apply(t *testing.T) {
	pkg := api.Package{
		Components: []api.Component{
			{Name: "component1"},
			{Name: "component2"},
		},
	}
	filter := Empty()

	result, err := filter.Apply(pkg)

	require.NoError(t, err)
	require.Equal(t, []int{0, 1}, result)
}
