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
		case "required=<nil>":
			c.Required = nil
		case "required=false":
			c.Required = helpers.BoolPtr(false)
		case "required=true":
			c.Required = helpers.BoolPtr(true)
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

func componentMatrix(t *testing.T) []types.ZarfComponent {
	var components []types.ZarfComponent

	defaultValues := []bool{true, false}
	requiredValues := []interface{}{nil, true, false}

	// all possible combinations of default, required, and optional
	for _, defaultValue := range defaultValues {
		for _, requiredValue := range requiredValues {

			// per validate, components cannot be both default and required
			if defaultValue == true && requiredValue == true {
				continue
			}

			query := fmt.Sprintf("required=%v", requiredValue)

			if defaultValue {
				query = fmt.Sprintf("%s && default=true", query)
			}

			c := componentFromQuery(t, query)
			components = append(components, c)
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
			// c.Default == true && c.Required == true)
			c.Required = nil
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
		"Test when no optional components selected": {
			pkg: types.ZarfPackage{
				Components: possibilities,
			},
			optionalComponents: "",
			want: []types.ZarfComponent{
				componentFromQuery(t, "required=<nil> && default=true"),
				componentFromQuery(t, "required=false && default=true"),
				componentFromQuery(t, "required=true"),
				componentFromQuery(t, "group=foo && idx=0 && default=true"),
				componentFromQuery(t, "group=bar && idx=0 && default=true"),
			},
		},
		"Test when some optional components selected": {
			pkg: types.ZarfPackage{
				Components: possibilities,
			},
			optionalComponents: strings.Join([]string{
				"required=false",
				"group=bar && idx=2 && default=false",
				"-required=true",
			}, ","),
			want: []types.ZarfComponent{
				componentFromQuery(t, "required=<nil> && default=true"),
				componentFromQuery(t, "required=false && default=true"),
				componentFromQuery(t, "required=true"),  // required components cannot be deselected
				componentFromQuery(t, "required=false"), // optional components can be selected
				componentFromQuery(t, "group=foo && idx=0 && default=true"),
				componentFromQuery(t, "group=bar && idx=2 && default=false"), // components within a group can be selected, the default is not selected
			},
		},
		"Test failing when group has no default and no selection was made": {
			pkg: types.ZarfPackage{
				Components: []types.ZarfComponent{
					componentFromQuery(t, "group=foo && default=true"),
					componentFromQuery(t, "group=foo && default=false"),
					componentFromQuery(t, "group=foo && default=false"),
				},
			},
			optionalComponents: "-group=foo && default=true",
			expectedErr:        ErrNoDefaultOrSelection,
		},
		"Test failing when multiple are selected from the same group": {
			pkg: types.ZarfPackage{
				Components: []types.ZarfComponent{
					componentFromQuery(t, "group=foo && default=true"),
					componentFromQuery(t, "group=foo && default=false"),
				},
			},
			optionalComponents: strings.Join([]string{"group=foo && default=false", "group=foo && default=true"}, ","),
			expectedErr:        ErrMultipleSameGroup,
		},
		"Test failing when no components are found that match the query": {
			pkg: types.ZarfPackage{
				Components: possibilities,
			},
			optionalComponents: "nonexistent",
			expectedErr:        ErrNotFound,
		},
		"[default-required] Test when no optional components selected": {
			pkg: types.ZarfPackage{
				Metadata: types.ZarfMetadata{
					Features: []types.FeatureFlag{
						types.DefaultRequired,
					},
				},
				Components: possibilities,
			},
			optionalComponents: "",
			want: []types.ZarfComponent{
				componentFromQuery(t, "required=<nil> && default=true"),
				componentFromQuery(t, "required=false && default=true"),
				componentFromQuery(t, "required=<nil>"),
				componentFromQuery(t, "required=true"),
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
