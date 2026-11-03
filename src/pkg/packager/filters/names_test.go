// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package filters

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
)

func TestByComponentNamesIsIdempotent(t *testing.T) {
	t.Parallel()

	filter := ByComponentNames([]string{"two"})
	pkg := api.Package{Components: []api.Component{{Name: "one"}, {Name: "two"}}}

	first, err := filter.Apply(pkg)
	require.NoError(t, err)
	second, err := filter.Apply(api.Package{Components: first})
	require.NoError(t, err)
	require.Equal(t, []api.Component{{Name: "two"}}, first)
	require.Equal(t, first, second)
}
