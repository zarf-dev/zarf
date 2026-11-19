// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"fmt"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

func componentFromQuery(t *testing.T, q string) v1alpha1.ZarfComponent {
	c := v1alpha1.ZarfComponent{
		Name: q,
	}

	conditions := strings.Split(q, "&&")
	for _, cond := range conditions {
		cond = strings.TrimSpace(cond)
		switch cond {
		case "default=true":
			c.Default = true
		case "default=false":
			c.Default = false
		case "required=<nil>":
			c.Required = nil
		case "required=false":
			c.Required = new(false)
		case "required=true":
			c.Required = new(true)
		default:
			if strings.HasPrefix(cond, "group=") {
				c.DeprecatedGroup = cond[6:]
				continue
			}
			if strings.HasPrefix(cond, "idx=") {
				continue
			}
			require.FailNow(t, "unknown condition", "unknown condition %q", cond)
		}
	}

	return c
}

func componentMatrix(t *testing.T) []v1alpha1.ZarfComponent {
	t.Helper()
	var components []v1alpha1.ZarfComponent

	defaultValues := []bool{true, false}
	requiredValues := []interface{}{nil, true, false}
	// the duplicate groups are intentional
	// this is to test group membership + default filtering
	groupValues := []string{"", "foo", "foo", "foo", "bar", "bar", "bar"}

	for idx, groupValue := range groupValues {
		for _, defaultValue := range defaultValues {
			for _, requiredValue := range requiredValues {
				name := strings.Builder{}

				// per validate rules, components in groups cannot be required
				if requiredValue != nil {
					boolValue, ok := requiredValue.(bool)
					require.True(t, ok)
					if boolValue && groupValue != "" {
						continue
					}
				}

				fmt.Fprintf(&name, "required=%v", requiredValue)

				if groupValue != "" {
					fmt.Fprintf(&name, " && group=%s && idx=%d && default=%t", groupValue, idx, defaultValue)
				} else if defaultValue {
					name.WriteString(" && default=true")
				}

				if groupValue != "" {
					// if there already exists a component in this group that is default, then set the default to false
					// otherwise the filter will error
					defaultAlreadyExists := false
					if defaultValue {
						for _, c := range components {
							if c.DeprecatedGroup == groupValue && c.Default {
								defaultAlreadyExists = true
								break
							}
						}
					}
					if defaultAlreadyExists {
						defaultValue = false
					}
				}

				c := v1alpha1.ZarfComponent{
					Name:            name.String(),
					Default:         defaultValue,
					DeprecatedGroup: groupValue,
				}

				if requiredValue != nil {
					boolValue, ok := requiredValue.(bool)
					require.True(t, ok)
					c.Required = new(boolValue)
				}

				components = append(components, c)
			}
		}
	}

	return components
}

func TestDeployFilter_Apply(t *testing.T) {
	possibilities := componentMatrix(t)

	tests := map[string]struct {
		pkg                v1alpha1.ZarfPackage
		optionalComponents string
		want               []v1alpha1.ZarfComponent
		expectedErr        error
	}{
		"Test when version is less than v0.33.0 w/ no optional components selected": {
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					Version: "v0.32.0",
				},
				Components: possibilities,
			},
			optionalComponents: "",
			want: []v1alpha1.ZarfComponent{
				componentFromQuery(t, "required=<nil> && default=true"),
				componentFromQuery(t, "required=true && default=true"),
				componentFromQuery(t, "required=false && default=true"),
				componentFromQuery(t, "required=true"),
				componentFromQuery(t, "required=<nil> && group=foo && idx=1 && default=true"),
				componentFromQuery(t, "required=<nil> && group=bar && idx=4 && default=true"),
			},
		},
		"Test when version is less than v0.33.0 w/ some optional components selected": {
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					Version: "v0.32.0",
				},
				Components: possibilities,
			},
			optionalComponents: strings.Join([]string{"required=false", "required=<nil> && group=bar && idx=5 && default=false", "-required=true"}, ","),
			want: []v1alpha1.ZarfComponent{
				componentFromQuery(t, "required=<nil> && default=true"),
				componentFromQuery(t, "required=true && default=true"),
				componentFromQuery(t, "required=false && default=true"),
				// while "required=true" was deselected, it is still required
				// therefore it should be included
				componentFromQuery(t, "required=true"),
				componentFromQuery(t, "required=false"),
				componentFromQuery(t, "required=<nil> && group=foo && idx=1 && default=true"),
				componentFromQuery(t, "required=<nil> && group=bar && idx=5 && default=false"),
			},
		},
		"Test failing when group has no default and no selection was made": {
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					Version: "v0.32.0",
				},
				Components: []v1alpha1.ZarfComponent{
					componentFromQuery(t, "group=foo && default=false"),
					componentFromQuery(t, "group=foo && default=false"),
				},
			},
			optionalComponents: "",
			expectedErr:        ErrNoDefaultOrSelection,
		},
		"Test failing when multiple are selected from the same group": {
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					Version: "v0.32.0",
				},
				Components: []v1alpha1.ZarfComponent{
					componentFromQuery(t, "group=foo && default=true"),
					componentFromQuery(t, "group=foo && default=false"),
				},
			},
			optionalComponents: strings.Join([]string{"group=foo && default=false", "group=foo && default=true"}, ","),
			expectedErr:        ErrMultipleSameGroup,
		},
		"Test failing when no components are found that match the query": {
			pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					Version: "v0.32.0",
				},
				Components: possibilities,
			},
			optionalComponents: "nonexistent",
			expectedErr:        ErrNotFound,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			filter := ForDeploy(tt.optionalComponents)

			components, err := filter.Apply(convert.PackageFromV1alpha1(tt.pkg))
			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, convert.PackageFromV1alpha1(v1alpha1.ZarfPackage{Components: tt.want}).Components, components)
			}
		})
	}
}

func TestDeployFilterOverlappingSelectors(t *testing.T) {
	t.Parallel()

	pkg := api.Package{Components: []api.Component{
		{Name: "abc", Optional: true},
		{Name: "axyz", Optional: true},
		{Name: "default", Optional: true, Default: true},
		{Name: "required"},
		{Name: "group-default", Optional: true, Default: true, Group: "choice"},
		{Name: "group-other", Optional: true, Group: "choice"},
	}}
	tests := []struct {
		name      string
		selectors string
		want      []string
		wantErr   error
	}{
		{"exact then glob", "abc,a*", []string{"abc", "axyz", "default", "required", "group-default"}, nil},
		{"glob then exact", "a*,abc", []string{"abc", "axyz", "default", "required", "group-default"}, nil},
		{"fully excluded inclusion", "abc,-abc", []string{"default", "required", "group-default"}, nil},
		{"later exclusion wins", "abc,a*,-abc", []string{"axyz", "default", "required", "group-default"}, nil},
		{"later inclusion wins", "-abc,a*,abc", []string{"abc", "axyz", "default", "required", "group-default"}, nil},
		{"repeated inclusion wins", "abc,-a*,abc", []string{"abc", "default", "required", "group-default"}, nil},
		{"overlapping exclusions", "-a*,-abc", []string{"default", "required", "group-default"}, nil},
		{"excluded required remains selected", "-required,required", []string{"default", "required", "group-default"}, nil},
		{"group selection replaces default", "group-other,group-o*", []string{"default", "required", "group-other"}, nil},
		{"unmatched inclusion", "abc,a*,missing", nil, ErrNotFound},
		{"unmatched exclusion", "abc,a*,-missing", nil, ErrNotFound},
		{"multiple group selections", "group-*", nil, ErrMultipleSameGroup},
		{"empty selector entries", " , ABC, ,", []string{"abc", "default", "required", "group-default"}, nil},
		{"only separators and whitespace", " , , ", []string{"default", "required", "group-default"}, nil},
		{"ordered selectors with empty entries", " ABC, , -A*, ABC, ", []string{"abc", "default", "required", "group-default"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			components, err := ForDeploy(tt.selectors).Apply(pkg)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			names := make([]string, 0, len(components))
			for _, component := range components {
				names = append(names, component.Name)
			}
			require.Equal(t, tt.want, names)
		})
	}
}

func TestDeployFilterExplicitRequiredGroupChoice(t *testing.T) {
	t.Parallel()

	pkg := api.Package{Components: []api.Component{
		{Name: "required"},
		{Name: "group-a", Group: "group"},
		{Name: "group-b", Group: "group"},
	}}
	for i, name := range []string{"group-a", "group-b"} {
		t.Run(name, func(t *testing.T) {
			components, err := ForDeploy(name + ",-required").Apply(pkg)
			require.NoError(t, err)
			require.Equal(t, []api.Component{pkg.Components[0], pkg.Components[i+1]}, components)
		})
	}
	_, err := ForDeploy("group-a,group-b").Apply(pkg)
	require.ErrorIs(t, err, ErrMultipleSameGroup)
}

func TestDeployFilterRejectsInvalidSelectors(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		pkg  api.Package
	}{
		{"empty package", api.Package{}},
		{"required component", api.Package{Components: []api.Component{{Name: "required"}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ForDeploy(" , [, ").Apply(tt.pkg)
			require.ErrorIs(t, err, path.ErrBadPattern)
		})
	}
}

func TestLevenshteinDistance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		s1   string
		s2   string
		want int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "abc", 3},
		{"kitten", "sitting", 3},
		{"zarf", "zarf", 0},
		{"dos-game", "dos-games", 1},
		{"manifest", "manifests", 1},
		{"hello", "world", 4},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.s1, tt.s2), func(t *testing.T) {
			got := levenshteinDistance(tt.s1, tt.s2)
			require.Equal(t, tt.want, got)
		})
	}

	// Verify suggestion behavior on unknown component
	pkg := api.Package{Components: []api.Component{
		{Name: "dos-games"},
		{Name: "manifests"},
		{Name: "logging"},
	}}
	_, err := ForDeploy("dos-game").Apply(pkg)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotFound)
	require.Contains(t, err.Error(), "suggestions (dos-games)")
}
