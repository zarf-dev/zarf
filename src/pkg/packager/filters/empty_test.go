// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/types"
)

func TestEmptyFilter_Apply(t *testing.T) {
	components := []types.ZarfComponent{
		{
			Name: "component1",
		},
		{
			Name: "component2",
		},
	}
	pkg := types.ZarfPackage{
		Components: components,
	}
	filter := Empty()

	result, err := filter.Apply(pkg)

	require.NoError(t, err)
	require.Equal(t, components, result)
}
