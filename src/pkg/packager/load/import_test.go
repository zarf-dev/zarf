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
		name             string
		path             string
		flavor           string
		expectedChecksum string
	}{
		{
			name:             "two zarf.yaml files import each other",
			path:             "./testdata/import/import-each-other",
			expectedChecksum: "1ba733591d28761e89f6a576593cb3a09000f3d6a699212214a5aceaf74455c0",
		},
		{
			name:             "variables and constants are resolved correctly",
			path:             "./testdata/import/variables",
			expectedChecksum: "41e3bdf823769eb2c13079191179ee723a6b8550c5492a8668233de8b77e03da",
		},
		{
			name:             "values files from nested imports preserve deepest-first precedence order",
			path:             "./testdata/import/values/precedence-order",
			expectedChecksum: "1269606562ec5f7065169f601f0c3d7dff4707ec613216050f513b4ea0161849",
		},
		{
			name:             "values files from multiple sibling imports preserve left-to-right order",
			path:             "./testdata/import/values/multiple-imports",
			expectedChecksum: "06b9e2cbc17034b371efd57f75bb299e4849b8644f4fdde257f778ff2c48fb01",
		},
		{
			name:             "duplicate values file paths from consecutive imports are deduplicated",
			path:             "./testdata/import/values/duplicate-consecutive",
			expectedChecksum: "9698b8c12900a862f370d12bf240c721a62b5508fc26a34af33cd787261eaca3",
		},
		{
			name:             "duplicate values file paths from non-consecutive imports are deduplicated",
			path:             "./testdata/import/values/duplicate-interleaved",
			expectedChecksum: "23c92f2941e30e5717546a0f4d3cd76ced28787346d4681936d0acb1df204255",
		},
		{
			name:             "an empty parent schema is kept even when an imported package has one",
			path:             "./testdata/import/values/schema-parent-empty",
			expectedChecksum: "63135e84455ebf25324cbe847d2c778da2adecee477d7c0172744b9825e8615f",
		},
		{
			name:             "a parent schema takes precedence over an imported package's schema",
			path:             "./testdata/import/values/schema-parent-wins",
			expectedChecksum: "e43e13f0f064be03780d69f2772caed374e9f4c30ddfd8c0f09dcb0461a6e53d",
		},
		{
			name:             "two separate chains of imports importing a common file",
			path:             "./testdata/import/branch",
			expectedChecksum: "5213106f8fb4a752a44fc2fd370c06335c31069113d9148ad627082510e9a4ef",
		},
		{
			name:             "flavor is preserved when importing",
			path:             "./testdata/import/flavor",
			flavor:           "pistachio",
			expectedChecksum: "9c60125954b1b38a5947401411b87cde3d586e5ff8eef03bcc37dae1e24ab08e",
		},
		{
			name:             "chart version and url properties are not overridden",
			path:             "./testdata/import/chart",
			expectedChecksum: "cc62674a6faa1c9685aac0c8266dacec3b91e0a9466c8d1ce3664e019348b43a",
		},
		{
			name:             "archives work as expected",
			path:             "./testdata/import/archives",
			expectedChecksum: "9601cb578d72727bba116d008a23f63ac6dd40c3a685e1d790d376469792db5a",
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
			testutil.RequireNoBackslashInPackagePaths(t, resolvedPkg)
			require.Equal(t, tc.expectedChecksum, testutil.ChecksumZarfYAMLContent(t, resolvedPkg), "resolved zarf.yaml checksum drift — package would differ across build hosts")
		})
	}
}

func TestResolveImportsDedupNormalization(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	// Imports rebase through makePathRelativeTo (which Cleans paths), but the parent's
	// own Values.Files entries are appended verbatim. Without normalization in the
	// dedup loop, syntactic variants like "./foo.yaml" and "foo.yaml" survive as two
	// entries pointing at the same file. Verify the loop normalizes both forms to one.
	pkg := v1alpha1.ZarfPackage{
		Kind:     v1alpha1.ZarfPackageConfig,
		Metadata: v1alpha1.ZarfMetadata{Name: "parent"},
		Values: v1alpha1.ZarfValues{
			Files: []string{"./parent-values.yaml", "parent-values.yaml"},
		},
		Components: []v1alpha1.ZarfComponent{{Name: "standalone"}},
	}

	// Reuse an existing fixture's directory only as the on-disk anchor — resolveImports
	// stats the path but does not re-parse zarf.yaml when pkg is passed in.
	resolved, err := resolveImports(ctx, pkg, "./testdata/import/values/duplicate-consecutive",
		"", "", []string{}, "", false, types.RemoteOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"parent-values.yaml"}, resolved.Values.Files)
}

func TestMakePathRelativeTo(t *testing.T) {
	t.Parallel()

	absPath, err := filepath.Abs(filepath.Join("abs", "data.txt"))
	require.NoError(t, err)

	tests := []struct {
		name       string
		path       string
		relativeTo string
		expected   string
	}{
		{
			name:       "multi-segment relative path joins with forward slashes",
			path:       "nested/data.txt",
			relativeTo: "import",
			expected:   "import/nested/data.txt",
		},
		{
			name:       "single-segment relative path joins with forward slash",
			path:       "data.txt",
			relativeTo: "import",
			expected:   "import/data.txt",
		},
		{
			name:       "URL passes through untouched",
			path:       "oci://example.com/pkg:v1",
			relativeTo: "import",
			expected:   "oci://example.com/pkg:v1",
		},
		{
			name:       "absolute path passes through untouched",
			path:       absPath,
			relativeTo: "import",
			expected:   absPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := makePathRelativeTo(tt.path, tt.relativeTo)
			require.Equal(t, tt.expected, got)
			if !filepath.IsAbs(tt.path) {
				require.Falsef(t, strings.ContainsRune(got, '\\'), "result %q contains a backslash", got)
			}
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
