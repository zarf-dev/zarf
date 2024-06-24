// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"fmt"
	"strings"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
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

func componentMatrix(_ *testing.T) []types.ZarfComponent {
	var components []types.ZarfComponent

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
				if requiredValue != nil && requiredValue.(bool) == true && groupValue != "" {
					continue
				}

				name.WriteString(fmt.Sprintf("required=%v", requiredValue))

				if groupValue != "" {
					name.WriteString(fmt.Sprintf(" && group=%s && idx=%d && default=%t", groupValue, idx, defaultValue))
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

				c := types.ZarfComponent{
					Name:            name.String(),
					Default:         defaultValue,
					DeprecatedGroup: groupValue,
				}

				if requiredValue != nil {
					c.Required = helpers.BoolPtr(requiredValue.(bool))
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
		pkg                types.ZarfPackage
		optionalComponents string
		want               []types.ZarfComponent
		expectedErr        error
	}{
		"Test when version is less than v0.33.0 w/ no optional components selected": {
			pkg: types.ZarfPackage{
				Build: types.ZarfBuildData{
					Version: "v0.32.0",
				},
				Components: possibilities,
			},
			optionalComponents: "",
			want: []types.ZarfComponent{
				componentFromQuery(t, "required=<nil> && default=true"),
				componentFromQuery(t, "required=true && default=true"),
				componentFromQuery(t, "required=false && default=true"),
				componentFromQuery(t, "required=true"),
				componentFromQuery(t, "required=<nil> && group=foo && idx=1 && default=true"),
				componentFromQuery(t, "required=<nil> && group=bar && idx=4 && default=true"),
			},
		},
		"Test when version is less than v0.33.0 w/ some optional components selected": {
			pkg: types.ZarfPackage{
				Build: types.ZarfBuildData{
					Version: "v0.32.0",
				},
				Components: possibilities,
			},
			optionalComponents: strings.Join([]string{"required=false", "required=<nil> && group=bar && idx=5 && default=false", "-required=true"}, ","),
			want: []types.ZarfComponent{
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
			pkg: types.ZarfPackage{
				Build: types.ZarfBuildData{
					Version: "v0.32.0",
				},
				Components: []types.ZarfComponent{
					componentFromQuery(t, "group=foo && default=false"),
					componentFromQuery(t, "group=foo && default=false"),
				},
			},
			optionalComponents: "",
			expectedErr:        ErrNoDefaultOrSelection,
		},
		"Test failing when multiple are selected from the same group": {
			pkg: types.ZarfPackage{
				Build: types.ZarfBuildData{
					Version: "v0.32.0",
				},
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
				Build: types.ZarfBuildData{
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
			// we do not currently support interactive mode in unit tests
			isInteractive := false
			filter := ForDeploy(tt.optionalComponents, isInteractive)

			result, err := filter.Apply(tt.pkg)
			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.want, result)
		})
	}
}
