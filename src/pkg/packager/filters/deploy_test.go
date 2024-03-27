// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
)

func componentFromQuery(t *testing.T, q string) types.ZarfComponent {
	c := types.ZarfComponent{
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
		case "optional=<nil>":
			c.Optional = nil
		case "optional=false":
			c.Optional = helpers.BoolPtr(false)
		case "optional=true":
			c.Optional = helpers.BoolPtr(true)
		case "required=<nil>":
			c.DeprecatedRequired = nil
		case "required=false":
			c.DeprecatedRequired = helpers.BoolPtr(false)
		case "required=true":
			c.DeprecatedRequired = helpers.BoolPtr(true)
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

// componentMatrix generates all possible combinations of component values
// for testing the deploy filter.
// some of these combinations are invalid, but the deploy filter should
// handle them gracefully.
func componentMatrix(t *testing.T) []types.ZarfComponent {
	var components []types.ZarfComponent

	defaultValues := []bool{true, false}
	requiredValues := []interface{}{nil, true, false}
	optionalValues := []interface{}{nil, true, false}

	// all possible combinations of default, required, and optional
	for _, defaultValue := range defaultValues {
		for _, optionalValue := range optionalValues {
			for _, requiredValue := range requiredValues {

				// per validate, components cannot be both default and required
				if defaultValue == true && (requiredValue == true || optionalValue == false || optionalValue == nil) {
					continue
				}

				query := fmt.Sprintf("default=%t && required=%v && optional=%v", defaultValue, requiredValue, optionalValue)

				c := componentFromQuery(t, query)
				components = append(components, c)
			}
		}
	}

	members := 3
	for _, group := range []string{"foo", "bar"} {
		for i := 0; i < members; i++ {
			var defaultValue bool
			// ensure there is only one default per group
			// this enforced on `zarf package create`'s validate
			if i == 0 {
				defaultValue = true
			}
			c := componentFromQuery(t, fmt.Sprintf("group=%s && idx=%d && default=%t", group, i, defaultValue))
			// due to validation on create, there will not be a case where
			// c.Default == true && (c.Optional = false || c.DeprecatedRequired = true)
			c.Optional = nil
			c.DeprecatedRequired = nil
			components = append(components, c)
		}
	}

	return components
}

func TestDeployFilter_Apply(t *testing.T) {

	possibilities := componentMatrix(t)

	testCases := map[string]struct {
		pkg                types.ZarfPackage
		optionalComponents string
		want               []types.ZarfComponent
		expectedErr        error
	}{
		"[v0.32.6] Test when no optional components selected": {
			pkg: types.ZarfPackage{
				Build: types.ZarfBuildData{
					Version: "v0.32.6",
				},
				Components: possibilities,
			},
			optionalComponents: "",
			want: []types.ZarfComponent{
				componentFromQuery(t, "default=true && required=<nil> && optional=true"),
				componentFromQuery(t, "default=true && required=false && optional=true"),
				componentFromQuery(t, "default=false && required=true && optional=<nil>"),
				componentFromQuery(t, "default=false && required=true && optional=true"),
				componentFromQuery(t, "default=false && required=true && optional=false"),
				componentFromQuery(t, "group=foo && idx=0 && default=true"),
				componentFromQuery(t, "group=bar && idx=0 && default=true"),
			},
		},
		"[v0.32.6] Test when some optional components selected": {
			pkg: types.ZarfPackage{
				Build: types.ZarfBuildData{
					Version: "v0.32.6",
				},
				Components: possibilities,
			},
			optionalComponents: strings.Join([]string{
				"default=false && required=false && optional=true",
				"group=bar && idx=2 && default=false",
				"-default=false && required=true && optional=false",
			}, ","),
			want: []types.ZarfComponent{
				componentFromQuery(t, "default=true && required=<nil> && optional=true"),
				componentFromQuery(t, "default=true && required=false && optional=true"),
				componentFromQuery(t, "default=false && required=true && optional=<nil>"),
				componentFromQuery(t, "default=false && required=true && optional=true"),
				componentFromQuery(t, "default=false && required=false && optional=true"),
				componentFromQuery(t, "default=false && required=true && optional=false"),
				componentFromQuery(t, "group=foo && idx=0 && default=true"),
				componentFromQuery(t, "group=bar && idx=2 && default=false"),
			},
		},
		"[v0.32.6] Test failing when group has no default and no selection was made": {
			pkg: types.ZarfPackage{
				Build: types.ZarfBuildData{
					Version: "v0.32.6",
				},
				Components: []types.ZarfComponent{
					componentFromQuery(t, "group=foo && default=true"),
					componentFromQuery(t, "group=foo && default=false"),
					componentFromQuery(t, "group=foo && default=false"),
				},
			},
			optionalComponents: "-group=foo && default=true",
			expectedErr:        ErrNoDefaultOrSelection,
		},
		"[v0.32.6] Test failing when multiple are selected from the same group": {
			pkg: types.ZarfPackage{
				Build: types.ZarfBuildData{
					Version: "v0.32.6",
				},
				Components: []types.ZarfComponent{
					componentFromQuery(t, "group=foo && default=true"),
					componentFromQuery(t, "group=foo && default=false"),
				},
			},
			optionalComponents: strings.Join([]string{"group=foo && default=false", "group=foo && default=true"}, ","),
			expectedErr:        ErrMultipleSameGroup,
		},
		"[v0.32.6] Test failing when no components are found that match the query": {
			pkg: types.ZarfPackage{
				Build: types.ZarfBuildData{
					Version: "v0.32.6",
				},
				Components: possibilities,
			},
			optionalComponents: "nonexistent",
			expectedErr:        ErrNotFound,
		},
		"[v0.33.0+] Test when no optional components selected": {
			pkg: types.ZarfPackage{
				Build: types.ZarfBuildData{
					Version: "v0.33.0",
				},
				Components: possibilities,
			},
			optionalComponents: "",
			want: []types.ZarfComponent{
				componentFromQuery(t, "default=true && required=<nil> && optional=true"),
				componentFromQuery(t, "default=true && required=false && optional=true"),
				componentFromQuery(t, "default=false && required=<nil> && optional=<nil>"),
				componentFromQuery(t, "default=false && required=true && optional=<nil>"),
				componentFromQuery(t, "default=false && required=false && optional=<nil>"),
				componentFromQuery(t, "default=false && required=<nil> && optional=false"),
				componentFromQuery(t, "default=false && required=true && optional=false"),
				componentFromQuery(t, "default=false && required=false && optional=false"),
				componentFromQuery(t, "group=foo && idx=0 && default=true"),
				componentFromQuery(t, "group=bar && idx=0 && default=true"),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// we do not currently support interactive mode in unit tests
			isInteractive := false
			filter := ForDeploy(tc.optionalComponents, isInteractive)

			result, err := filter.Apply(tc.pkg)
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
			equal := reflect.DeepEqual(tc.want, result)
			if !equal {
				left := []string{}
				right := []string{}

				for _, c := range tc.want {
					left = append(left, c.Name)
				}

				for _, c := range result {
					right = append(right, c.Name)
					fmt.Printf("componentFromQuery(t, %q),\n", strings.TrimSpace(c.Name))
				}

				// cause the test to fail
				require.FailNow(t, "expected and actual are not equal", "\n\nexpected: %#v\n\nactual: %#v", left, right)
			}
		})
	}
}
