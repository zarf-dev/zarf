// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package selection

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
)

func TestDeploymentSelectorsReplay(t *testing.T) {
	pkg := api.Package{Components: []api.Component{
		{Name: "required"},
		{Name: "declined", Optional: true, Default: true},
		{Name: "first", Group: "choice", Default: true},
		{Name: "selected", Optional: true},
		{Name: "second", Group: "choice"},
	}}
	var calls []string
	selectors, err := promptDeploymentSelectors(pkg, deploymentPrompts{
		optional: func(_ api.Package, c api.Component) (bool, error) {
			calls = append(calls, c.Name)
			return c.Name == "selected", nil
		},
		group: func(group []api.Component) (int, error) {
			calls = append(calls, "group")
			require.Equal(t, "second", group[1].Name)
			return 1, nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"declined", "group", "selected"}, calls)

	components, err := filters.ForDeploy(selectors).Apply(pkg)
	require.NoError(t, err)
	require.Equal(t, []api.Component{pkg.Components[0], pkg.Components[4], pkg.Components[3]}, components)
}

func TestDeploymentSelectorsErrors(t *testing.T) {
	canceled := errors.New("interrupted")
	pkg := api.Package{Components: []api.Component{{Name: "optional", Optional: true}, {Name: "a", Group: "g"}, {Name: "b", Group: "g"}}}
	for _, tc := range []struct {
		name    string
		prompts deploymentPrompts
		want    string
	}{
		{"optional canceled", deploymentPrompts{optional: func(api.Package, api.Component) (bool, error) { return false, canceled }}, "selection canceled: interrupted"},
		{"group canceled", deploymentPrompts{optional: func(api.Package, api.Component) (bool, error) { return true, nil }, group: func([]api.Component) (int, error) { return 0, canceled }}, "selection canceled: interrupted"},
		{"invalid group index", deploymentPrompts{optional: func(api.Package, api.Component) (bool, error) { return true, nil }, group: func([]api.Component) (int, error) { return 2, nil }}, "invalid component choice"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selectors, err := promptDeploymentSelectors(pkg, tc.prompts)
			require.ErrorContains(t, err, tc.want)
			require.Empty(t, selectors)
			if tc.name != "invalid group index" {
				require.ErrorIs(t, err, canceled)
			}
		})
	}
}
