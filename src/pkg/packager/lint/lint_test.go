// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/variables"
	"github.com/defenseunicorns/zarf/src/types"
	goyaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
)

func TestZarfSchema(t *testing.T) {
	t.Parallel()
	zarfSchema, err := os.ReadFile("../../../../zarf.schema.json")
	require.NoError(t, err)

	tests := []struct {
		name                  string
		pkg                   types.ZarfPackage
		expectedSchemaStrings []string
	}{
		{
			name: "valid package",
			pkg: types.ZarfPackage{
				Kind: types.ZarfInitConfig,
				Metadata: types.ZarfMetadata{
					Name: "valid-name",
				},
				Components: []types.ZarfComponent{
					{
						Name: "valid-comp",
					},
				},
			},
			expectedSchemaStrings: nil,
		},
		{
			name: "no comp or kind",
			pkg: types.ZarfPackage{
				Metadata: types.ZarfMetadata{
					Name: "no-comp-or-kind",
				},
				Components: []types.ZarfComponent{},
			},
			expectedSchemaStrings: []string{
				"kind: kind must be one of the following: \"ZarfInitConfig\", \"ZarfPackageConfig\"",
				"components: Array must have at least 1 items",
			},
		},
		{
			name: "invalid package",
			pkg: types.ZarfPackage{
				Kind: types.ZarfInitConfig,
				Metadata: types.ZarfMetadata{
					Name: "-invalid-name",
				},
				Components: []types.ZarfComponent{
					{
						Name: "invalid-name",
						Only: types.ZarfComponentOnlyTarget{
							LocalOS: "unsupportedOS",
						},
						Import: types.ZarfComponentImport{
							Path: fmt.Sprintf("start%send", types.ZarfPackageTemplatePrefix),
							URL:  fmt.Sprintf("oci://start%send", types.ZarfPackageTemplatePrefix),
						},
					},
					{
						Name: "actions",
						Actions: types.ZarfComponentActions{
							OnCreate: types.ZarfComponentActionSet{
								Before: []types.ZarfComponentAction{
									{
										Cmd:          "echo 'invalid setVariable'",
										SetVariables: []variables.Variable{{Name: "not_uppercase"}},
									},
								},
							},
							OnRemove: types.ZarfComponentActionSet{
								OnSuccess: []types.ZarfComponentAction{
									{
										Cmd:          "echo 'invalid setVariable'",
										SetVariables: []variables.Variable{{Name: "not_uppercase"}},
									},
								},
							},
						},
					},
				},
				Variables: []variables.InteractiveVariable{
					{
						Variable: variables.Variable{Name: "not_uppercase"},
					},
				},
				Constants: []variables.Constant{
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
				"components.0.import.path: Must not validate the schema (not)",
				"components.0.import.url: Must not validate the schema (not)",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
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
		findings, err := validateSchema(zarfSchema, types.ZarfPackage{
			Kind: types.ZarfInitConfig,
			Metadata: types.ZarfMetadata{
				Name: "invalid",
			},
		})
		require.NoError(t, err)
		expected := []types.PackageFinding{
			{
				Description: "Invalid type. Expected: array, given: null",
				Severity:    types.SevErr,
				YqPath:      ".components",
			},
		}
		require.ElementsMatch(t, expected, findings)
	})
}

func TestValidateComponent(t *testing.T) {
	t.Parallel()

	t.Run("Unpinnned repo warning", func(t *testing.T) {
		t.Parallel()
		unpinnedRepo := "https://github.com/defenseunicorns/zarf-public-test.git"
		component := types.ZarfComponent{Repos: []string{
			unpinnedRepo,
			"https://dev.azure.com/defenseunicorns/zarf-public-test/_git/zarf-public-test@v0.0.1",
		}}
		findings := checkForUnpinnedRepos(component, 0)
		expected := []types.PackageFinding{
			{
				Item:        unpinnedRepo,
				Description: "Unpinned repository",
				Severity:    types.SevWarn,
				YqPath:      ".components.[0].repos.[0]",
			},
		}
		require.Equal(t, expected, findings)
	})

	t.Run("Unpinnned image warning", func(t *testing.T) {
		t.Parallel()
		unpinnedImage := "registry.com:9001/whatever/image:1.0.0"
		badImage := "badimage:badimage@@sha256:3fbc632167424a6d997e74f5"
		cosignedImage := "ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig"
		component := types.ZarfComponent{Images: []string{
			unpinnedImage,
			"busybox:latest@sha256:3fbc632167424a6d997e74f52b878d7cc478225cffac6bc977eedfe51c7f4e79",
			badImage,
			cosignedImage,
		}}
		findings := checkForUnpinnedImages(component, 0)
		expected := []types.PackageFinding{
			{
				Item:        unpinnedImage,
				Description: "Image not pinned with digest",
				Severity:    types.SevWarn,
				YqPath:      ".components.[0].images.[0]",
			},
			{
				Item:        badImage,
				Description: "Failed to parse image reference",
				Severity:    types.SevWarn,
				YqPath:      ".components.[0].images.[2]",
			},
		}
		require.Equal(t, expected, findings)
	})

	t.Run("Unpinnned file warning", func(t *testing.T) {
		t.Parallel()
		fileURL := "http://example.com/file.zip"
		localFile := "local.txt"
		zarfFiles := []types.ZarfFile{
			{
				Source: fileURL,
			},
			{
				Source: localFile,
			},
			{
				Source: fileURL,
				Shasum: "fake-shasum",
			},
		}
		component := types.ZarfComponent{Files: zarfFiles}
		findings := checkForUnpinnedFiles(component, 0)
		expectedErr := []types.PackageFinding{
			{
				Item:        fileURL,
				Description: "No shasum for remote file",
				Severity:    types.SevWarn,
				YqPath:      ".components.[0].files.[0]",
			},
		}
		require.Equal(t, expectedErr, findings)
		require.Len(t, findings, 1)
	})

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

	t.Run("Test composable components with bad path", func(t *testing.T) {
		t.Parallel()
		zarfPackage := types.ZarfPackage{
			Components: []types.ZarfComponent{
				{
					Import: types.ZarfComponentImport{Path: "bad-path"},
				},
			},
			Metadata: types.ZarfMetadata{Name: "test-zarf-package"},
		}

		createOpts := types.ZarfCreateOptions{Flavor: "", BaseDir: "."}
		_, err := lintComponents(context.Background(), zarfPackage, createOpts)
		require.Error(t, err)
	})

	t.Run("isImagePinned", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			input    string
			expected bool
			err      error
		}{
			{
				input:    "registry.com:8080/defenseunicorns/whatever",
				expected: false,
				err:      nil,
			},
			{
				input:    "ghcr.io/defenseunicorns/pepr/controller:v0.15.0",
				expected: false,
				err:      nil,
			},
			{
				input:    "busybox:latest@sha256:3fbc632167424a6d997e74f52b878d7cc478225cffac6bc977eedfe51c7f4e79",
				expected: true,
				err:      nil,
			},
			{
				input:    "busybox:bad/image",
				expected: false,
				err:      errors.New("invalid reference format"),
			},
			{
				input:    "busybox:###ZARF_PKG_TMPL_BUSYBOX_IMAGE###",
				expected: true,
				err:      nil,
			},
			{
				input:    "ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig",
				expected: true,
				err:      nil,
			},
		}
		for _, tc := range tests {
			t.Run(tc.input, func(t *testing.T) {
				actual, err := isPinnedImage(tc.input)
				if err != nil {
					require.EqualError(t, err, tc.err.Error())
				}
				require.Equal(t, tc.expected, actual)
			})
		}
	})
}
