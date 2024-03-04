package filters

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
)

func cfq(t *testing.T, q string) types.ZarfComponent {
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
			require.Fail(t, "unknown condition", "unknown condition %q", cond)
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

	testCases := map[string]struct {
		pkg                types.ZarfPackage
		optionalComponents string
		want               []types.ZarfComponent
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
				cfq(t, "required=<nil> && default=true"),
				cfq(t, "required=true && default=true"),
				cfq(t, "required=false && default=true"),
				cfq(t, "required=true"),
				cfq(t, "required=<nil> && group=foo && idx=1 && default=true"),
				cfq(t, "required=<nil> && group=bar && idx=4 && default=true"),
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
				cfq(t, "required=<nil> && default=true"),
				cfq(t, "required=true && default=true"),
				cfq(t, "required=false && default=true"),
				cfq(t, "required=true"),
				cfq(t, "required=false"),
				cfq(t, "required=<nil> && group=foo && idx=1 && default=true"),
				cfq(t, "required=<nil> && group=bar && idx=5 && default=false"),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// we do not currently support interactive mode in unit tests
			isInteractive := false
			filter := ForDeploy(tc.optionalComponents, isInteractive)

			result, err := filter.Apply(tc.pkg)
			require.NoError(t, err)
			equal := reflect.DeepEqual(tc.want, result)
			if !equal {
				left := []string{}
				right := []string{}

				for _, c := range tc.want {
					left = append(left, c.Name)
				}

				for _, c := range result {
					right = append(right, c.Name)
					fmt.Printf("cfq(t, %q),\n", strings.TrimSpace(c.Name))
				}

				// cause the test to fail
				require.FailNow(t, "expected and actual are not equal", "\n\nexpected: %#v\n\nactual: %#v", left, right)
			}
		})
	}
}
