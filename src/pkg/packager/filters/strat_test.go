// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	internaltypes "github.com/zarf-dev/zarf/src/internal/api/types"
)

func TestCombine(t *testing.T) {
	f1 := BySelectState("*a*")
	f2 := BySelectState("*bar, foo")
	f3 := Empty()

	combo := Combine(f1, f2, f3)

	pkg := v1alpha1.ZarfPackage{
		Components: []v1alpha1.ZarfComponent{
			{
				Name: "foo",
			},
			{
				Name: "bar",
			},
			{
				Name: "baz",
			},
			{
				Name: "foobar",
			},
		},
	}

	expected := []v1alpha1.ZarfComponent{
		{
			Name: "bar",
		},
		{
			Name: "foobar",
		},
	}

	indices, err := combo.Apply(packageViewFromV1alpha1(pkg))
	result := selectV1alpha1Components(pkg, indices)
	require.NoError(t, err)
	require.Equal(t, expected, result)

	// Test error propagation
	combo = Combine(f1, f2, ForDeploy("group with no default", false))
	pkg.Components = append(pkg.Components, v1alpha1.ZarfComponent{
		Name:            "group with no default",
		DeprecatedGroup: "g1",
	})
	_, err = combo.Apply(packageViewFromV1alpha1(pkg))
	require.Error(t, err)
}

func TestApply(t *testing.T) {
	t.Parallel()

	definition := api.PackageDefinition{Pkg: internaltypes.Package{Components: []internaltypes.Component{
		{Name: "keep"},
		{Name: "discard", Target: internaltypes.ComponentTarget{OS: "windows"}},
	}}}

	filtered, err := Apply(definition, ByLocalOS("linux"))

	require.NoError(t, err)
	require.Len(t, definition.Pkg.Components, 2)
	require.Equal(t, []internaltypes.Component{{Name: "keep"}}, filtered.Pkg.Components)
}
