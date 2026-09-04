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
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestLoadPackage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		flavor             string
		skipVariantFilters []VariantDimension
		packageDir         string
		expectedErr        string
	}{
		{
			name:        "when all components have a flavor, inputting no flavor should error",
			flavor:      "",
			packageDir:  "package-with-flavors",
			expectedErr: fmt.Sprintf("package validation failed: %s", "package does not contain any compatible components"),
		},
		{
			name:       "flavors work",
			packageDir: "package-with-flavors",
			flavor:     "cashew",
		},
		{
			name:               "flavor and skipping flavor validation should error",
			packageDir:         "package-with-flavors",
			expectedErr:        "only one of Flavor or skipping flavor variant filtering can be set",
			flavor:             "cashew",
			skipVariantFilters: []VariantDimension{VariantFlavor},
		},
		{
			name:               "no flavor and skipping flavor validation should work",
			packageDir:         "package-with-flavors",
			flavor:             "",
			skipVariantFilters: []VariantDimension{VariantFlavor},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts := DefinitionOptions{
				Flavor:             tt.flavor,
				SkipVariantFilters: tt.skipVariantFilters,
			}
			_, err := PackageDefinition(context.Background(), filepath.Join("testdata", tt.packageDir), opts)
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateV1Beta1_FormatsValidationErrors(t *testing.T) {
	t.Parallel()

	err := validateV1Beta1(context.Background(), v1beta1.Package{}, "", "")

	require.EqualError(t, err, "package validation failed:\npackage does not contain any compatible components")
}

func TestPackageDefinitionRejectsUnsupportedRawFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		apiVersion string
	}{
		{
			name: "v1alpha1",
		},
		{
			name:       "v1beta1",
			apiVersion: "apiVersion: zarf.dev/v1beta1\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			packageYAML := tt.apiVersion + `kind: ZarfPackageConfig
metadata:
  name: raw-schema-error
components:
  - name: component
documenttaion:
  zarf.cli.openvex.json: .vex/zarf.cli.openvex.json
`
			require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), []byte(packageYAML), 0o600))

			_, err := PackageDefinition(testutil.TestContext(t), dir, DefinitionOptions{})

			var lintErr *lint.LintError
			require.ErrorAs(t, err, &lintErr)
			require.NotEmpty(t, lintErr.Findings)
		})
	}
}

func TestPackageDefinitionValidatesV1Beta1RepositoryGitReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		url    string
		commit string
		valid  bool
	}{
		{
			name:   "accepts a SHA-1 commit and HTTPS repository URL",
			url:    "https://example.com/repository.git",
			commit: "524980951ff16e19dc25232e9aea8fd693989ba6",
			valid:  true,
		},
		{
			name:   "rejects a short commit SHA",
			url:    "https://example.com/repository.git",
			commit: "5249809",
		},
		{
			name:   "rejects a non-hex commit SHA",
			url:    "https://example.com/repository.git",
			commit: "gggggggggggggggggggggggggggggggggggggggg",
		},
		{
			name:   "rejects an invalid repository URL",
			url:    "not a URI",
			commit: "524980951ff16e19dc25232e9aea8fd693989ba6",
		},
		{
			name:   "accepts an SSH repository URL",
			url:    "ssh://git@example.com/organization/repository.git",
			commit: "524980951ff16e19dc25232e9aea8fd693989ba6",
			valid:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			packageYAML := fmt.Sprintf(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: repository-schema-validation
components:
  - name: component
    repositories:
      - url: %q
        ref:
          commit: %q
`, tt.url, tt.commit)
			require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), []byte(packageYAML), 0o600))

			_, err := PackageDefinition(testutil.TestContext(t), dir, DefinitionOptions{})
			if tt.valid {
				require.NoError(t, err)
				return
			}
			var lintErr *lint.LintError
			require.ErrorAs(t, err, &lintErr)
			require.Len(t, lintErr.Findings, 1)
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
		{
			name:        "v1beta1 imported values fail imported schema validation",
			packagePath: filepath.Join("testdata", "v1beta1-imported-values-schema-invalid"),
			expectedErr: "values validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testutil.TestContext(t)
			loaded, err := Package(ctx, tt.packagePath, PackageOptions{})
			if loaded != nil {
				t.Cleanup(func() { require.NoError(t, loaded.Close()) })
			}
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

		pkg := defined.AsV1alpha1()
		require.Equal(t, v1alpha1.APIVersion, pkg.APIVersion)
		require.Equal(t, "beta-package", pkg.Metadata.Name)
		require.NotEmpty(t, pkg.Metadata.Architecture)
		require.Len(t, pkg.Components, 1)
		require.Equal(t, "first", pkg.Components[0].Name)
		require.Equal(t, []string{"nginx:1.27.0"}, pkg.Components[0].Images)
		require.Equal(t, []string{"https://github.com/zarf-dev/zarf.git"}, pkg.Components[0].Repos)

		// The v1beta1 view preserves fields with no v1alpha1 representation — here an image's source.
		// Collapsing to v1alpha1 on load (the previous approach) dropped these.
		betaPkg := defined.AsV1beta1()
		require.Equal(t, v1beta1.APIVersion, betaPkg.APIVersion)
		require.Len(t, betaPkg.Components, 1)
		require.Equal(t, "nginx:1.27.0", betaPkg.Components[0].Images[0].Name)
		require.Equal(t, "daemon", betaPkg.Components[0].Images[0].Source)
	})

	t.Run("resolves a local component config import", func(t *testing.T) {
		t.Parallel()
		defined, err := PackageDefinition(ctx, filepath.Join("testdata", "v1beta1-with-import"), DefinitionOptions{})
		require.NoError(t, err)

		pkg := defined.AsV1alpha1()
		require.Equal(t, v1alpha1.APIVersion, pkg.APIVersion)
		require.Len(t, pkg.Components, 1)
		require.Equal(t, "imported", pkg.Components[0].Name)
		require.Equal(t, []string{"nginx:1.27.0"}, pkg.Components[0].Images)
	})
}

func TestV1Beta1PackageDefinitionValuesSchemaValidation(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	dir := filepath.Join("testdata", "v1beta1-invalid-values")

	_, err := PackageDefinition(ctx, dir, DefinitionOptions{})
	require.NoError(t, err)

	_, err = Package(ctx, dir, PackageOptions{})
	require.ErrorContains(t, err, "values validation failed")

	loaded, err := Package(ctx, dir, PackageOptions{SkipValuesSchemaValidation: true})
	require.NoError(t, err)
	require.NoError(t, loaded.Close())
}

func TestPackageDefinitionDoesNotAccessSourceResources(t *testing.T) {
	ctx := testutil.TestContext(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: definition-only
values:
  files:
    - missing-values.yaml
components:
  - name: component
`), 0o600))

	definition, err := PackageDefinition(ctx, dir, DefinitionOptions{})
	require.NoError(t, err)
	require.Equal(t, "definition-only", definition.AsV1alpha1().Metadata.Name)

	_, err = Package(ctx, dir, PackageOptions{})
	require.ErrorContains(t, err, "unable to access local resource \"missing-values.yaml\"")
}

func TestLoadedPackageCloseInvalidatesResources(t *testing.T) {
	ctx := testutil.TestContext(t)
	loaded, err := Package(ctx, filepath.Join("testdata", "v1beta1-package"), PackageOptions{})
	require.NoError(t, err)
	require.NoError(t, loaded.Close())
	require.NoError(t, loaded.Close())

	_, err = loaded.Resources.Root()
	require.ErrorContains(t, err, "package resources are closed")
}

func TestV1Beta1PackageDefinitionParentChartSourceTakesPriority(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "child.yaml"), []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: child
component:
  charts:
    - name: app
      namespace: app
      local:
        path: chart
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: source-replacement
components:
  - name: app
    import:
      local:
        - path: child.yaml
    charts:
      - name: app
        namespace: app
        oci:
          url: oci://example.com/chart
          ref:
            tag: 1.0.0
`), 0o600))

	defined, err := PackageDefinition(ctx, dir, DefinitionOptions{})
	require.NoError(t, err)
	chart := defined.AsV1beta1().Components[0].Charts[0]
	require.Nil(t, chart.Local)
	require.NotNil(t, chart.OCI)
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
