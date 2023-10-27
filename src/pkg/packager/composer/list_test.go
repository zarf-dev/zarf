// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package composer contains functions for composing components within Zarf packages.
package composer

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/defenseunicorns/zarf/src/types/extensions"
	"github.com/stretchr/testify/require"
)

func TestNewImportChain(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                 string
		head                 types.ZarfComponent
		arch                 string
		expectedErrorMessage string
	}

	testCases := []testCase{
		{
			name:                 "No Architecture",
			head:                 types.ZarfComponent{},
			expectedErrorMessage: "architecture must be provided",
		},
		{
			name: "Circular Import",
			head: types.ZarfComponent{
				Import: types.ZarfComponentImport{
					Path: ".",
				},
			},
			arch:                 "amd64",
			expectedErrorMessage: "detected circular import chain",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewImportChain(testCase.head, testCase.arch)
			require.Contains(t, err.Error(), testCase.expectedErrorMessage)
		})
	}
}

func TestCompose(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                 string
		ic                   *ImportChain
		returnError          bool
		expectedComposed     types.ZarfComponent
		expectedErrorMessage string
	}

	currentDirectory := "."
	firstDirectory := "hello"
	secondDirectory := "world"
	finalDirectory := filepath.Join(firstDirectory, secondDirectory)

	testCases := []testCase{
		{
			name: "Single Component",
			ic: createChainFromSlice([]types.ZarfComponent{
				{
					Name: "no-import",
				},
			}),
			returnError: false,
			expectedComposed: types.ZarfComponent{
				Name: "no-import",
			},
		},
		{
			name: "Multiple Components",
			ic: createChainFromSlice([]types.ZarfComponent{
				createDummyComponent("hello", firstDirectory, "hello"),
				createDummyComponent("world", secondDirectory, "world"),
				createDummyComponent("today", "", "hello"),
			}),
			returnError: false,
			expectedComposed: types.ZarfComponent{
				Name: "import-hello",
				Files: []types.ZarfFile{
					{
						Source: fmt.Sprintf("%s%stoday.txt", finalDirectory, string(os.PathSeparator)),
					},
					{
						Source: fmt.Sprintf("%s%sworld.txt", firstDirectory, string(os.PathSeparator)),
					},
					{
						Source: "hello.txt",
					},
				},
				Charts: []types.ZarfChart{
					{
						Name:      "hello",
						LocalPath: fmt.Sprintf("%s%schart", finalDirectory, string(os.PathSeparator)),
						ValuesFiles: []string{
							fmt.Sprintf("%s%svalues.yaml", finalDirectory, string(os.PathSeparator)),
							"values.yaml",
						},
					},
					{
						Name:      "world",
						LocalPath: fmt.Sprintf("%s%schart", firstDirectory, string(os.PathSeparator)),
						ValuesFiles: []string{
							fmt.Sprintf("%s%svalues.yaml", firstDirectory, string(os.PathSeparator)),
						},
					},
				},
				Manifests: []types.ZarfManifest{
					{
						Name: "hello",
						Files: []string{
							fmt.Sprintf("%s%smanifest.yaml", finalDirectory, string(os.PathSeparator)),
							"manifest.yaml",
						},
					},
					{
						Name: "world",
						Files: []string{
							fmt.Sprintf("%s%smanifest.yaml", firstDirectory, string(os.PathSeparator)),
						},
					},
				},
				DataInjections: []types.ZarfDataInjection{
					{
						Source: fmt.Sprintf("%s%stoday", finalDirectory, string(os.PathSeparator)),
					},
					{
						Source: fmt.Sprintf("%s%sworld", firstDirectory, string(os.PathSeparator)),
					},
					{
						Source: "hello",
					},
				},
				Actions: types.ZarfComponentActions{
					OnCreate: types.ZarfComponentActionSet{
						Before: []types.ZarfComponentAction{
							{
								Cmd: "today",
								Dir: &finalDirectory,
							},
							{
								Cmd: "world",
								Dir: &firstDirectory,
							},
							{
								Cmd: "hello",
								Dir: &currentDirectory,
							},
						},
						After: []types.ZarfComponentAction{
							{
								Cmd: "today",
								Dir: &finalDirectory,
							},
							{
								Cmd: "world",
								Dir: &firstDirectory,
							},
							{
								Cmd: "hello",
								Dir: &currentDirectory,
							},
						},
						OnSuccess: []types.ZarfComponentAction{
							{
								Cmd: "today",
								Dir: &finalDirectory,
							},
							{
								Cmd: "world",
								Dir: &firstDirectory,
							},
							{
								Cmd: "hello",
								Dir: &currentDirectory,
							},
						},
						OnFailure: []types.ZarfComponentAction{
							{
								Cmd: "today",
								Dir: &finalDirectory,
							},
							{
								Cmd: "world",
								Dir: &firstDirectory,
							},
							{
								Cmd: "hello",
								Dir: &currentDirectory,
							},
						},
					},
				},
				Extensions: extensions.ZarfComponentExtensions{
					BigBang: &extensions.BigBang{
						ValuesFiles: []string{
							fmt.Sprintf("%s%svalues.yaml", finalDirectory, string(os.PathSeparator)),
							fmt.Sprintf("%s%svalues.yaml", firstDirectory, string(os.PathSeparator)),
							"values.yaml",
						},
						FluxPatchFiles: []string{
							fmt.Sprintf("%s%spatch.yaml", finalDirectory, string(os.PathSeparator)),
							fmt.Sprintf("%s%spatch.yaml", firstDirectory, string(os.PathSeparator)),
							"patch.yaml",
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			composed, err := testCase.ic.Compose()
			if testCase.returnError {
				require.Contains(t, err.Error(), testCase.expectedErrorMessage)
			} else {
				require.EqualValues(t, testCase.expectedComposed, composed)
			}
		})
	}
}

func createChainFromSlice(components []types.ZarfComponent) (ic *ImportChain) {
	ic = &ImportChain{}

	if len(components) == 0 {
		return ic
	}

	ic.append(components[0], ".", nil, nil)
	history := []string{}

	for idx := 1; idx < len(components); idx++ {
		history = append(history, components[idx-1].Import.Path)
		ic.append(components[idx], filepath.Join(history...), nil, nil)
	}

	return ic
}

func createDummyComponent(name, importDir, subName string) types.ZarfComponent {
	return types.ZarfComponent{
		Name: fmt.Sprintf("import-%s", name),
		Import: types.ZarfComponentImport{
			Path: importDir,
		},
		Files: []types.ZarfFile{
			{
				Source: fmt.Sprintf("%s.txt", name),
			},
		},
		Charts: []types.ZarfChart{
			{
				Name:      subName,
				LocalPath: "chart",
				ValuesFiles: []string{
					"values.yaml",
				},
			},
		},
		Manifests: []types.ZarfManifest{
			{
				Name: subName,
				Files: []string{
					"manifest.yaml",
				},
			},
		},
		DataInjections: []types.ZarfDataInjection{
			{
				Source: name,
			},
		},
		Actions: types.ZarfComponentActions{
			OnCreate: types.ZarfComponentActionSet{
				Before: []types.ZarfComponentAction{
					{
						Cmd: name,
					},
				},
				After: []types.ZarfComponentAction{
					{
						Cmd: name,
					},
				},
				OnSuccess: []types.ZarfComponentAction{
					{
						Cmd: name,
					},
				},
				OnFailure: []types.ZarfComponentAction{
					{
						Cmd: name,
					},
				},
			},
		},
		Extensions: extensions.ZarfComponentExtensions{
			BigBang: &extensions.BigBang{
				ValuesFiles: []string{
					"values.yaml",
				},
				FluxPatchFiles: []string{
					"patch.yaml",
				},
			},
		},
	}
}
