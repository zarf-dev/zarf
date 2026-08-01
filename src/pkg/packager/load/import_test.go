// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pkgoci "github.com/defenseunicorns/pkg/oci"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	ocistore "oras.land/oras-go/v2/content/oci"

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

	_, err = resolveImports(ctx, pkg, "./testdata/import/circular/first", "", "", []string{}, "", false, types.RemoteOptions{}, &importResources{})
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

			resources := importResources{}
			resolvedPkg, err := resolveImports(ctx, pkg, tc.path, "", tc.flavor, []string{}, "", false, types.RemoteOptions{}, &resources)
			require.NoError(t, err)

			b, err = os.ReadFile(filepath.Join(tc.path, "expected.yaml"))
			require.NoError(t, err)
			expectedPkg, err := pkgcfg.Parse(ctx, b)

			require.NoError(t, err)
			// Values are resolved separately from the package object; their artifact
			// paths are canonicalized only by PackageDefinition.
			expectedPkg.Values = resolvedPkg.Values
			require.Equal(t, expectedPkg, resolvedPkg)
			testutil.RequireNoBackslashInPackagePaths(t, resolvedPkg)
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
	resources := importResources{}
	_, err := resolveImports(ctx, pkg, "./testdata/import/values/duplicate-consecutive",
		"", "", []string{}, "", false, types.RemoteOptions{}, &resources)
	require.NoError(t, err)
	resolved, err := resources.resolve(ctx, "")
	require.NoError(t, err)
	require.False(t, resolved.HasValues)
	require.Empty(t, deduplicateSources(resources.values))
}

func TestFetchOCISkeletonValuesMissingDeclaredLayer(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	tests := []struct {
		name        string
		pkg         v1alpha1.ZarfPackage
		expectedErr string
	}{
		{
			name: "values",
			pkg: v1alpha1.ZarfPackage{Values: v1alpha1.ZarfValues{
				Files: []string{layout.ValuesYAML},
			}},
			expectedErr: "declares values but does not contain \"values.yaml\"",
		},
		{
			name: "schema",
			pkg: v1alpha1.ZarfPackage{Values: v1alpha1.ZarfValues{
				Schema: layout.ValuesSchema,
			}},
			expectedErr: "declares values schema but does not contain \"values.schema.json\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rootDesc := ocispec.Descriptor{Digest: digest.FromString(tt.name)}
			_, err := fetchOCISkeletonValues(ctx, nil, &pkgoci.Manifest{}, rootDesc, "oci://example.com/skeleton", t.TempDir(), tt.pkg)
			require.ErrorContains(t, err, tt.expectedErr)
		})
	}
}

func TestFetchOCISkeletonValuesReadsSources(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	cachePath := t.TempDir()
	cache := filepath.Join(cachePath, "oci")
	store, err := ocistore.New(cache)
	require.NoError(t, err)

	contents := []byte("from: skeleton\n")
	desc := ocispec.Descriptor{
		Digest:      digest.FromBytes(contents),
		Size:        int64(len(contents)),
		Annotations: map[string]string{ocispec.AnnotationTitle: layout.ValuesYAML},
	}
	require.NoError(t, store.Push(ctx, desc, bytes.NewReader(contents)))

	rootDesc := ocispec.Descriptor{Digest: digest.FromString("skeleton-root")}
	pkg := v1alpha1.ZarfPackage{Values: v1alpha1.ZarfValues{Files: []string{layout.ValuesYAML}}}
	manifest := &pkgoci.Manifest{Manifest: ocispec.Manifest{Layers: []ocispec.Descriptor{desc}}}
	got, err := fetchOCISkeletonValues(ctx, nil, manifest, rootDesc, "oci://example.com/skeleton", cachePath, pkg)
	require.NoError(t, err)

	require.Len(t, got.values, 1)
	require.Equal(t, layout.ValuesYAML, got.values[0].Name)
	require.Equal(t, contents, got.values[0].Data)
	require.NoDirExists(t, filepath.Join(cache, "packages"))
}

func TestFetchOCISkeletonValuesLayerSizeLimit(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	cachePath := t.TempDir()
	cache := filepath.Join(cachePath, "oci")
	store, err := ocistore.New(cache)
	require.NoError(t, err)

	contents := bytes.Repeat([]byte("a"), maxOCISkeletonValuesLayerSize)
	desc := ocispec.Descriptor{
		Digest:      digest.FromBytes(contents),
		Size:        int64(len(contents)),
		Annotations: map[string]string{ocispec.AnnotationTitle: layout.ValuesYAML},
	}
	require.NoError(t, store.Push(ctx, desc, bytes.NewReader(contents)))

	pkg := v1alpha1.ZarfPackage{Values: v1alpha1.ZarfValues{Files: []string{layout.ValuesYAML}}}
	manifest := &pkgoci.Manifest{Manifest: ocispec.Manifest{Layers: []ocispec.Descriptor{desc}}}
	resolved, err := fetchOCISkeletonValues(ctx, nil, manifest, ocispec.Descriptor{Digest: digest.FromString("limit")}, "oci://example.com/skeleton", cachePath, pkg)
	require.NoError(t, err)
	require.Len(t, resolved.values, 1)
	require.Len(t, resolved.values[0].Data, maxOCISkeletonValuesLayerSize)

	overLimit := desc
	overLimit.Size++
	manifest = &pkgoci.Manifest{Manifest: ocispec.Manifest{Layers: []ocispec.Descriptor{overLimit}}}
	_, err = fetchOCISkeletonValues(ctx, nil, manifest, ocispec.Descriptor{Digest: digest.FromString("over-limit")}, "oci://example.com/skeleton", cachePath, pkg)
	require.ErrorContains(t, err, "exceeds the 1048576 byte limit")

	inconsistent := desc
	inconsistent.Size--
	manifest = &pkgoci.Manifest{Manifest: ocispec.Manifest{Layers: []ocispec.Descriptor{inconsistent}}}
	_, err = fetchOCISkeletonValues(ctx, nil, manifest, ocispec.Descriptor{Digest: digest.FromString("inconsistent")}, "oci://example.com/skeleton", cachePath, pkg)
	require.ErrorContains(t, err, "inconsistent descriptor size")
}

func sourceNames(sources []value.Source) []string {
	names := make([]string, len(sources))
	for i, source := range sources {
		names[i] = source.Name
	}
	return names
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

			resources := importResources{}
			resolved, err := resolveImports(ctx, pkg, tc.path, "", "", []string{}, "", false, types.RemoteOptions{}, &resources)
			require.NoError(t, err)

			merged, err := resources.resolve(ctx, resolved.Values.Schema)
			require.NoError(t, err)
			require.Equal(t, tc.expected, merged.Values)
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
			expectedSchemas: []string{"child-values.schema.json"},
			expectedParent:  "",
		},
		{
			name:            "child schema is collected when parent also has a schema",
			path:            "./testdata/import/values/schema-parent-wins",
			expectedSchemas: []string{"parent-values.schema.json", "child-values.schema.json"},
			expectedParent:  "parent-values.schema.json",
		},
		{
			name: "schemas are collected transitively through 3-level deep imports",
			path: "./testdata/import/values/schema-deep",
			// middle's own schema comes first; bottom's schema (from middle's imports) comes second
			expectedSchemas: []string{
				"middle-values.schema.json",
				"bottom-values.schema.json",
			},
			expectedParent: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b, err := os.ReadFile(filepath.Join(tc.path, layout.ZarfYAML))
			require.NoError(t, err)
			pkg, err := pkgcfg.Parse(ctx, b)
			require.NoError(t, err)

			resources := importResources{}
			resolved, err := resolveImports(ctx, pkg, tc.path, "", "", []string{}, "", false, types.RemoteOptions{}, &resources)
			require.NoError(t, err)

			require.Equal(t, tc.expectedSchemas, sourceNames(resources.schemas))
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
