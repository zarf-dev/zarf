// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package composer contains functions for composing components within Zarf packages.
package composer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1alpha1/extensions"
)

func TestNewImportChain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		head        v1alpha1.ZarfComponent
		arch        string
		flavor      string
		expectedErr string
	}{
		{
			name:        "No Architecture",
			head:        v1alpha1.ZarfComponent{},
			expectedErr: "architecture must be provided",
		},
		{
			name: "Circular Import",
			head: v1alpha1.ZarfComponent{
				Import: v1alpha1.ZarfComponentImport{
					Path: ".",
				},
			},
			arch:        "amd64",
			expectedErr: "detected circular import chain",
		},
	}
	testPackageName := "test-package"
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewImportChain(context.Background(), tt.head, 0, testPackageName, tt.arch, tt.flavor)
			require.ErrorContains(t, err, tt.expectedErr)
		})
	}
}

func TestCompose(t *testing.T) {
	t.Parallel()

	firstDirectory := "hello"
	secondDirectory := "world"
	finalDirectory := filepath.Join(firstDirectory, secondDirectory)

	finalDirectoryActionDefault := filepath.Join(firstDirectory, secondDirectory, "today-dc")
	secondDirectoryActionDefault := filepath.Join(firstDirectory, "world-dc")
	firstDirectoryActionDefault := "hello-dc"

	tests := []struct {
		name             string
		ic               *ImportChain
		expectedComposed v1alpha1.ZarfComponent
	}{
		{
			name: "Single Component",
			ic: createChainFromSlice(t, []v1alpha1.ZarfComponent{
				{
					Name: "no-import",
				},
			}),
			expectedComposed: v1alpha1.ZarfComponent{
				Name: "no-import",
			},
		},
		{
			name: "Multiple Components",
			ic: createChainFromSlice(t, []v1alpha1.ZarfComponent{
				createDummyComponent(t, "hello", firstDirectory, "hello"),
				createDummyComponent(t, "world", secondDirectory, "world"),
				createDummyComponent(t, "today", "", "hello"),
			}),
			expectedComposed: v1alpha1.ZarfComponent{
				Name: "import-hello",
				// Files should always be appended with corrected directories
				Files: []v1alpha1.ZarfFile{
					{Source: fmt.Sprintf("%s%stoday.txt", finalDirectory, string(os.PathSeparator))},
					{Source: fmt.Sprintf("%s%sworld.txt", firstDirectory, string(os.PathSeparator))},
					{Source: "hello.txt"},
				},
				// Charts should be merged if names match and appended if not with corrected directories
				Charts: []v1alpha1.ZarfChart{
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
				Manifests: []v1alpha1.ZarfManifest{
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
				DataInjections: []v1alpha1.ZarfDataInjection{
					{Source: fmt.Sprintf("%s%stoday", finalDirectory, string(os.PathSeparator))},
					{Source: fmt.Sprintf("%s%sworld", firstDirectory, string(os.PathSeparator))},
					{Source: "hello"},
				},
				Actions: v1alpha1.ZarfComponentActions{
					// OnCreate actions should be appended with corrected directories that properly handle default directories
					OnCreate: v1alpha1.ZarfComponentActionSet{
						Defaults: v1alpha1.ZarfComponentActionDefaults{
							Dir: "hello-dc",
						},
						Before: []v1alpha1.ZarfComponentAction{
							{Cmd: "today-bc", Dir: &finalDirectoryActionDefault},
							{Cmd: "world-bc", Dir: &secondDirectoryActionDefault},
							{Cmd: "hello-bc", Dir: &firstDirectoryActionDefault},
						},
						After: []v1alpha1.ZarfComponentAction{
							{Cmd: "today-ac", Dir: &finalDirectoryActionDefault},
							{Cmd: "world-ac", Dir: &secondDirectoryActionDefault},
							{Cmd: "hello-ac", Dir: &firstDirectoryActionDefault},
						},
						OnSuccess: []v1alpha1.ZarfComponentAction{
							{Cmd: "today-sc", Dir: &finalDirectoryActionDefault},
							{Cmd: "world-sc", Dir: &secondDirectoryActionDefault},
							{Cmd: "hello-sc", Dir: &firstDirectoryActionDefault},
						},
						OnFailure: []v1alpha1.ZarfComponentAction{
							{Cmd: "today-fc", Dir: &finalDirectoryActionDefault},
							{Cmd: "world-fc", Dir: &secondDirectoryActionDefault},
							{Cmd: "hello-fc", Dir: &firstDirectoryActionDefault},
						},
					},
					// OnDeploy actions should be appended without corrected directories
					OnDeploy: v1alpha1.ZarfComponentActionSet{
						Defaults: v1alpha1.ZarfComponentActionDefaults{
							Dir: "hello-dd",
						},
						Before: []v1alpha1.ZarfComponentAction{
							{Cmd: "today-bd"},
							{Cmd: "world-bd"},
							{Cmd: "hello-bd"},
						},
						After: []v1alpha1.ZarfComponentAction{
							{Cmd: "today-ad"},
							{Cmd: "world-ad"},
							{Cmd: "hello-ad"},
						},
						OnSuccess: []v1alpha1.ZarfComponentAction{
							{Cmd: "today-sd"},
							{Cmd: "world-sd"},
							{Cmd: "hello-sd"},
						},
						OnFailure: []v1alpha1.ZarfComponentAction{
							{Cmd: "today-fd"},
							{Cmd: "world-fd"},
							{Cmd: "hello-fd"},
						},
					},
					// OnRemove actions should be appended without corrected directories
					OnRemove: v1alpha1.ZarfComponentActionSet{
						Defaults: v1alpha1.ZarfComponentActionDefaults{
							Dir: "hello-dr",
						},
						Before: []v1alpha1.ZarfComponentAction{
							{Cmd: "today-br"},
							{Cmd: "world-br"},
							{Cmd: "hello-br"},
						},
						After: []v1alpha1.ZarfComponentAction{
							{Cmd: "today-ar"},
							{Cmd: "world-ar"},
							{Cmd: "hello-ar"},
						},
						OnSuccess: []v1alpha1.ZarfComponentAction{
							{Cmd: "today-sr"},
							{Cmd: "world-sr"},
							{Cmd: "hello-sr"},
						},
						OnFailure: []v1alpha1.ZarfComponentAction{
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
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			composed, err := tt.ic.Compose(context.Background())
			require.NoError(t, err)
			require.EqualValues(t, &tt.expectedComposed, composed)
		})
	}
}

func TestMerging(t *testing.T) {
	t.Parallel()

	head := Node{
		vars: []v1alpha1.InteractiveVariable{
			{
				Variable: v1alpha1.Variable{Name: "TEST"},
				Default:  "head",
			},
			{
				Variable: v1alpha1.Variable{Name: "HEAD"},
			},
		},
		consts: []v1alpha1.Constant{
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
		vars: []v1alpha1.InteractiveVariable{
			{
				Variable: v1alpha1.Variable{Name: "TEST"},
				Default:  "tail",
			},
			{
				Variable: v1alpha1.Variable{Name: "TAIL"},
			},
		},
		consts: []v1alpha1.Constant{
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

	tests := []struct {
		name           string
		ic             *ImportChain
		existingVars   []v1alpha1.InteractiveVariable
		existingConsts []v1alpha1.Constant
		expectedVars   []v1alpha1.InteractiveVariable
		expectedConsts []v1alpha1.Constant
	}{
		{
			name: "empty-ic",
			ic:   &ImportChain{},
			existingVars: []v1alpha1.InteractiveVariable{
				{
					Variable: v1alpha1.Variable{Name: "TEST"},
				},
			},
			existingConsts: []v1alpha1.Constant{
				{
					Name: "TEST",
				},
			},
			expectedVars: []v1alpha1.InteractiveVariable{
				{
					Variable: v1alpha1.Variable{Name: "TEST"},
				},
			},
			expectedConsts: []v1alpha1.Constant{
				{
					Name: "TEST",
				},
			},
		},
		{
			name:           "no-existing",
			ic:             testIC,
			existingVars:   []v1alpha1.InteractiveVariable{},
			existingConsts: []v1alpha1.Constant{},
			expectedVars: []v1alpha1.InteractiveVariable{
				{
					Variable: v1alpha1.Variable{Name: "TEST"},
					Default:  "head",
				},
				{
					Variable: v1alpha1.Variable{Name: "HEAD"},
				},
				{
					Variable: v1alpha1.Variable{Name: "TAIL"},
				},
			},
			expectedConsts: []v1alpha1.Constant{
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
			existingVars: []v1alpha1.InteractiveVariable{
				{
					Variable: v1alpha1.Variable{Name: "TEST"},
					Default:  "existing",
				},
				{
					Variable: v1alpha1.Variable{Name: "EXISTING"},
				},
			},
			existingConsts: []v1alpha1.Constant{
				{
					Name:  "TEST",
					Value: "existing",
				},
				{
					Name: "EXISTING",
				},
			},
			expectedVars: []v1alpha1.InteractiveVariable{
				{
					Variable: v1alpha1.Variable{Name: "TEST"},
					Default:  "existing",
				},
				{
					Variable: v1alpha1.Variable{Name: "EXISTING"},
				},
				{
					Variable: v1alpha1.Variable{Name: "HEAD"},
				},
				{
					Variable: v1alpha1.Variable{Name: "TAIL"},
				},
			},
			expectedConsts: []v1alpha1.Constant{
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

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mergedVars := tt.ic.MergeVariables(tt.existingVars)
			require.EqualValues(t, tt.expectedVars, mergedVars)

			mergedConsts := tt.ic.MergeConstants(tt.existingConsts)
			require.EqualValues(t, tt.expectedConsts, mergedConsts)
		})
	}
}

func createChainFromSlice(t *testing.T, components []v1alpha1.ZarfComponent) (ic *ImportChain) {
	t.Helper()

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

func createDummyComponent(t *testing.T, name, importDir, subName string) v1alpha1.ZarfComponent {
	t.Helper()

	return v1alpha1.ZarfComponent{
		Name: fmt.Sprintf("import-%s", name),
		Import: v1alpha1.ZarfComponentImport{
			Path: importDir,
		},
		Files: []v1alpha1.ZarfFile{
			{
				Source: fmt.Sprintf("%s.txt", name),
			},
		},
		Charts: []v1alpha1.ZarfChart{
			{
				Name:      subName,
				LocalPath: "chart",
				ValuesFiles: []string{
					"values.yaml",
				},
			},
		},
		Manifests: []v1alpha1.ZarfManifest{
			{
				Name: subName,
				Files: []string{
					"manifest.yaml",
				},
			},
		},
		DataInjections: []v1alpha1.ZarfDataInjection{
			{
				Source: name,
			},
		},
		Actions: v1alpha1.ZarfComponentActions{
			OnCreate: v1alpha1.ZarfComponentActionSet{
				Defaults: v1alpha1.ZarfComponentActionDefaults{
					Dir: name + "-dc",
				},
				Before: []v1alpha1.ZarfComponentAction{
					{Cmd: name + "-bc"},
				},
				After: []v1alpha1.ZarfComponentAction{
					{Cmd: name + "-ac"},
				},
				OnSuccess: []v1alpha1.ZarfComponentAction{
					{Cmd: name + "-sc"},
				},
				OnFailure: []v1alpha1.ZarfComponentAction{
					{Cmd: name + "-fc"},
				},
			},
			OnDeploy: v1alpha1.ZarfComponentActionSet{
				Defaults: v1alpha1.ZarfComponentActionDefaults{
					Dir: name + "-dd",
				},
				Before: []v1alpha1.ZarfComponentAction{
					{Cmd: name + "-bd"},
				},
				After: []v1alpha1.ZarfComponentAction{
					{Cmd: name + "-ad"},
				},
				OnSuccess: []v1alpha1.ZarfComponentAction{
					{Cmd: name + "-sd"},
				},
				OnFailure: []v1alpha1.ZarfComponentAction{
					{Cmd: name + "-fd"},
				},
			},
			OnRemove: v1alpha1.ZarfComponentActionSet{
				Defaults: v1alpha1.ZarfComponentActionDefaults{
					Dir: name + "-dr",
				},
				Before: []v1alpha1.ZarfComponentAction{
					{Cmd: name + "-br"},
				},
				After: []v1alpha1.ZarfComponentAction{
					{Cmd: name + "-ar"},
				},
				OnSuccess: []v1alpha1.ZarfComponentAction{
					{Cmd: name + "-sr"},
				},
				OnFailure: []v1alpha1.ZarfComponentAction{
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
