// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
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

	result, err := combo.Apply(pkg)
	require.NoError(t, err)
	require.Equal(t, expected, result)

	// Test error propagation
	combo = Combine(f1, f2, ForDeploy("group with no default", false))
	pkg.Components = append(pkg.Components, v1alpha1.ZarfComponent{
		Name:            "group with no default",
		DeprecatedGroup: "g1",
	})
	_, err = combo.Apply(pkg)
	require.Error(t, err)
}
