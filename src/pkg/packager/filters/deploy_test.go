package filters

import (
	"fmt"
	"strings"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cfq(t *testing.T, q string) types.ZarfComponent {
	c := types.ZarfComponent{
		Name: q,
	}

	// conditions := strings.Split(q[firstEq+1:], "&&")
	conditions := strings.Split(q, "&&")
	for _, cond := range conditions {
		cond = strings.TrimSpace(cond)
		switch cond {
		case "default=true":
			c.Default = true
		case "default=false":
			c.Default = false
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
			require.Fail(t, "unknown condition", "unknown condition %q", cond)
		}
	}

	return c
}

func componentMatrix(_ *testing.T) []types.ZarfComponent {
	var components []types.ZarfComponent

	defaultValues := []bool{true, false}
	requiredValues := []interface{}{nil, true, false}
	groupValues := []string{"", "foo", "foo", "bar", "bar"}

	// how many combinations of the above values are there?
	// 2 * 3 * 5 = 30

	for _, defaultValue := range defaultValues {
		for _, requiredValue := range requiredValues {
			for _, groupValue := range groupValues {
				c := types.ZarfComponent{
					Name:            fmt.Sprintf("default=%t && required=%v && group=%s", defaultValue, requiredValue, groupValue),
					Default:         defaultValue,
					DeprecatedGroup: groupValue,
				}

				if requiredValue != nil {
					c.DeprecatedRequired = helpers.BoolPtr(requiredValue.(bool))
				}

				components = append(components, c)
			}
		}
	}

	return components
}

func TestDeployFilter_Apply(t *testing.T) {
	testCases := map[string]struct {
		pkg  types.ZarfPackage
		want []types.ZarfComponent
	}{
		"Test when version is less than v0.33.0": {
			pkg: types.ZarfPackage{
				Build: types.ZarfBuildData{
					Version: "v0.32.0",
				},
				Components: componentMatrix(t),
			},
			want: []types.ZarfComponent{
				cfq(t, "default=true && required=<nil> && group="),
				cfq(t, "default=true && required=<nil> && group=foo"),
				cfq(t, "default=true && required=<nil> && group=bar"),
				cfq(t, "default=true && required=true && group="),
				cfq(t, "default=true && required=false && group="),
				cfq(t, "default=false && required=true && group="),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			filter := &deploymentFilter{
				// we dont currently unit test interactive CLI behavior
				isInteractive: false,
			}

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
					fmt.Printf("cfq(t, \"%s\"),\n", strings.TrimSpace(c.Name))
				}

				// cause the test to fail
				require.Equal(t, left, right)
			}
		})
	}
}
