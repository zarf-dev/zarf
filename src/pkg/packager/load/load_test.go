// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/feature"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestLoadPackageWithFlavors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		flavor      string
		expectedErr string
	}{
		{
			name:        "when all components have a flavor, inputting no flavor should error",
			flavor:      "",
			expectedErr: fmt.Sprintf("package validation failed: %s", "package does not contain any compatible components"),
		},
		{
			name:   "flavors work",
			flavor: "cashew",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts := DefinitionOptions{
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

func TestPackageDefinitionWithValuesSchema(t *testing.T) {
	t.Parallel()

	// Enable the values feature for these tests
	err := feature.Set([]feature.Feature{
		{
			Name:    feature.Values,
			Enabled: true,
		},
	})
	require.NoError(t, err)

	tests := []struct {
		name        string
		packagePath string
		expectedErr string
	}{
		{
			name:        "valid values pass schema validation",
			packagePath: filepath.Join("testdata", "package-with-values-schema"),
		},
		{
			name:        "invalid values fail schema validation",
			packagePath: filepath.Join("testdata", "package-with-invalid-values"),
			expectedErr: "values validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testutil.TestContext(t)
			opts := DefinitionOptions{}
			_, err := PackageDefinition(ctx, tt.packagePath, opts)
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestV1Beta1PackageDefinition(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	t.Run("loads and validates, exposing both a v1alpha1 and a faithful v1beta1 view", func(t *testing.T) {
		t.Parallel()
		defined, err := PackageDefinition(ctx, filepath.Join("testdata", "v1beta1-package"), DefinitionOptions{})
		require.NoError(t, err)
		require.Equal(t, v1beta1.APIVersion, defined.OriginalAPIVersion())

		pkg, err := defined.AsV1alpha1()
		require.NoError(t, err)
		require.Equal(t, v1alpha1.APIVersion, pkg.APIVersion)
		require.Equal(t, "beta-package", pkg.Metadata.Name)
		require.NotEmpty(t, pkg.Metadata.Architecture)
		require.Len(t, pkg.Components, 1)
		require.Equal(t, "first", pkg.Components[0].Name)
		require.Equal(t, []string{"nginx:1.27.0"}, pkg.Components[0].Images)
		require.Equal(t, []string{"https://github.com/zarf-dev/zarf.git"}, pkg.Components[0].Repos)
		require.Empty(t, defined.ImportedSchemas)

		// The v1beta1 view preserves fields with no v1alpha1 representation — here an image's source.
		// Collapsing to v1alpha1 on load (the previous approach) dropped these.
		betaPkg, err := defined.AsV1beta1()
		require.NoError(t, err)
		require.Equal(t, v1beta1.APIVersion, betaPkg.APIVersion)
		require.Len(t, betaPkg.Components, 1)
		require.Equal(t, "nginx:1.27.0", betaPkg.Components[0].Images[0].Name)
		require.Equal(t, "daemon", betaPkg.Components[0].Images[0].Source)
	})

	t.Run("resolves a local component config import", func(t *testing.T) {
		t.Parallel()
		defined, err := PackageDefinition(ctx, filepath.Join("testdata", "v1beta1-with-import"), DefinitionOptions{})
		require.NoError(t, err)

		pkg, err := defined.AsV1alpha1()
		require.NoError(t, err)
		require.Equal(t, v1alpha1.APIVersion, pkg.APIVersion)
		require.Len(t, pkg.Components, 1)
		require.Equal(t, "imported", pkg.Components[0].Name)
		require.Equal(t, []string{"nginx:1.27.0"}, pkg.Components[0].Images)
	})
}

func TestPackageDefinitionErrors(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	t.Run("returns error for non-existent package path", func(t *testing.T) {
		t.Parallel()
		_, err := PackageDefinition(ctx, filepath.Join(t.TempDir(), "does-not-exist"), DefinitionOptions{})
		require.Error(t, err)
	})

	t.Run("returns error when zarf.yaml contains invalid YAML", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "zarf.yaml"), []byte("this: is: not: valid: yaml: ["), 0o600))
		_, err := PackageDefinition(ctx, dir, DefinitionOptions{})
		require.Error(t, err)
	})

	t.Run("returns error when a component import path does not exist", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		zarfYAML := `kind: ZarfPackageConfig
metadata:
  name: test
components:
  - name: test
    import:
      path: ./does-not-exist
`
		require.NoError(t, os.WriteFile(filepath.Join(dir, "zarf.yaml"), []byte(zarfYAML), 0o600))
		_, err := PackageDefinition(ctx, dir, DefinitionOptions{})
		require.ErrorContains(t, err, "does-not-exist")
	})

	t.Run("returns error when a required package template variable is not set", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		zarfYAML := `kind: ZarfPackageConfig
metadata:
  name: test
components:
  - name: test
    required: true
    actions:
      onCreate:
        before:
          - cmd: "###ZARF_PKG_TMPL_MYVAR###"
`
		require.NoError(t, os.WriteFile(filepath.Join(dir, "zarf.yaml"), []byte(zarfYAML), 0o600))
		_, err := PackageDefinition(ctx, dir, DefinitionOptions{
			SetVariables: map[string]string{}, // non-nil triggers fillActiveTemplate; MYVAR is absent
		})
		require.ErrorContains(t, err, "MYVAR")
	})
}
