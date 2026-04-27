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
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestResolveImportsCircular(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	lint.ZarfSchema = testutil.LoadSchema(t, "../../../../zarf.schema.json")

	b, err := os.ReadFile(filepath.Join("./testdata/import/circular/first", layout.ZarfYAML))
	require.NoError(t, err)
	pkg, err := pkgcfg.Parse(ctx, b)
	require.NoError(t, err)

	_, err = resolveImports(ctx, pkg, "./testdata/import/circular/first", "", "", []string{}, "", false)
	require.EqualError(t, err, "package testdata/import/circular/second imported in cycle by testdata/import/circular/third in component component")
}

func TestResolveImports(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	lint.ZarfSchema = testutil.LoadSchema(t, "../../../../zarf.schema.json")
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
			expectedChecksum: "e1b2c6ed6e17d37a6861046e4443440028994c38400e7089d481435a9e149df8",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b, err := os.ReadFile(filepath.Join(tc.path, layout.ZarfYAML))
			require.NoError(t, err)
			pkg, err := pkgcfg.Parse(ctx, b)
			require.NoError(t, err)

			resolvedPkg, err := resolveImports(ctx, pkg, tc.path, "", tc.flavor, []string{}, "", false)
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
