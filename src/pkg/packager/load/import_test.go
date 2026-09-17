// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/api/convert"
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
	pkg, err := pkgcfg.ParseAs(ctx, b, pkgcfg.V1Alpha1)
	require.NoError(t, err)

	_, _, err = resolveImports(ctx, convert.PackageFromV1alpha1(pkg), "./testdata/import/circular/first", "", "", []string{}, "", false, types.RemoteOptions{})
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
			expectedChecksum: "56851d4445f1e53797812c3480716a9afb23a2f35ee1710ab8fcc146058baa95",
		},
		{
			name:             "variables and constants are resolved correctly",
			path:             "./testdata/import/variables",
			expectedChecksum: "65bcb19b6a36e43621f6789875f30c40abc131549837051ea4a6a4da8e3e069a",
		},
		{
			name:             "values files from nested imports preserve deepest-first precedence order",
			path:             "./testdata/import/values/precedence-order",
			expectedChecksum: "8ceda625711717294695cd02c1aebc6f878292a4a12ea42bf05e2da875ec2c63",
		},
		{
			name:             "values files from multiple sibling imports preserve left-to-right order",
			path:             "./testdata/import/values/multiple-imports",
			expectedChecksum: "5659e6ea458dbfbcaf0eb16e8955e11abeffd8dd1cd4202f58a85969b6523378",
		},
		{
			name:             "duplicate values file paths from consecutive imports are deduplicated",
			path:             "./testdata/import/values/duplicate-consecutive",
			expectedChecksum: "24b9aee0c2aaf66b3efe0337a5cd08954da98ced28e3ba1e53c166de5d474f3a",
		},
		{
			name:             "duplicate values file paths from non-consecutive imports are deduplicated",
			path:             "./testdata/import/values/duplicate-interleaved",
			expectedChecksum: "341908959e2753328f53cb79345d9e1d509d01729fb3a8bbce83faed4e55b3ea",
		},
		{
			name:             "an empty parent schema is kept even when an imported package has one",
			path:             "./testdata/import/values/schema-parent-empty",
			expectedChecksum: "8d5d599a87cf5bcf69e053426580f5902329acab826c1d9b52a77dfb95a95f9d",
		},
		{
			name:             "a parent schema takes precedence over an imported package's schema",
			path:             "./testdata/import/values/schema-parent-wins",
			expectedChecksum: "6a03bf06329b743ada9500fd64664ebd111f5863e2a8b1ed5aae7c5114b5c62a",
		},
		{
			name:             "two separate chains of imports importing a common file",
			path:             "./testdata/import/branch",
			expectedChecksum: "a75b6079f2b090dd3d124f9d0e1f854ace92071503999609d18c759eee519816",
		},
		{
			name:             "flavor is preserved when importing",
			path:             "./testdata/import/flavor",
			flavor:           "pistachio",
			expectedChecksum: "a43253815628aebcb506c97716cc370c33cbfe7b6f532f74a9b60623c0140266",
		},
		{
			name:             "chart version and url properties are not overridden",
			path:             "./testdata/import/chart",
			expectedChecksum: "e70086ac46b963cfd923f4d68d7df6389e79b60d146a31a43db64d8c4124f65a",
		},
		{
			name:             "archives work as expected",
			path:             "./testdata/import/archives",
			expectedChecksum: "ba81af688158cbe6e3f9b10ffb27c82f38f42ff119301cf2a117ecc5d1f90c68",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b, err := os.ReadFile(filepath.Join(tc.path, layout.ZarfYAML))
			require.NoError(t, err)
			pkg, err := pkgcfg.ParseAs(ctx, b, pkgcfg.V1Alpha1)
			require.NoError(t, err)

			resolvedPkg, _, err := resolveImports(ctx, convert.PackageFromV1alpha1(pkg), tc.path, "", tc.flavor, []string{}, "", false, types.RemoteOptions{})
			require.NoError(t, err)

			b, err = os.ReadFile(filepath.Join(tc.path, "expected.yaml"))
			require.NoError(t, err)
			expectedPkg, err := pkgcfg.ParseAs(ctx, b, pkgcfg.V1Alpha1)

			require.NoError(t, err)
			require.Equal(t, convert.PackageFromV1alpha1(expectedPkg), resolvedPkg)
			resolvedLegacy := convert.PackageToV1alpha1(resolvedPkg)
			testutil.RequireNoBackslashInPackagePaths(t, resolvedLegacy)
			require.Equal(t, tc.expectedChecksum, testutil.ChecksumZarfYAMLContent(t, resolvedLegacy), "resolved zarf.yaml checksum drift — package would differ across build hosts")
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
	resolved, _, err := resolveImports(ctx, convert.PackageFromV1alpha1(pkg), "./testdata/import/values/duplicate-consecutive",
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
			pkg, err := pkgcfg.ParseAs(ctx, b, pkgcfg.V1Alpha1)
			require.NoError(t, err)

			resolved, _, err := resolveImports(ctx, convert.PackageFromV1alpha1(pkg), tc.path, "", "", []string{}, "", false, types.RemoteOptions{})
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

func TestResolveImportsSchemaCollection(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	testCases := []struct {
		name            string
		path            string
		expectedSchemas []string
		expectedParent  string
	}{
		{
			name:            "child schema is collected when parent has no schema",
			path:            "./testdata/import/values/schema-parent-empty",
			expectedSchemas: []string{"import/child-values.schema.json"},
			expectedParent:  "",
		},
		{
			name:            "child schema is collected when parent also has a schema",
			path:            "./testdata/import/values/schema-parent-wins",
			expectedSchemas: []string{"import/child-values.schema.json"},
			expectedParent:  "parent-values.schema.json",
		},
		{
			name: "schemas are collected transitively through 3-level deep imports",
			path: "./testdata/import/values/schema-deep",
			// middle's own schema comes first; bottom's schema (from middle's imports) comes second
			expectedSchemas: []string{
				"middle/middle-values.schema.json",
				"middle/bottom/bottom-values.schema.json",
			},
			expectedParent: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b, err := os.ReadFile(filepath.Join(tc.path, layout.ZarfYAML))
			require.NoError(t, err)
			pkg, err := pkgcfg.ParseAs(ctx, b, pkgcfg.V1Alpha1)
			require.NoError(t, err)

			resolved, importedSchemas, err := resolveImports(ctx, convert.PackageFromV1alpha1(pkg), tc.path, "", "", []string{}, "", false, types.RemoteOptions{})
			require.NoError(t, err)

			require.Equal(t, tc.expectedSchemas, importedSchemas)
			require.Equal(t, tc.expectedParent, resolved.Values.Schema)
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

			err := validateComponentCompose(convert.PackageFromV1alpha1(v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{tt.component}}).Components[0])
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

			result := compatibleComponent(convert.PackageFromV1alpha1(v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{tt.component}}).Components[0], tt.arch, tt.flavor)
			require.Equal(t, tt.expectedResult, result)
		})
	}
}
