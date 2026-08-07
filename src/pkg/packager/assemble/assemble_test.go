// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package assemble

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	goyaml "github.com/goccy/go-yaml"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/pkgcfg"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/test/testutil"
	_ "modernc.org/sqlite"
)

func TestGetChecksum(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	files := map[string]string{
		"empty.txt":                "",
		"foo":                      "bar",
		"zarf.yaml":                "Zarf Yaml Data",
		"checksums.txt":            "Old Checksum Data",
		"nested/directory/file.md": "nested",
	}
	for k, v := range files {
		err := os.MkdirAll(filepath.Join(tmpDir, filepath.Dir(k)), 0o700)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tmpDir, k), []byte(v), 0o600)
		require.NoError(t, err)
	}

	checksumContent, checksumHash, err := getChecksum(tmpDir)
	require.NoError(t, err)

	expectedContent := `233562de1a0288b139c4fa40b7d189f806e906eeb048517aeb67f34ac0e2faf1 nested/directory/file.md
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 empty.txt
fcde2b2edba56bf408601fb721fe9b5c338d10ee429ea04fae5511b68fbf8fb9 foo
`
	require.Equal(t, expectedContent, checksumContent)
	require.Equal(t, "7c554cf67e1c2b50a1b728299c368cd56d53588300c37479623f29a52812ca3f", checksumHash)
}

func TestValidateFileChecksum(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(path, []byte("content"), 0o600))
	sha, err := helpers.GetSHA256OfFile(path)
	require.NoError(t, err)

	require.NoError(t, validateFileChecksum(path, sha))
	require.NoError(t, validateFileChecksum(path, "sha256:"+sha))
	require.ErrorContains(t, validateFileChecksum(path, "sha512:"+sha), `unsupported checksum algorithm "sha512"`)
}

func TestCreateReproducibleTarballFromDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello world"), 0o600)
	require.NoError(t, err)
	tarPath := filepath.Join(t.TempDir(), "data.tar")

	err = createReproducibleTarballFromDir(tmpDir, "", tarPath, true)
	require.NoError(t, err)

	shaSum, err := helpers.GetSHA256OfFile(tarPath)
	require.NoError(t, err)
	require.Equal(t, "c09d17f612f241cdf549e5fb97c9e063a8ad18ae7a9f3af066332ed6b38556ad", shaSum)
}

func TestValidateImageArchivesNoDuplicates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		components    []v1alpha1.ZarfComponent
		errorContains string
	}{
		{
			name: "no duplicates",
			components: []v1alpha1.ZarfComponent{
				{
					Name: "component1",
					ImageArchives: []v1alpha1.ImageArchive{
						{
							Path:   "/path/to/archive1.tar",
							Images: []string{"nginx:1.21"},
						},
					},
					Images: []string{"redis:6.2"},
				},
			},
		},
		{
			name: "duplicate in different image archive",
			components: []v1alpha1.ZarfComponent{
				{
					Name: "component1",
					ImageArchives: []v1alpha1.ImageArchive{
						{
							Path:   "/path/to/archive1.tar",
							Images: []string{"postgres:13"},
						},
						{
							Path:   "/path/to/archive2.tar",
							Images: []string{"postgres:13"},
						},
					},
				},
			},
			errorContains: "appears in multiple image archives",
		},
		{
			name: "duplicate in component images",
			components: []v1alpha1.ZarfComponent{
				{
					Name: "component1",
					ImageArchives: []v1alpha1.ImageArchive{
						{
							Path:   "/path/to/archive1.tar",
							Images: []string{"ghcr.io/zarf-dev/zarf/agent:0.65.0"},
						},
					},
					Images: []string{"nginx:1.21", "ghcr.io/zarf-dev/zarf/agent:0.65.0"},
				},
			},
			errorContains: "is also pulled by component",
		},
		{
			name: "duplicate in component with docker ref",
			components: []v1alpha1.ZarfComponent{
				{
					Name: "component1",
					ImageArchives: []v1alpha1.ImageArchive{
						{
							Path:   "/path/to/archive1.tar",
							Images: []string{"nginx:1.21"},
						},
					},
					Images: []string{"nginx:1.21"},
				},
			},
			errorContains: "is also pulled by component",
		},
		{
			name: "duplicate across multiple components",
			components: []v1alpha1.ZarfComponent{
				{
					Name: "component1",
					ImageArchives: []v1alpha1.ImageArchive{
						{
							Path:   "/path/to/archive1.tar",
							Images: []string{"nginx:1.21"},
						},
					},
				},
				{
					Name: "component2",
					ImageArchives: []v1alpha1.ImageArchive{
						{
							Path:   "/path/to/archive2.tar",
							Images: []string{"nginx:1.21"},
						},
					},
				},
			},
			errorContains: "appears in multiple image archives",
		},
		{
			name: "empty components",
			components: []v1alpha1.ZarfComponent{
				{
					Name: "component1",
				},
			},
		},
		{
			name: "duplicate images in component.Images is allowed",
			components: []v1alpha1.ZarfComponent{
				{
					Name:   "component1",
					Images: []string{"nginx:1.21"},
				},
				{
					Name:   "component2",
					Images: []string{"nginx:1.21"},
				},
			},
		},
		{
			name: "same archive path used by multiple components is allowed",
			components: []v1alpha1.ZarfComponent{
				{
					Name: "component1",
					ImageArchives: []v1alpha1.ImageArchive{
						{
							Path:   "/path/to/shared-archive.tar",
							Images: []string{"nginx:1.21", "redis:6.2"},
						},
					},
				},
				{
					Name: "component2",
					ImageArchives: []v1alpha1.ImageArchive{
						{
							Path:   "/path/to/shared-archive.tar",
							Images: []string{"nginx:1.21", "postgres:13"},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateImageArchivesNoDuplicates(tt.components)

			if tt.errorContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCollectVersionRequirements(t *testing.T) {
	t.Parallel()

	imageArchivesReq := v1alpha1.VersionRequirement{
		Version: "v0.68.0",
		Reason:  "This package contains image archives which will only be recognized on v0.68.0+",
	}
	indexReq := v1alpha1.VersionRequirement{
		Version: "v0.77.0",
		Reason:  "This package contains multi-platform images preserved by index digest, which require v0.77.0+",
	}
	versionlessChartReq := v1alpha1.VersionRequirement{
		Version: "v0.65.0",
		Reason:  "This package contains a chart without a version, which is only supported on v0.65.0+",
	}

	tests := []struct {
		name     string
		pkg      v1alpha1.ZarfPackage
		hasIndex bool
		expected []v1alpha1.VersionRequirement
	}{
		{
			name:     "no requirements for a plain package",
			pkg:      v1alpha1.ZarfPackage{},
			expected: nil,
		},
		{
			name: "image archives trigger v0.68.0",
			pkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "c1",
						ImageArchives: []v1alpha1.ImageArchive{
							{Path: "/tmp/archive.tar", Images: []string{"nginx:1.21"}},
						},
					},
				},
			},
			expected: []v1alpha1.VersionRequirement{imageArchivesReq},
		},
		{
			name:     "preserved index triggers v0.76.0",
			pkg:      v1alpha1.ZarfPackage{},
			hasIndex: true,
			expected: []v1alpha1.VersionRequirement{indexReq},
		},
		{
			name: "image archives and preserved index trigger both",
			pkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name:          "c1",
						ImageArchives: []v1alpha1.ImageArchive{{Path: "/tmp/a.tar", Images: []string{"x:y"}}},
					},
				},
			},
			hasIndex: true,
			expected: []v1alpha1.VersionRequirement{imageArchivesReq, indexReq},
		},
		{
			name: "image archives requirement is only emitted once across components",
			pkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{Name: "c1", ImageArchives: []v1alpha1.ImageArchive{{Path: "/tmp/a.tar", Images: []string{"x:y"}}}},
					{Name: "c2", ImageArchives: []v1alpha1.ImageArchive{{Path: "/tmp/b.tar", Images: []string{"p:q"}}}},
				},
			},
			expected: []v1alpha1.VersionRequirement{imageArchivesReq},
		},
		{
			name: "chart without a version triggers v0.65.0",
			pkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name:   "c1",
						Charts: []v1alpha1.ZarfChart{{Name: "local", LocalPath: "./chart"}},
					},
				},
			},
			expected: []v1alpha1.VersionRequirement{versionlessChartReq},
		},
		{
			name: "versionless chart requirement is only emitted once across charts",
			pkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{Name: "c1", Charts: []v1alpha1.ZarfChart{{Name: "a", LocalPath: "./a"}}},
					{Name: "c2", Charts: []v1alpha1.ZarfChart{{Name: "b", LocalPath: "./b"}}},
				},
			},
			expected: []v1alpha1.VersionRequirement{versionlessChartReq},
		},
		{
			name: "chart with a version has no requirement",
			pkg: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name:   "c1",
						Charts: []v1alpha1.ZarfChart{{Name: "local", LocalPath: "./chart", Version: "1.0.0"}},
					},
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, collectVersionRequirements(tt.pkg, tt.hasIndex))
		})
	}
}

func TestImageLayoutHasIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		indexJSON   string
		writeFile   bool
		expected    bool
		errContains string
	}{
		{
			name:      "missing index.json returns false",
			writeFile: false,
			expected:  false,
		},
		{
			name:      "empty manifests returns false",
			writeFile: true,
			indexJSON: `{"schemaVersion":2,"manifests":[]}`,
			expected:  false,
		},
		{
			name:      "only image manifests returns false",
			writeFile: true,
			indexJSON: `{"schemaVersion":2,"manifests":[{"mediaType":"` + ocispec.MediaTypeImageManifest + `","digest":"sha256:abc","size":1}]}`,
			expected:  false,
		},
		{
			name:      "OCI image index returns true",
			writeFile: true,
			indexJSON: `{"schemaVersion":2,"manifests":[{"mediaType":"` + ocispec.MediaTypeImageIndex + `","digest":"sha256:abc","size":1}]}`,
			expected:  true,
		},
		{
			name:      "docker manifest list returns true",
			writeFile: true,
			indexJSON: `{"schemaVersion":2,"manifests":[{"mediaType":"` + images.DockerMediaTypeManifestList + `","digest":"sha256:abc","size":1}]}`,
			expected:  true,
		},
		{
			name:        "malformed JSON returns error",
			writeFile:   true,
			indexJSON:   `{not valid json`,
			expected:    false,
			errContains: "failed to parse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if tt.writeFile {
				err := os.WriteFile(filepath.Join(dir, layout.IndexJSON), []byte(tt.indexJSON), 0o600)
				require.NoError(t, err)
			}

			got, err := layout.HasImageIndex(dir)
			if tt.errContains != "" {
				require.ErrorContains(t, err, tt.errContains)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestMergeAndWriteValuesFile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("no-op when no files provided", func(t *testing.T) {
		t.Parallel()
		buildPath := t.TempDir()
		err := mergeAndWriteValuesFile(ctx, nil, t.TempDir(), buildPath)
		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(buildPath, layout.ValuesYAML))
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("merges multiple values files into a single output", func(t *testing.T) {
		t.Parallel()
		pkgDir := t.TempDir()
		buildPath := t.TempDir()

		require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "base.yaml"), []byte("key: base\nextra: present\n"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "override.yaml"), []byte("key: override\n"), 0o600))

		err := mergeAndWriteValuesFile(ctx, []string{"base.yaml", "override.yaml"}, pkgDir, buildPath)
		require.NoError(t, err)

		out, err := os.ReadFile(filepath.Join(buildPath, layout.ValuesYAML))
		require.NoError(t, err)
		require.Contains(t, string(out), "key: override")
		require.Contains(t, string(out), "extra: present")
	})

	t.Run("returns error when a values file does not exist", func(t *testing.T) {
		t.Parallel()
		err := mergeAndWriteValuesFile(ctx, []string{"does-not-exist.yaml"}, t.TempDir(), t.TempDir())
		require.ErrorContains(t, err, "does-not-exist.yaml")
	})
}

func TestMergeAndWriteValuesSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testdataDir := filepath.Join("testdata", "schema-merge")

	t.Run("no-op when neither parent nor children are provided", func(t *testing.T) {
		t.Parallel()
		buildPath := t.TempDir()
		err := mergeAndWriteValuesSchema(ctx, "", nil, testdataDir, buildPath)
		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(buildPath, layout.ValuesSchema))
		require.ErrorIs(t, err, os.ErrNotExist, "no schema file should be written when there is nothing to merge")
	})

	t.Run("copies parent verbatim when no child schemas are present", func(t *testing.T) {
		t.Parallel()
		buildPath := t.TempDir()
		err := mergeAndWriteValuesSchema(ctx, "parent-with-required.schema.json", nil, testdataDir, buildPath)
		require.NoError(t, err)
		written, err := os.ReadFile(filepath.Join(buildPath, layout.ValuesSchema))
		require.NoError(t, err)
		original, err := os.ReadFile(filepath.Join(testdataDir, "parent-with-required.schema.json"))
		require.NoError(t, err)
		require.Equal(t, string(original), string(written), "verbatim copy should match source file exactly")
	})

	t.Run("rejects parent schema containing external $ref", func(t *testing.T) {
		t.Parallel()
		buildPath := t.TempDir()
		err := mergeAndWriteValuesSchema(ctx, "child-with-external-ref.schema.json", nil, testdataDir, buildPath)
		require.ErrorContains(t, err, "$ref")
	})

	t.Run("rejects child schema containing external $ref", func(t *testing.T) {
		t.Parallel()
		buildPath := t.TempDir()
		err := mergeAndWriteValuesSchema(ctx, "parent-with-required.schema.json", []string{"child-with-external-ref.schema.json"}, testdataDir, buildPath)
		require.ErrorContains(t, err, "$ref")
	})

	t.Run("allows internal fragment refs in schemas being merged", func(t *testing.T) {
		t.Parallel()
		buildPath := t.TempDir()
		// child-with-ref.schema.json uses "$ref": "#/definitions/name" — internal, safe to merge
		err := mergeAndWriteValuesSchema(ctx, "parent-with-required.schema.json", []string{"child-with-ref.schema.json"}, testdataDir, buildPath)
		require.NoError(t, err)
	})

	t.Run("rejects merge when parent and child declare different versions", func(t *testing.T) {
		t.Parallel()
		buildPath := t.TempDir()
		// parent-with-required declares draft-07; child-wrong-version declares 2019-09
		err := mergeAndWriteValuesSchema(ctx, "parent-with-required.schema.json", []string{"child-wrong-version.schema.json"}, testdataDir, buildPath)
		require.ErrorContains(t, err, "different versions")
		require.ErrorContains(t, err, "draft-07")
		require.ErrorContains(t, err, "2019-09")
	})

	t.Run("preserves child definitions when parent overrides with empty map so internal refs remain valid", func(t *testing.T) {
		t.Parallel()
		buildPath := t.TempDir()
		// child-with-ref uses $ref: "#/definitions/name" with a matching definition.
		// parent-overrides-definitions sets definitions: {} (empty), which previously
		// deleted the child's definition and left the $ref unresolvable.
		// With definitions merged like properties, the child-only "name" entry survives.
		err := mergeAndWriteValuesSchema(ctx, "parent-overrides-definitions.schema.json", []string{"child-with-ref.schema.json"}, testdataDir, buildPath)
		require.NoError(t, err)
	})

	t.Run("rejects merge when child omits version", func(t *testing.T) {
		t.Parallel()
		buildPath := t.TempDir()
		err := mergeAndWriteValuesSchema(ctx, "parent-with-required.schema.json", []string{"child-no-dialect.schema.json"}, testdataDir, buildPath)
		require.ErrorContains(t, err, "missing \"$schema\" version declaration")
	})

	t.Run("rejects merge when parent omits version", func(t *testing.T) {
		t.Parallel()
		buildPath := t.TempDir()
		err := mergeAndWriteValuesSchema(ctx, "child-no-dialect.schema.json", []string{"child.schema.json"}, testdataDir, buildPath)
		require.ErrorContains(t, err, "missing \"$schema\" version declaration")
	})

	mergeTests := []struct {
		name            string
		parentSchema    string
		importedSchemas []string
		expectedSchema  string
	}{
		{
			name:            "parent and child required arrays are merged — parent entries first",
			parentSchema:    "parent-with-required.schema.json",
			importedSchemas: []string{"child.schema.json"},
			// parent required: ["namespace"], child required: ["appName","replicas"]
			// merged (parent-first): ["namespace","appName","replicas"]
			// parent replicas.maximum:5 wins over child's 10
			expectedSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"required": ["namespace","appName","replicas"],
				"properties": {
					"namespace": {"type":"string","minLength":1},
					"replicas":  {"type":"integer","minimum":1,"maximum":5},
					"appName":   {"type":"string","minLength":1},
					"enabled":   {"type":"boolean"}
				}
			}`,
		},
		{
			name:            "child required survives when parent declares no required array",
			parentSchema:    "parent-no-required.schema.json",
			importedSchemas: []string{"child.schema.json"},
			// parent has no required; child required: ["appName","replicas"] preserved as-is
			expectedSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"required": ["appName","replicas"],
				"properties": {
					"namespace": {"type":"string","minLength":1},
					"replicas":  {"type":"integer","minimum":1,"maximum":5},
					"appName":   {"type":"string","minLength":1},
					"enabled":   {"type":"boolean"}
				}
			}`,
		},
		{
			name:            "overlapping required entries are deduplicated with parent ordering preserved",
			parentSchema:    "parent-overlapping-required.schema.json",
			importedSchemas: []string{"child.schema.json"},
			// parent required: ["appName","namespace"], child required: ["appName","replicas"]
			// dedup (parent-first): ["appName","namespace","replicas"]
			expectedSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"required": ["appName","namespace","replicas"],
				"properties": {
					"namespace": {"type":"string","minLength":1},
					"replicas":  {"type":"integer","minimum":1,"maximum":5},
					"appName":   {"type":"string","minLength":1},
					"enabled":   {"type":"boolean"}
				}
			}`,
		},
		{
			name:            "first sibling wins on property conflicts when no parent is present",
			importedSchemas: []string{"child.schema.json", "child2.schema.json"},
			// child required: ["appName","replicas"], child2 required: ["version"]
			// child replicas.maximum:10 wins over child2's 20 (conflict: child wins)
			// child enabled has no description; child2 adds description — no conflict, description is inherited
			// version property comes from child2 only
			expectedSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"required": ["appName","replicas","version"],
				"properties": {
					"appName":  {"type":"string","minLength":1},
					"replicas": {"type":"integer","minimum":1,"maximum":10},
					"enabled":  {"type":"boolean","description":"child2"},
					"version":  {"type":"string","pattern":"^v[0-9]+"}
				}
			}`,
		},
		{
			name:            "parent wins over all siblings; sibling-only properties are still included",
			parentSchema:    "parent-with-required.schema.json",
			importedSchemas: []string{"child.schema.json", "child2.schema.json"},
			// children merged first: replicas.maximum:10 (child wins child2)
			// parent merged on top: replicas.maximum:5 (parent wins children)
			// required: parent ["namespace"] + child ["appName","replicas"] + child2 ["version"]
			// enabled.description inherited from child2 (no conflict with parent or child1)
			expectedSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"required": ["namespace","appName","replicas","version"],
				"properties": {
					"namespace": {"type":"string","minLength":1},
					"replicas":  {"type":"integer","minimum":1,"maximum":5},
					"appName":   {"type":"string","minLength":1},
					"enabled":   {"type":"boolean","description":"child2"},
					"version":   {"type":"string","pattern":"^v[0-9]+"}
				}
			}`,
		},
	}

	for _, tt := range mergeTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buildPath := t.TempDir()
			err := mergeAndWriteValuesSchema(ctx, tt.parentSchema, tt.importedSchemas, testdataDir, buildPath)
			require.NoError(t, err)
			written, err := os.ReadFile(filepath.Join(buildPath, layout.ValuesSchema))
			require.NoError(t, err)
			require.JSONEq(t, tt.expectedSchema, string(written))
		})
	}
}

func TestAssembleSkeleton(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	defined, err := load.PackageDefinition(ctx, "./testdata/zarf-skeleton-package", load.DefinitionOptions{})
	require.NoError(t, err)

	opt := AssembleSkeletonOptions{}
	pkgLayout, err := AssembleSkeleton(ctx, defined, "./testdata/zarf-skeleton-package", opt)
	require.NoError(t, err)

	b, err := os.ReadFile(filepath.Join(pkgLayout.DirPath(), "checksums.txt"))
	require.NoError(t, err)
	expectedChecksum := `0fea7403536c0c0e2a2d9b235d4b3716e86eefd8e78e7b14412dd5a750b77474 components/kustomizations.tar
54f657b43323e1ebecb0758835b8d01a0113b61b7bab0f4a8156f031128d00f9 components/data-injections.tar
879bfe82d20f7bdcd60f9e876043cc4343af4177a6ee8b2660c304a5b6c70be7 components/files.tar
bd82245bfc3c79abfa23dcf72c8099a2788c1b6073464f1ee0c6b64b9c8ef2f6 documentation.tar
c497f1a56559ea0a9664160b32e4b377df630454ded6a3787924130c02f341a6 components/manifests.tar
fb7ebee94a4479bacddd71195030a483b0b0b96d4f73f7fcd2c2c8e0fce0c5c6 components/helm-charts.tar
`

	require.Equal(t, expectedChecksum, string(b))
	testutil.RequireNoBackslashInPackagePaths(t, pkgLayout.AsV1alpha1())
	require.Equal(t, "20c2cf8bde902c8daad1ad9fb3cd9f06741550ac34401474500a24835cb36114", testutil.ChecksumZarfYAMLContent(t, pkgLayout.AsV1alpha1()), "skeleton zarf.yaml checksum drift — package would differ across build hosts")
}

func writePackageToDisk(t *testing.T, pkg v1alpha1.ZarfPackage, dir string) {
	t.Helper()
	b, err := goyaml.Marshal(pkg)
	require.NoError(t, err)
	path := filepath.Join(dir, layout.ZarfYAML)
	err = os.WriteFile(path, b, 0700)
	require.NoError(t, err)
}

func TestAssemblePackageV1Beta1WritesMultiDocDefinition(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	tmpdir := t.TempDir()
	fixture := filepath.Join("testdata", "zarf-package")
	dataPath, err := filepath.Abs(filepath.Join(fixture, "data.txt"))
	require.NoError(t, err)
	chartPath, err := filepath.Abs(filepath.Join(fixture, "chart"))
	require.NoError(t, err)
	kustomizePath, err := filepath.Abs(filepath.Join(fixture, "kustomize"))
	require.NoError(t, err)
	valuesPath, err := filepath.Abs(filepath.Join(fixture, "values.yaml"))
	require.NoError(t, err)
	docPath, err := filepath.Abs(filepath.Join(fixture, "doc.md"))
	require.NoError(t, err)

	zarfYAML := fmt.Sprintf(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: beta-local
  version: 0.0.1
  architecture: amd64
documentation:
  docs: %q
components:
  - name: beta-component
    files:
      - source: %q
        destination: data.txt
    manifests:
      - name: beta-manifest
        files:
          - %q
        kustomize:
          files:
            - %q
    charts:
      - name: beta-chart
        namespace: beta
        local:
          path: %q
        valuesFiles:
          - path: %q
        skipWait: true
`, docPath, dataPath, dataPath, kustomizePath, chartPath, valuesPath)
	require.NoError(t, os.WriteFile(filepath.Join(tmpdir, layout.ZarfYAML), []byte(zarfYAML), 0o600))

	defined, err := load.PackageDefinition(ctx, tmpdir, load.DefinitionOptions{})
	require.NoError(t, err)
	pkgLayout, err := AssemblePackage(ctx, defined, tmpdir, AssembleOptions{SkipSBOM: true})
	require.NoError(t, err)

	b, err := os.ReadFile(filepath.Join(pkgLayout.DirPath(), layout.ZarfYAML))
	require.NoError(t, err)
	alphaPkg, err := pkgcfg.ParseAs(ctx, b, pkgcfg.V1Alpha1)
	require.NoError(t, err)
	betaPkg, err := pkgcfg.ParseAs(ctx, b, pkgcfg.V1Beta1)
	require.NoError(t, err)
	require.Equal(t, v1alpha1.APIVersion, alphaPkg.APIVersion)
	require.Equal(t, v1beta1.APIVersion, betaPkg.APIVersion)
	require.Equal(t, "beta-local", betaPkg.Metadata.Name)
	require.NotEmpty(t, alphaPkg.Metadata.AggregateChecksum)
	require.Equal(t, alphaPkg.Metadata.AggregateChecksum, betaPkg.Build.AggregateChecksum)
	require.Len(t, betaPkg.Components, 1)
	require.Len(t, betaPkg.Components[0].Charts, 1)
	require.NotNil(t, betaPkg.Components[0].Charts[0].Local)
	require.Equal(t, chartPath, betaPkg.Components[0].Charts[0].Local.Path)

	chartComponent, err := pkgLayout.GetComponentDir(ctx, tmpdir, "beta-component", layout.ChartsComponentDir)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(chartComponent, "beta-chart.tgz"))
}

func TestGetSBOM(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	tmpdir := t.TempDir()
	pkg := v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Metadata: v1alpha1.ZarfMetadata{
			Name: "test-sbom",
		},
		Components: []v1alpha1.ZarfComponent{
			{
				Name: "do-nothing",
			},
		},
	}
	writePackageToDisk(t, pkg, tmpdir)
	defined, err := load.PackageDefinition(ctx, tmpdir, load.DefinitionOptions{})
	require.NoError(t, err)
	pkgLayout, err := AssemblePackage(ctx, defined, tmpdir, AssembleOptions{})
	require.NoError(t, err)

	// Ensure the SBOM does not exist
	require.NoFileExists(t, filepath.Join(pkgLayout.DirPath(), layout.SBOMTar))
	// Ensure Zarf errors correctly
	err = pkgLayout.GetSBOM(ctx, tmpdir)
	var noSBOMErr *layout.NoSBOMAvailableError
	require.ErrorAs(t, err, &noSBOMErr)
}

func TestCreateAbsoluteSources(t *testing.T) {
	ctx := testutil.TestContext(t)
	tests := []struct {
		name       string
		isSkeleton bool
	}{
		{
			name:       "regular package",
			isSkeleton: false,
		},
		{
			name:       "skeleton package",
			isSkeleton: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpdir := t.TempDir()
			absoluteFilePath, err := filepath.Abs(filepath.Join("testdata", "zarf-package", "data.txt"))
			require.NoError(t, err)
			absoluteChartPath, err := filepath.Abs(filepath.Join("testdata", "zarf-package", "chart"))
			require.NoError(t, err)
			absoluteKustomizePath, err := filepath.Abs(filepath.Join("testdata", "zarf-package", "kustomize"))
			require.NoError(t, err)
			absoluteDocsPath, err := filepath.Abs(filepath.Join("testdata", "zarf-package", "doc.md"))
			require.NoError(t, err)
			componentName := "absolute-files"
			pkg := v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Name: "standard",
				},
				Documentation: map[string]string{
					"docs": absoluteDocsPath,
				},
				Components: []v1alpha1.ZarfComponent{
					{
						Name: componentName,
						Files: []v1alpha1.ZarfFile{
							{
								Source: absoluteFilePath,
								Target: "file.txt",
							},
						},
						Manifests: []v1alpha1.ZarfManifest{
							{
								Name: "test-manifest",
								Files: []string{
									absoluteFilePath,
								},
								Kustomizations: []string{
									absoluteKustomizePath,
								},
							},
						},
						DataInjections: []v1alpha1.ZarfDataInjection{
							{
								Source: absoluteFilePath,
							},
						},
						Charts: []v1alpha1.ZarfChart{
							{
								Name:      "test-chart",
								Namespace: "test",
								Version:   "1.0.0",
								LocalPath: absoluteChartPath,
								ValuesFiles: []string{
									absoluteFilePath,
								},
							},
						},
					},
				},
			}
			// Create the zarf.yaml file in the tmpdir
			writePackageToDisk(t, pkg, tmpdir)

			defined, err := load.PackageDefinition(ctx, tmpdir, load.DefinitionOptions{})
			require.NoError(t, err)
			var pkgLayout *layout.PackageLayout
			if tt.isSkeleton {
				pkgLayout, err = AssembleSkeleton(ctx, defined, tmpdir, AssembleSkeletonOptions{})
				require.NoError(t, err)
			} else {
				pkgLayout, err = AssemblePackage(ctx, defined, tmpdir, AssembleOptions{SkipSBOM: true})
				require.NoError(t, err)
			}
			docsDir := filepath.Join(tmpdir, "docs-dir")
			err = pkgLayout.GetDocumentation(ctx, docsDir, []string{})
			require.NoError(t, err)
			require.FileExists(t, filepath.Join(docsDir, "doc.md"))

			// Ensure the component has the correct files
			fileComponent, err := pkgLayout.GetComponentDir(ctx, tmpdir, componentName, layout.FilesComponentDir)
			require.NoError(t, err)
			require.FileExists(t, filepath.Join(fileComponent, "0", "file.txt"))

			chartComponent, err := pkgLayout.GetComponentDir(ctx, tmpdir, componentName, layout.ChartsComponentDir)
			require.NoError(t, err)
			if tt.isSkeleton {
				require.DirExists(t, filepath.Join(chartComponent, "test-chart-0"))
			} else {
				require.FileExists(t, filepath.Join(chartComponent, "test-chart-1.0.0.tgz"))
			}

			manifestComponent, err := pkgLayout.GetComponentDir(ctx, tmpdir, componentName, layout.ManifestsComponentDir)
			require.NoError(t, err)
			require.FileExists(t, filepath.Join(manifestComponent, "test-manifest-0.yaml"))
			require.FileExists(t, filepath.Join(manifestComponent, "kustomization-test-manifest-0.yaml"))

			dataInjectionsDir, err := pkgLayout.GetComponentDir(ctx, tmpdir, componentName, layout.DataComponentDir)
			require.NoError(t, err)
			require.FileExists(t, filepath.Join(dataInjectionsDir, "0"))
		})
	}
}

func TestCreateAbsolutePathImports(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	tmpdir := t.TempDir()
	absoluteFilePath, err := filepath.Abs(filepath.Join("testdata", "zarf-package", "data.txt"))
	require.NoError(t, err)
	parentPkg := v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Metadata: v1alpha1.ZarfMetadata{
			Name: "parent",
		},
		Components: []v1alpha1.ZarfComponent{
			{
				Name: "file-import",
				Import: v1alpha1.ZarfComponentImport{
					Path: "child",
				},
			},
		},
	}
	// Create package using absolute file path set to be import
	childPkg := v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Metadata: v1alpha1.ZarfMetadata{
			Name: "child",
		},
		Components: []v1alpha1.ZarfComponent{
			{
				Name: "file-import",
				Files: []v1alpha1.ZarfFile{
					{
						Source: absoluteFilePath,
						Target: "file.txt",
					},
				},
			},
		},
	}
	// Create zarf.yaml files in the tempdir
	writePackageToDisk(t, parentPkg, tmpdir)
	childDir := filepath.Join(tmpdir, "child")
	err = os.Mkdir(childDir, 0700)
	require.NoError(t, err)
	writePackageToDisk(t, childPkg, childDir)
	defined, err := load.PackageDefinition(ctx, tmpdir, load.DefinitionOptions{})
	require.NoError(t, err)
	// create the package
	pkgLayout, err := AssemblePackage(context.Background(), defined, tmpdir, AssembleOptions{})
	require.NoError(t, err)

	// Ensure the component has the correct file
	importedFileComponent, err := pkgLayout.GetComponentDir(ctx, tmpdir, "file-import", layout.FilesComponentDir)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(importedFileComponent, "0", "file.txt"))

	// Ensure the sbom exists as expected
	err = pkgLayout.GetSBOM(ctx, tmpdir)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(tmpdir, "zarf-component-file-import.json"))
}
