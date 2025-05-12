// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestZarfSchema(t *testing.T) {
	t.Parallel()
	zarfSchema, err := os.ReadFile("../../../zarf.schema.json")
	require.NoError(t, err)

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
			findings, err := runSchema(zarfSchema, tt.pkg)
			require.NoError(t, err)
			var schemaStrings []string
			for _, schemaErr := range findings {
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
		schemaErrs, err := runSchema(zarfSchema, unmarshalledYaml)
		require.NoError(t, err)
		var schemaStrings []string
		for _, schemaErr := range schemaErrs {
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

	t.Run("test schema findings is created as expected", func(t *testing.T) {
		t.Parallel()
		findings, err := getSchemaFindings(zarfSchema, v1alpha1.ZarfPackage{
			Kind: v1alpha1.ZarfInitConfig,
			Metadata: v1alpha1.ZarfMetadata{
				Name: "invalid",
			},
		})
		require.NoError(t, err)
		expected := []PackageFinding{
			{
				Description: "Invalid type. Expected: array, given: null",
				Severity:    SevErr,
				YqPath:      ".components",
			},
		}
		require.ElementsMatch(t, expected, findings)
	})
}

func TestValidatePackageSchema(t *testing.T) {
	ZarfSchema = testutil.LoadSchema(t, "../../../zarf.schema.json")
	setVariables := map[string]string{
		"PACKAGE_NAME": "test-package",
		"MY_COMP_NAME": "test-comp",
	}
	cwd, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(filepath.Join("testdata", "package-with-templates"))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.Chdir(cwd))
	}()
	findings, err := ValidatePackageSchema(setVariables)
	require.Empty(t, findings)
	require.NoError(t, err)
}

func TestYqCompat(t *testing.T) {
	t.Parallel()
	t.Run("Wrap standalone numbers in bracket", func(t *testing.T) {
		t.Parallel()
		input := "components12.12.import.path"
		expected := ".components12.[12].import.path"
		actual := makeFieldPathYqCompat(input)
		require.Equal(t, expected, actual)
	})

	t.Run("root doesn't change", func(t *testing.T) {
		t.Parallel()
		input := "(root)"
		actual := makeFieldPathYqCompat(input)
		require.Equal(t, input, actual)
	})
}

func TestFillObjTemplate(t *testing.T) {
	testCases := []struct {
		name              string
		variables         map[string]string
		component         v1alpha1.ZarfComponent
		expectedComponent v1alpha1.ZarfComponent
	}{
		{
			name: "template variables",
			variables: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			component: v1alpha1.ZarfComponent{
				Images: []string{
					fmt.Sprintf("%s%s###", v1alpha1.ZarfPackageTemplatePrefix, "KEY1"),
					fmt.Sprintf("%s%s###", v1alpha1.ZarfPackageVariablePrefix, "KEY2"),
					fmt.Sprintf("%s%s###", v1alpha1.ZarfPackageTemplatePrefix, "KEY3"),
				},
			},
			expectedComponent: v1alpha1.ZarfComponent{
				Images: []string{
					"value1",
					"value2",
					fmt.Sprintf("%s%s###", v1alpha1.ZarfPackageTemplatePrefix, "KEY3"),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			component := tc.component
			if err := templateZarfObj(&component, tc.variables); err != nil {
				require.NoError(t, err)
			}

			require.Equal(t, tc.expectedComponent, component)
		})
	}
}
