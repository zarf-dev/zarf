// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package composer contains functions for composing components within Zarf packages.
package composer

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/variables"
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
		flavor               string
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
	testPackageName := "test-package"
	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewImportChain(testCase.head, 0, testPackageName, testCase.arch, testCase.flavor)
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

	firstDirectory := "hello"
	secondDirectory := "world"
	finalDirectory := filepath.Join(firstDirectory, secondDirectory)

	finalDirectoryActionDefault := filepath.Join(firstDirectory, secondDirectory, "today-dc")
	secondDirectoryActionDefault := filepath.Join(firstDirectory, "world-dc")
	firstDirectoryActionDefault := "hello-dc"

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
				// Files should always be appended with corrected directories
				Files: []types.ZarfFile{
					{Source: fmt.Sprintf("%s%stoday.txt", finalDirectory, string(os.PathSeparator))},
					{Source: fmt.Sprintf("%s%sworld.txt", firstDirectory, string(os.PathSeparator))},
					{Source: "hello.txt"},
				},
				// Charts should be merged if names match and appended if not with corrected directories
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
				// Manifests should be merged if names match and appended if not with corrected directories
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
				// DataInjections should always be appended with corrected directories
				DataInjections: []types.ZarfDataInjection{
					{Source: fmt.Sprintf("%s%stoday", finalDirectory, string(os.PathSeparator))},
					{Source: fmt.Sprintf("%s%sworld", firstDirectory, string(os.PathSeparator))},
					{Source: "hello"},
				},
				Actions: types.ZarfComponentActions{
					// OnCreate actions should be appended with corrected directories that properly handle default directories
					OnCreate: types.ZarfComponentActionSet{
						Defaults: types.ZarfComponentActionDefaults{
							Dir: "hello-dc",
						},
						Before: []types.ZarfComponentAction{
							{Cmd: "today-bc", Dir: &finalDirectoryActionDefault},
							{Cmd: "world-bc", Dir: &secondDirectoryActionDefault},
							{Cmd: "hello-bc", Dir: &firstDirectoryActionDefault},
						},
						After: []types.ZarfComponentAction{
							{Cmd: "today-ac", Dir: &finalDirectoryActionDefault},
							{Cmd: "world-ac", Dir: &secondDirectoryActionDefault},
							{Cmd: "hello-ac", Dir: &firstDirectoryActionDefault},
						},
						OnSuccess: []types.ZarfComponentAction{
							{Cmd: "today-sc", Dir: &finalDirectoryActionDefault},
							{Cmd: "world-sc", Dir: &secondDirectoryActionDefault},
							{Cmd: "hello-sc", Dir: &firstDirectoryActionDefault},
						},
						OnFailure: []types.ZarfComponentAction{
							{Cmd: "today-fc", Dir: &finalDirectoryActionDefault},
							{Cmd: "world-fc", Dir: &secondDirectoryActionDefault},
							{Cmd: "hello-fc", Dir: &firstDirectoryActionDefault},
						},
					},
					// OnDeploy actions should be appended without corrected directories
					OnDeploy: types.ZarfComponentActionSet{
						Defaults: types.ZarfComponentActionDefaults{
							Dir: "hello-dd",
						},
						Before: []types.ZarfComponentAction{
							{Cmd: "today-bd"},
							{Cmd: "world-bd"},
							{Cmd: "hello-bd"},
						},
						After: []types.ZarfComponentAction{
							{Cmd: "today-ad"},
							{Cmd: "world-ad"},
							{Cmd: "hello-ad"},
						},
						OnSuccess: []types.ZarfComponentAction{
							{Cmd: "today-sd"},
							{Cmd: "world-sd"},
							{Cmd: "hello-sd"},
						},
						OnFailure: []types.ZarfComponentAction{
							{Cmd: "today-fd"},
							{Cmd: "world-fd"},
							{Cmd: "hello-fd"},
						},
					},
					// OnRemove actions should be appended without corrected directories
					OnRemove: types.ZarfComponentActionSet{
						Defaults: types.ZarfComponentActionDefaults{
							Dir: "hello-dr",
						},
						Before: []types.ZarfComponentAction{
							{Cmd: "today-br"},
							{Cmd: "world-br"},
							{Cmd: "hello-br"},
						},
						After: []types.ZarfComponentAction{
							{Cmd: "today-ar"},
							{Cmd: "world-ar"},
							{Cmd: "hello-ar"},
						},
						OnSuccess: []types.ZarfComponentAction{
							{Cmd: "today-sr"},
							{Cmd: "world-sr"},
							{Cmd: "hello-sr"},
						},
						OnFailure: []types.ZarfComponentAction{
							{Cmd: "today-fr"},
							{Cmd: "world-fr"},
							{Cmd: "hello-fr"},
						},
					},
				},
				// Extensions should be appended with corrected directories
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
				require.EqualValues(t, &testCase.expectedComposed, composed)
			}
		})
	}
}

func TestMerging(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		ic             *ImportChain
		existingVars   []variables.InteractiveVariable
		existingConsts []variables.Constant
		expectedVars   []variables.InteractiveVariable
		expectedConsts []variables.Constant
	}

	head := Node{
		vars: []variables.InteractiveVariable{
			{
				Variable: variables.Variable{Name: "TEST"},
				Default:  "head",
			},
			{
				Variable: variables.Variable{Name: "HEAD"},
			},
		},
		consts: []variables.Constant{
			{
				Name:  "TEST",
				Value: "head",
			},
			{
				Name: "HEAD",
			},
		},
	}
	tail := Node{
		vars: []variables.InteractiveVariable{
			{
				Variable: variables.Variable{Name: "TEST"},
				Default:  "tail",
			},
			{
				Variable: variables.Variable{Name: "TAIL"},
			},
		},
		consts: []variables.Constant{
			{
				Name:  "TEST",
				Value: "tail",
			},
			{
				Name: "TAIL",
			},
		},
	}
	head.next = &tail
	tail.prev = &head
	testIC := &ImportChain{head: &head, tail: &tail}

	testCases := []testCase{
		{
			name: "empty-ic",
			ic:   &ImportChain{},
			existingVars: []variables.InteractiveVariable{
				{
					Variable: variables.Variable{Name: "TEST"},
				},
			},
			existingConsts: []variables.Constant{
				{
					Name: "TEST",
				},
			},
			expectedVars: []variables.InteractiveVariable{
				{
					Variable: variables.Variable{Name: "TEST"},
				},
			},
			expectedConsts: []variables.Constant{
				{
					Name: "TEST",
				},
			},
		},
		{
			name:           "no-existing",
			ic:             testIC,
			existingVars:   []variables.InteractiveVariable{},
			existingConsts: []variables.Constant{},
			expectedVars: []variables.InteractiveVariable{
				{
					Variable: variables.Variable{Name: "TEST"},
					Default:  "head",
				},
				{
					Variable: variables.Variable{Name: "HEAD"},
				},
				{
					Variable: variables.Variable{Name: "TAIL"},
				},
			},
			expectedConsts: []variables.Constant{
				{
					Name:  "TEST",
					Value: "head",
				},
				{
					Name: "HEAD",
				},
				{
					Name: "TAIL",
				},
			},
		},
		{
			name: "with-existing",
			ic:   testIC,
			existingVars: []variables.InteractiveVariable{
				{
					Variable: variables.Variable{Name: "TEST"},
					Default:  "existing",
				},
				{
					Variable: variables.Variable{Name: "EXISTING"},
				},
			},
			existingConsts: []variables.Constant{
				{
					Name:  "TEST",
					Value: "existing",
				},
				{
					Name: "EXISTING",
				},
			},
			expectedVars: []variables.InteractiveVariable{
				{
					Variable: variables.Variable{Name: "TEST"},
					Default:  "existing",
				},
				{
					Variable: variables.Variable{Name: "EXISTING"},
				},
				{
					Variable: variables.Variable{Name: "HEAD"},
				},
				{
					Variable: variables.Variable{Name: "TAIL"},
				},
			},
			expectedConsts: []variables.Constant{
				{
					Name:  "TEST",
					Value: "existing",
				},
				{
					Name: "EXISTING",
				},
				{
					Name: "HEAD",
				},
				{
					Name: "TAIL",
				},
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			mergedVars := testCase.ic.MergeVariables(testCase.existingVars)
			require.EqualValues(t, testCase.expectedVars, mergedVars)

			mergedConsts := testCase.ic.MergeConstants(testCase.existingConsts)
			require.EqualValues(t, testCase.expectedConsts, mergedConsts)
		})
	}
}

func createChainFromSlice(components []types.ZarfComponent) (ic *ImportChain) {
	ic = &ImportChain{}
	testPackageName := "test-package"

	if len(components) == 0 {
		return ic
	}

	ic.append(components[0], 0, testPackageName, ".", nil, nil)
	history := []string{}

	for idx := 1; idx < len(components); idx++ {
		history = append(history, components[idx-1].Import.Path)
		ic.append(components[idx], idx, testPackageName, filepath.Join(history...), nil, nil)
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
				Defaults: types.ZarfComponentActionDefaults{
					Dir: name + "-dc",
				},
				Before: []types.ZarfComponentAction{
					{Cmd: name + "-bc"},
				},
				After: []types.ZarfComponentAction{
					{Cmd: name + "-ac"},
				},
				OnSuccess: []types.ZarfComponentAction{
					{Cmd: name + "-sc"},
				},
				OnFailure: []types.ZarfComponentAction{
					{Cmd: name + "-fc"},
				},
			},
			OnDeploy: types.ZarfComponentActionSet{
				Defaults: types.ZarfComponentActionDefaults{
					Dir: name + "-dd",
				},
				Before: []types.ZarfComponentAction{
					{Cmd: name + "-bd"},
				},
				After: []types.ZarfComponentAction{
					{Cmd: name + "-ad"},
				},
				OnSuccess: []types.ZarfComponentAction{
					{Cmd: name + "-sd"},
				},
				OnFailure: []types.ZarfComponentAction{
					{Cmd: name + "-fd"},
				},
			},
			OnRemove: types.ZarfComponentActionSet{
				Defaults: types.ZarfComponentActionDefaults{
					Dir: name + "-dr",
				},
				Before: []types.ZarfComponentAction{
					{Cmd: name + "-br"},
				},
				After: []types.ZarfComponentAction{
					{Cmd: name + "-ar"},
				},
				OnSuccess: []types.ZarfComponentAction{
					{Cmd: name + "-sr"},
				},
				OnFailure: []types.ZarfComponentAction{
					{Cmd: name + "-fr"},
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
