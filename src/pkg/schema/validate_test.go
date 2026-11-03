// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package schema

import (
	"strings"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

func TestZarfSchema(t *testing.T) {
	t.Parallel()
	zarfSchema := GetV1Alpha1Schema()

	tests := []struct {
		name                  string
		pkg                   v1alpha1.ZarfPackage
		expectedSchemaStrings []string
	}{
		{
			name: "valid package",
			pkg: v1alpha1.ZarfPackage{
				APIVersion: v1alpha1.APIVersion,
				Kind:       v1alpha1.ZarfInitConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Name: "valid-name",
				},
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "valid-comp",
					},
				},
			},
			expectedSchemaStrings: nil,
		},
		{
			name: "no comp or kind",
			pkg: v1alpha1.ZarfPackage{
				Metadata: v1alpha1.ZarfMetadata{
					Name: "no-comp-or-kind",
				},
				Components: []v1alpha1.ZarfComponent{},
			},
			expectedSchemaStrings: []string{
				"kind: kind must be one of the following: \"ZarfInitConfig\", \"ZarfPackageConfig\"",
				"components: Array must have at least 1 items",
			},
		},
		{
			name: "path separators in dynamic path fields",
			pkg: v1alpha1.ZarfPackage{
				APIVersion: v1alpha1.APIVersion,
				Kind:       v1alpha1.ZarfPackageConfig,
				Metadata:   v1alpha1.ZarfMetadata{Name: "pkg", Version: "a/b"},
				Components: []v1alpha1.ZarfComponent{
					{
						Name:      "comp",
						Charts:    []v1alpha1.ZarfChart{{Name: "a/b", Version: "1.0"}},
						Manifests: []v1alpha1.ZarfManifest{{Name: `a\b`}},
					},
				},
			},
			expectedSchemaStrings: []string{
				"metadata.version: Does not match pattern '^[^/\\\\]*$'",
				"components.0.charts.0.name: Does not match pattern '^[^/\\\\]*$'",
				"components.0.manifests.0.name: Does not match pattern '^[^/\\\\]*$'",
			},
		},
		{
			name: "invalid package",
			pkg: v1alpha1.ZarfPackage{
				APIVersion: "bad-api-version/wrong",
				Kind:       v1alpha1.ZarfInitConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Name: "-invalid-name",
				},
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "invalid-name",
						Only: v1alpha1.ZarfComponentOnlyTarget{
							LocalOS: "unsupportedOS",
						},
					},
					{
						Name: "actions",
						Actions: v1alpha1.ZarfComponentActions{
							OnCreate: v1alpha1.ZarfComponentActionSet{
								Before: []v1alpha1.ZarfComponentAction{
									{
										Cmd:          "echo 'invalid setVariable'",
										SetVariables: []v1alpha1.Variable{{Name: "not_uppercase"}},
									},
								},
							},
							OnRemove: v1alpha1.ZarfComponentActionSet{
								OnSuccess: []v1alpha1.ZarfComponentAction{
									{
										Cmd:          "echo 'invalid setVariable'",
										SetVariables: []v1alpha1.Variable{{Name: "not_uppercase"}},
									},
								},
							},
						},
					},
				},
				Variables: []v1alpha1.InteractiveVariable{
					{
						Variable: v1alpha1.Variable{Name: "not_uppercase"},
					},
				},
				Constants: []v1alpha1.Constant{
					{
						Name: "not_uppercase",
					},
				},
			},
			expectedSchemaStrings: []string{
				"metadata.name: Does not match pattern '^[a-z0-9][a-z0-9\\-]*$'",
				"variables.0.name: Does not match pattern '^[A-Z0-9_]+$'",
				"constants.0.name: Does not match pattern '^[A-Z0-9_]+$'",
				"components.0.only.localOS: components.0.only.localOS must be one of the following: \"linux\", \"darwin\", \"windows\"",
				"components.1.actions.onCreate.before.0.setVariables.0.name: Does not match pattern '^[A-Z0-9_]+$'",
				"components.1.actions.onRemove.onSuccess.0.setVariables.0.name: Does not match pattern '^[A-Z0-9_]+$'",
				"apiVersion: apiVersion must be one of the following: \"zarf.dev/v1alpha1\"",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(zarfSchema, tt.pkg)
			if len(tt.expectedSchemaStrings) == 0 {
				require.NoError(t, err)
				return
			}
			var validationErr *ValidationError
			require.ErrorAs(t, err, &validationErr)
			var schemaStrings []string
			for _, schemaErr := range validationErr.Errors {
				schemaStrings = append(schemaStrings, schemaErr.String())
			}
			require.ElementsMatch(t, tt.expectedSchemaStrings, schemaStrings)
		})
	}

	t.Run("validate schema fail with errors not possible from object", func(t *testing.T) {
		t.Parallel()
		// When we want to test the absence of a field, an incorrect type, or an extra field
		// we can't do it through a struct since non pointer fields will have a zero value of their type
		const badZarfPackage = `
kind: ZarfInitConfig
extraField: whatever
metadata:
  name: invalid
  description: Testing bad yaml

components:
- name: import-test
  import:
    path: 123123
  charts:
  - noWait: true
  manifests:
  - namespace: no-name-for-manifest
`
		var unmarshalledYaml interface{}
		err := goyaml.Unmarshal([]byte(badZarfPackage), &unmarshalledYaml)
		require.NoError(t, err)
		err = Validate(zarfSchema, unmarshalledYaml)
		var validationErr *ValidationError
		require.ErrorAs(t, err, &validationErr)
		var schemaStrings []string
		for _, schemaErr := range validationErr.Errors {
			schemaStrings = append(schemaStrings, schemaErr.String())
		}
		expectedSchemaStrings := []string{
			"(root): Additional property extraField is not allowed",
			"components.0.import.path: Invalid type. Expected: string, given: integer",
			"components.0.charts.0: name is required",
			"components.0.manifests.0: name is required",
		}

		require.ElementsMatch(t, expectedSchemaStrings, schemaStrings)
	})

	t.Run("validation error retains field path and description", func(t *testing.T) {
		t.Parallel()
		err := Validate(zarfSchema, v1alpha1.ZarfPackage{
			Kind: v1alpha1.ZarfInitConfig,
			Metadata: v1alpha1.ZarfMetadata{
				Name: "invalid",
			},
		})
		var validationErr *ValidationError
		require.ErrorAs(t, err, &validationErr)
		require.Len(t, validationErr.Errors, 1)
		require.Equal(t, "components", validationErr.Errors[0].Field())
		require.Equal(t, "Invalid type. Expected: array, given: null", validationErr.Errors[0].Description())
		require.ErrorContains(t, err, "components: Invalid type. Expected: array, given: null")
	})
}

func TestV1Beta1SourceOneOfSchema(t *testing.T) {
	t.Parallel()

	baseDoc := func(component map[string]any) map[string]any {
		return map[string]any{
			"apiVersion": "zarf.dev/v1beta1",
			"kind":       "ZarfPackageConfig",
			"metadata":   map[string]any{"name": "test"},
			"components": []any{component},
		}
	}
	chartComponent := func(chart map[string]any) map[string]any {
		return map[string]any{
			"name":   "component",
			"charts": []any{chart},
		}
	}

	tests := []struct {
		name  string
		doc   map[string]any
		valid bool
	}{
		{
			name: "chart has exactly one source",
			doc: baseDoc(chartComponent(map[string]any{
				"name": "chart",
				"local": map[string]any{
					"path": "chart",
				},
			})),
			valid: true,
		},
		{
			name: "oci ref rejects tag and digest",
			doc: baseDoc(chartComponent(map[string]any{
				"name": "chart",
				"oci": map[string]any{
					"url": "oci://example.com/chart",
					"ref": map[string]any{
						"tag":    "1.0.0",
						"digest": "sha256:abcdef",
					},
				},
			})),
			valid: false,
		},
		{
			name: "git chart ref rejects tag and branch",
			doc: baseDoc(chartComponent(map[string]any{
				"name": "chart",
				"git": map[string]any{
					"url": "https://example.com/repo.git",
					"ref": map[string]any{
						"tag":    "1.0.0",
						"branch": "main",
					},
				},
			})),
			valid: false,
		},
		{
			name: "git repository ref rejects branch and commit",
			doc: baseDoc(map[string]any{
				"name": "component",
				"repositories": []any{
					map[string]any{
						"url": "https://example.com/repo.git",
						"ref": map[string]any{
							"branch": "main",
							"commit": "abc123",
						},
					},
				},
			}),
			valid: false,
		},
		{
			name: "component import accepts local sources",
			doc: baseDoc(map[string]any{
				"name": "component",
				"import": map[string]any{
					"local": []any{map[string]any{"path": "component.yaml"}},
				},
			}),
			valid: true,
		},
		{
			name: "component import rejects local and remote sources",
			doc: baseDoc(map[string]any{
				"name": "component",
				"import": map[string]any{
					"local":  []any{map[string]any{"path": "component.yaml"}},
					"remote": []any{map[string]any{"url": "oci://example.com/component"}},
				},
			}),
			valid: false,
		},
		{
			name: "manifest rejects forward path separators",
			doc: baseDoc(map[string]any{
				"name": "component",
				"manifests": []any{map[string]any{
					"name":  "nested/manifest",
					"files": []any{"manifest.yaml"},
				}},
			}),
			valid: false,
		},
		{
			name: "manifest rejects path separators",
			doc: baseDoc(map[string]any{
				"name": "component",
				"manifests": []any{map[string]any{
					"name":  `nested\manifest`,
					"files": []any{"manifest.yaml"},
				}},
			}),
			valid: false,
		},
		{
			name: "chart rejects forward path separators",
			doc: baseDoc(chartComponent(map[string]any{
				"name": "nested/chart",
				"local": map[string]any{
					"path": "chart",
				},
			})),
			valid: false,
		},
		{
			name: "chart rejects backslash path separators",
			doc: baseDoc(chartComponent(map[string]any{
				"name": `nested\chart`,
				"local": map[string]any{
					"path": "chart",
				},
			})),
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(GetV1Beta1Schema(), tt.doc)
			if tt.valid {
				require.NoError(t, err)
				return
			}
			var validationErr *ValidationError
			require.ErrorAs(t, err, &validationErr)
			require.NotEmpty(t, validationErr.Errors)
		})
	}
}

func TestValidateUntypedDocument(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		schema  []byte
		valid   string
		invalid string
	}{
		{
			name:    "package",
			schema:  GetV1Beta1Schema(),
			valid:   "apiVersion: zarf.dev/v1beta1\nkind: ZarfPackageConfig\nmetadata:\n  name: app\ncomponents:\n  - name: app\n",
			invalid: "components.0.images.0.name",
		},
		{
			name:    "component",
			schema:  GetV1Beta1ComponentSchema(),
			valid:   "apiVersion: zarf.dev/v1beta1\nkind: ZarfComponentConfig\nmetadata:\n  name: app\ncomponent:\n  images: []\n",
			invalid: "component.images.0.name",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var document any
			require.NoError(t, goyaml.Unmarshal([]byte(tc.valid), &document))
			require.NoError(t, Validate(tc.schema, document))

			require.NoError(t, goyaml.Unmarshal([]byte(tc.valid+"unknown: true\n"), &document))
			err := Validate(tc.schema, document)
			var validationErr *ValidationError
			require.ErrorAs(t, err, &validationErr)
			require.ErrorContains(t, err, "Additional property unknown is not allowed")

			invalid := strings.ReplaceAll(tc.valid, "  images: []\n", "")
			invalid += "    images:\n      - name: 123\n"
			require.NoError(t, goyaml.Unmarshal([]byte(invalid), &document))
			err = Validate(tc.schema, document)
			require.ErrorAs(t, err, &validationErr)
			require.ErrorContains(t, err, tc.invalid+": Invalid type. Expected: string, given: integer")
		})
	}
}

func TestValidateEngineErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		schema   []byte
		document any
	}{
		{name: "malformed schema", schema: []byte("{"), document: map[string]any{}},
		{name: "unserializable document", schema: GetV1Beta1Schema(), document: make(chan int)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(tc.schema, tc.document)
			require.Error(t, err)
			var validationErr *ValidationError
			require.NotErrorAs(t, err, &validationErr, "engine failures are not schema violations")
		})
	}
}
