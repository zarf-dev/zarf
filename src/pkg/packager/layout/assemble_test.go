// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/images"
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
				err := os.WriteFile(filepath.Join(dir, IndexJSON), []byte(tt.indexJSON), 0o600)
				require.NoError(t, err)
			}

			got, err := imageLayoutHasIndex(dir)
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
		_, err = os.Stat(filepath.Join(buildPath, ValuesYAML))
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

		out, err := os.ReadFile(filepath.Join(buildPath, ValuesYAML))
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
		_, err = os.Stat(filepath.Join(buildPath, ValuesSchema))
		require.ErrorIs(t, err, os.ErrNotExist, "no schema file should be written when there is nothing to merge")
	})

	t.Run("copies parent verbatim when no child schemas are present", func(t *testing.T) {
		t.Parallel()
		buildPath := t.TempDir()
		err := mergeAndWriteValuesSchema(ctx, "parent-with-required.schema.json", nil, testdataDir, buildPath)
		require.NoError(t, err)
		written, err := os.ReadFile(filepath.Join(buildPath, ValuesSchema))
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
			written, err := os.ReadFile(filepath.Join(buildPath, ValuesSchema))
			require.NoError(t, err)
			require.JSONEq(t, tt.expectedSchema, string(written))
		})
	}
}
