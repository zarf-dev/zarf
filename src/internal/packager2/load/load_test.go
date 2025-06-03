// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestLoadPackageWithFlavors(t *testing.T) {
	t.Parallel()
	lint.ZarfSchema = testutil.LoadSchema(t, "../../../../zarf.schema.json")

	tests := []struct {
		name        string
		flavor      string
		expectedErr string
	}{
		{
			name:        "when all components have a flavor, inputting no flavor should error",
			flavor:      "",
			expectedErr: fmt.Sprintf("package validation failed: %s", lint.PkgValidateErrNoComponents),
		},
		{
			name:   "flavors work",
			flavor: "cashew",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts := DefinitionOpts{
				Flavor: tt.flavor,
			}
			_, err := PackageDefinition(context.Background(), filepath.Join("testdata", "package-with-flavors"), opts)
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestPackageUsesFlavor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pkg      v1alpha1.ZarfPackage
		flavor   string
		expected bool
	}{
		{
			name: "when flavor is not set",
			pkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "do-nothing",
					},
					{
						Name: "do-nothing-flavored",
						Only: v1alpha1.ZarfComponentOnlyTarget{
							Flavor: "cashew",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "when flavor is not used",
			pkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "do-nothing",
					},
				},
			},
			flavor:   "cashew",
			expected: false,
		},
		{
			name: "when flavor is used",
			pkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "do-nothing",
						Only: v1alpha1.ZarfComponentOnlyTarget{
							Flavor: "cashew",
						},
					},
				},
			},
			flavor:   "cashew",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, hasFlavoredComponent(tt.pkg, tt.flavor))
		})
	}
}
