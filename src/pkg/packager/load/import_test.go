// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/pkgcfg"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
)

func TestResolveImportsCircular(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	b, err := os.ReadFile(filepath.Join("./testdata/import/circular/first", layout.ZarfYAML))
	require.NoError(t, err)
	pkg, err := pkgcfg.Parse(ctx, b)
	require.NoError(t, err)

	_, err = resolveImports(ctx, pkg, "./testdata/import/circular/first", "", "", []string{}, "", false, types.RemoteOptions{})
	require.EqualError(t, err, "package testdata/import/circular/second imported in cycle by testdata/import/circular/third in component component")
}

func TestResolveImports(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	testCases := []struct {
		name   string
		path   string
		flavor string
	}{
		{
			name: "two zarf.yaml files import each other",
			path: "./testdata/import/import-each-other",
		},
		{
			name: "variables and constants are resolved correctly",
			path: "./testdata/import/variables",
		},
		{
			name: "values files from a single import are merged before parent values",
			path: "./testdata/import/values/basic",
		},
		{
			name: "values files from nested imports preserve deepest-first precedence order",
			path: "./testdata/import/values/precedence-order",
		},
		{
			name: "values files from multiple sibling imports preserve left-to-right order",
			path: "./testdata/import/values/multiple-imports",
		},
		{
			name: "duplicate values file paths from consecutive imports are deduplicated",
			path: "./testdata/import/values/duplicate-consecutive",
		},
		{
			name: "duplicate values file paths from non-consecutive imports are deduplicated",
			path: "./testdata/import/values/duplicate-interleaved",
		},
		{
			name: "an empty parent schema is kept even when an imported package has one",
			path: "./testdata/import/values/schema-parent-empty",
		},
		{
			name: "a parent schema takes precedence over an imported package's schema",
			path: "./testdata/import/values/schema-parent-wins",
		},
		{
			name: "two separate chains of imports importing a common file",
			path: "./testdata/import/branch",
		},
		{
			name:   "flavor is preserved when importing",
			path:   "./testdata/import/flavor",
			flavor: "pistachio",
		},
		{
			name: "chart version and url properties are not overridden",
			path: "./testdata/import/chart",
		},
		{
			name: "archives work as expected",
			path: "./testdata/import/archives",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b, err := os.ReadFile(filepath.Join(tc.path, layout.ZarfYAML))
			require.NoError(t, err)
			pkg, err := pkgcfg.Parse(ctx, b)
			require.NoError(t, err)

			resolvedPkg, err := resolveImports(ctx, pkg, tc.path, "", tc.flavor, []string{}, "", false, types.RemoteOptions{})
			require.NoError(t, err)

			b, err = os.ReadFile(filepath.Join(tc.path, "expected.yaml"))
			require.NoError(t, err)
			expectedPkg, err := pkgcfg.Parse(ctx, b)

			require.NoError(t, err)
			require.Equal(t, expectedPkg, resolvedPkg)
		})
	}
}

func TestResolveImportsValueMerge(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	testCases := []struct {
		name     string
		path     string
		expected value.Values
	}{
		{
			name: "nested imports apply deepest-first so parent overrides inner values",
			path: "./testdata/import/values/precedence-order",
			expected: value.Values{
				"shared":      "top",
				"top-only":    "present",
				"middle-only": "present",
				"bottom-only": "present",
			},
		},
		{
			name: "non-consecutive duplicate imports are deduplicated so the later sibling's value wins",
			path: "./testdata/import/values/duplicate-interleaved",
			expected: value.Values{
				"origin":      "b",
				"a-only":      "present",
				"b-only":      "present",
				"parent-only": "present",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b, err := os.ReadFile(filepath.Join(tc.path, layout.ZarfYAML))
			require.NoError(t, err)
			pkg, err := pkgcfg.Parse(ctx, b)
			require.NoError(t, err)

			resolved, err := resolveImports(ctx, pkg, tc.path, "", "", []string{}, "", false, types.RemoteOptions{})
			require.NoError(t, err)

			absPaths := make([]string, len(resolved.Values.Files))
			for i, f := range resolved.Values.Files {
				absPaths[i] = filepath.Join(tc.path, f)
			}

			merged, err := value.ParseFiles(ctx, absPaths, value.ParseFilesOptions{})
			require.NoError(t, err)
			require.Equal(t, tc.expected, merged)
		})
	}
}

func TestValidateComponentCompose(t *testing.T) {
	t.Parallel()

	abs, err := filepath.Abs(".")
	require.NoError(t, err)

	tests := []struct {
		name         string
		component    v1alpha1.ZarfComponent
		expectedErrs []string
	}{
		{
			name: "valid path",
			component: v1alpha1.ZarfComponent{
				Name: "component1",
				Import: v1alpha1.ZarfComponentImport{
					Path: "relative/path",
				},
			},
			expectedErrs: nil,
		},
		{
			name: "valid URL",
			component: v1alpha1.ZarfComponent{
				Name: "component2",
				Import: v1alpha1.ZarfComponentImport{
					URL: "oci://example.com/package:v0.0.1",
				},
			},
			expectedErrs: nil,
		},
		{
			name: "neither path nor URL provided",
			component: v1alpha1.ZarfComponent{
				Name: "neither",
			},
			expectedErrs: []string{
				"neither a path nor a URL was provided",
			},
		},
		{
			name: "both path and URL provided",
			component: v1alpha1.ZarfComponent{
				Name: "both",
				Import: v1alpha1.ZarfComponentImport{
					Path: "relative/path",
					URL:  "https://example.com",
				},
			},
			expectedErrs: []string{
				"both a path and a URL were provided",
			},
		},
		{
			name: "absolute path provided",
			component: v1alpha1.ZarfComponent{
				Name: "abs-path",
				Import: v1alpha1.ZarfComponentImport{
					Path: abs,
				},
			},
			expectedErrs: []string{
				"path cannot be an absolute path",
			},
		},
		{
			name: "invalid URL provided",
			component: v1alpha1.ZarfComponent{
				Name: "bad-url",
				Import: v1alpha1.ZarfComponentImport{
					URL: "https://example.com",
				},
			},
			expectedErrs: []string{
				"URL is not a valid OCI URL",
			},
		},
		{
			name: "package template path provided",
			component: v1alpha1.ZarfComponent{
				Name: "template",
				Import: v1alpha1.ZarfComponentImport{
					Path: "###ZARF_PKG_TMPL_PATH###",
				},
			},
			expectedErrs: []string{
				"package templates are not supported for import path or URL",
			},
		},
		{
			name: "package template URL provided",
			component: v1alpha1.ZarfComponent{
				Name: "template",
				Import: v1alpha1.ZarfComponentImport{
					URL: "oci://registry.com/my-image:###ZARF_PKG_TMPL_TAG###",
				},
			},
			expectedErrs: []string{
				"package templates are not supported for import path or URL",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateComponentCompose(tt.component)
			if tt.expectedErrs == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			errs := strings.Split(err.Error(), "\n")
			require.ElementsMatch(t, tt.expectedErrs, errs)
		})
	}
}

func TestCompatibleComponent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		component      v1alpha1.ZarfComponent
		arch           string
		flavor         string
		expectedResult bool
	}{
		{
			name: "set architecture and set flavor",
			component: v1alpha1.ZarfComponent{
				Only: v1alpha1.ZarfComponentOnlyTarget{
					Cluster: v1alpha1.ZarfComponentOnlyCluster{
						Architecture: "amd64",
					},
					Flavor: "foo",
				},
			},
			arch:           "amd64",
			flavor:         "foo",
			expectedResult: true,
		},
		{
			name: "set architecture and empty flavor",
			component: v1alpha1.ZarfComponent{
				Only: v1alpha1.ZarfComponentOnlyTarget{
					Cluster: v1alpha1.ZarfComponentOnlyCluster{
						Architecture: "amd64",
					},
					Flavor: "",
				},
			},
			arch:           "amd64",
			flavor:         "foo",
			expectedResult: true,
		},
		{
			name: "empty architecture and set flavor",
			component: v1alpha1.ZarfComponent{
				Only: v1alpha1.ZarfComponentOnlyTarget{
					Cluster: v1alpha1.ZarfComponentOnlyCluster{
						Architecture: "",
					},
					Flavor: "foo",
				},
			},
			arch:           "amd64",
			flavor:         "foo",
			expectedResult: true,
		},
		{
			name: "architecture miss match",
			component: v1alpha1.ZarfComponent{
				Only: v1alpha1.ZarfComponentOnlyTarget{
					Cluster: v1alpha1.ZarfComponentOnlyCluster{
						Architecture: "arm",
					},
					Flavor: "foo",
				},
			},
			arch:           "amd64",
			flavor:         "foo",
			expectedResult: false,
		},
		{
			name: "flavor miss match",
			component: v1alpha1.ZarfComponent{
				Only: v1alpha1.ZarfComponentOnlyTarget{
					Cluster: v1alpha1.ZarfComponentOnlyCluster{
						Architecture: "arm",
					},
					Flavor: "bar",
				},
			},
			arch:           "amd64",
			flavor:         "foo",
			expectedResult: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := compatibleComponent(tt.component, tt.arch, tt.flavor)
			require.Equal(t, tt.expectedResult, result)
		})
	}
}
