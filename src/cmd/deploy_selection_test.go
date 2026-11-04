// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
)

type selectionTestLayout struct {
	pkg api.Package
}

func (l *selectionTestLayout) Filter(filter filters.ComponentFilterStrategy) error {
	pkg, err := filters.Apply(l.pkg, filter)
	if err != nil {
		return err
	}
	l.pkg = pkg
	return nil
}

func TestInteractiveDeploymentSelection(t *testing.T) {
	pkg := api.Package{Components: []api.Component{
		{Name: "required"},
		{Name: "declined", Optional: true, Default: true},
		{Name: "first", Group: "choice"},
		{Name: "selected", Optional: true},
		{Name: "second", Group: "choice"},
		{Name: "other-os", Optional: true, Target: api.ComponentTarget{OS: "not-a-real-os"}},
	}}
	layout := &selectionTestLayout{pkg: pkg}
	var calls []string
	selectors, err := resolveDeploymentSelectors(layout, "", deploymentPrompts{
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
	require.Equal(t, "-declined,second,selected", selectors)
	require.Equal(t, []api.Component{pkg.Components[0], pkg.Components[4], pkg.Components[3]}, layout.pkg.Components)
}

func TestDeploymentSelectorReplay(t *testing.T) {
	pkg := api.Package{Components: []api.Component{
		{Name: "required"},
		{Name: "default", Optional: true, Default: true},
		{Name: "first", Group: "choice", Optional: true, Default: true},
		{Name: "second", Group: "choice", Optional: true},
		{Name: "extra", Optional: true},
	}}
	interactive := &selectionTestLayout{pkg: pkg}
	selectors, err := resolveDeploymentSelectors(interactive, "", deploymentPrompts{
		optional: func(_ api.Package, c api.Component) (bool, error) { return c.Name == "extra", nil },
		group:    func([]api.Component) (int, error) { return 1, nil },
	})
	require.NoError(t, err)
	require.Equal(t, "-default,extra,second", selectors)
	replay := &selectionTestLayout{pkg: pkg}
	resolved, err := resolveDeploymentSelectors(replay, " EXTRA, -DEFAULT,second ", deploymentPrompts{
		optional: func(api.Package, api.Component) (bool, error) { t.Fatal("unexpected prompt"); return false, nil },
		group:    func([]api.Component) (int, error) { t.Fatal("unexpected prompt"); return 0, nil },
	})
	require.NoError(t, err)
	require.Equal(t, selectors, resolved)
	require.Equal(t, interactive.pkg.Components, replay.pkg.Components)
}

func TestDeploymentSelectionErrors(t *testing.T) {
	canceled := errors.New("interrupted")
	pkg := api.Package{Components: []api.Component{{Name: "optional", Optional: true}, {Name: "a", Group: "g", Optional: true, Default: true}, {Name: "b", Group: "g", Optional: true}}}
	for _, tc := range []struct {
		name      string
		selectors string
		prompts   deploymentPrompts
		want      string
	}{
		{name: "invalid selector", selectors: "[", want: "syntax"},
		{name: "unknown selector", selectors: "missing", want: "no compatible components found"},
		{name: "optional canceled", prompts: deploymentPrompts{optional: func(api.Package, api.Component) (bool, error) { return false, canceled }}, want: "selection canceled: interrupted"},
		{name: "group canceled", prompts: deploymentPrompts{optional: func(api.Package, api.Component) (bool, error) { return true, nil }, group: func([]api.Component) (int, error) { return 0, canceled }}, want: "selection canceled: interrupted"},
		{name: "invalid group index", prompts: deploymentPrompts{optional: func(api.Package, api.Component) (bool, error) { return true, nil }, group: func([]api.Component) (int, error) { return 2, nil }}, want: "invalid component choice"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layout := &selectionTestLayout{pkg: pkg}
			selectors, err := resolveDeploymentSelectors(layout, tc.selectors, tc.prompts)
			require.ErrorContains(t, err, tc.want)
			require.Empty(t, selectors)
			require.Equal(t, pkg, layout.pkg)
			if tc.name == "optional canceled" || tc.name == "group canceled" {
				require.ErrorIs(t, err, canceled)
			}
		})
	}
}
