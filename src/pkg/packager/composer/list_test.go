// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package composer contains functions for composing components within Zarf packages.
package composer

import (
	"fmt"
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

	importDirectory := "hello"
	currentDirectory := "."

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
				{
					Name: "import-hello",
					Import: types.ZarfComponentImport{
						Path:          importDirectory,
						ComponentName: "end-world",
					},
					Files: []types.ZarfFile{
						{
							Source: "hello.txt",
						},
					},
					Charts: []types.ZarfChart{
						{
							Name:      "hello",
							LocalPath: "chart",
							ValuesFiles: []string{
								"values.yaml",
							},
						},
					},
					Manifests: []types.ZarfManifest{
						{
							Name: "hello",
							Files: []string{
								"manifest.yaml",
							},
						},
					},
					DataInjections: []types.ZarfDataInjection{
						{
							Source: "hello",
						},
					},
					Actions: types.ZarfComponentActions{
						OnCreate: types.ZarfComponentActionSet{
							Before: []types.ZarfComponentAction{
								{
									Cmd: "hello",
								},
							},
							After: []types.ZarfComponentAction{
								{
									Cmd: "hello",
								},
							},
							OnSuccess: []types.ZarfComponentAction{
								{
									Cmd: "hello",
								},
							},
							OnFailure: []types.ZarfComponentAction{
								{
									Cmd: "hello",
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
				},
				{
					Name: "end-world",
					Files: []types.ZarfFile{
						{
							Source: "world.txt",
						},
					},
					Charts: []types.ZarfChart{
						{
							Name:      "world",
							LocalPath: "chart",
							ValuesFiles: []string{
								"values.yaml",
							},
						},
					},
					Manifests: []types.ZarfManifest{
						{
							Name: "world",
							Files: []string{
								"manifest.yaml",
							},
						},
					},
					DataInjections: []types.ZarfDataInjection{
						{
							Source: "world",
						},
					},
					Actions: types.ZarfComponentActions{
						OnCreate: types.ZarfComponentActionSet{
							Before: []types.ZarfComponentAction{
								{
									Cmd: "world",
								},
							},
							After: []types.ZarfComponentAction{
								{
									Cmd: "world",
								},
							},
							OnSuccess: []types.ZarfComponentAction{
								{
									Cmd: "world",
								},
							},
							OnFailure: []types.ZarfComponentAction{
								{
									Cmd: "world",
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
				},
			}),
			returnError: false,
			expectedComposed: types.ZarfComponent{
				Name: "import-hello",
				Files: []types.ZarfFile{
					{
						Source: fmt.Sprintf("%s/world.txt", importDirectory),
					},
					{
						Source: "hello.txt",
					},
				},
				Charts: []types.ZarfChart{
					{
						Name:      "world",
						LocalPath: fmt.Sprintf("%s/chart", importDirectory),
						ValuesFiles: []string{
							fmt.Sprintf("%s/values.yaml", importDirectory),
						},
					},
					{
						Name:      "hello",
						LocalPath: "chart",
						ValuesFiles: []string{
							"values.yaml",
						},
					},
				},
				Manifests: []types.ZarfManifest{
					{
						Name: "world",
						Files: []string{
							fmt.Sprintf("%s/manifest.yaml", importDirectory),
						},
					},
					{
						Name: "hello",
						Files: []string{
							"manifest.yaml",
						},
					},
				},
				DataInjections: []types.ZarfDataInjection{
					{
						Source: fmt.Sprintf("%s/world", importDirectory),
					},
					{
						Source: "hello",
					},
				},
				Actions: types.ZarfComponentActions{
					OnCreate: types.ZarfComponentActionSet{
						Before: []types.ZarfComponentAction{
							{
								Cmd: "world",
								Dir: &importDirectory,
							},
							{
								Cmd: "hello",
								Dir: &currentDirectory,
							},
						},
						After: []types.ZarfComponentAction{
							{
								Cmd: "world",
								Dir: &importDirectory,
							},
							{
								Cmd: "hello",
								Dir: &currentDirectory,
							},
						},
						OnSuccess: []types.ZarfComponentAction{
							{
								Cmd: "world",
								Dir: &importDirectory,
							},
							{
								Cmd: "hello",
								Dir: &currentDirectory,
							},
						},
						OnFailure: []types.ZarfComponentAction{
							{
								Cmd: "world",
								Dir: &importDirectory,
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
							fmt.Sprintf("%s/values.yaml", importDirectory),
							"values.yaml",
						},
						FluxPatchFiles: []string{
							fmt.Sprintf("%s/patch.yaml", importDirectory),
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
