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

	"github.com/defenseunicorns/zarf/src/pkg/variables"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/defenseunicorns/zarf/src/types/extensions"
	"github.com/stretchr/testify/require"
)

func TestNewImportChain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		head        types.ZarfComponent
		arch        string
		flavor      string
		expectedErr string
	}{
		{
			name:        "No Architecture",
			head:        types.ZarfComponent{},
			expectedErr: "architecture must be provided",
		},
		{
			name: "Circular Import",
			head: types.ZarfComponent{
				Import: types.ZarfComponentImport{
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

	tests := []struct {
		name             string
		ic               *ImportChain
		expectedComposed types.ZarfComponent
	}{
		{
			name: "Single Component",
			ic: createChainFromSlice(t, []types.ZarfComponent{
				{
					Name: "no-import",
				},
			}),
			expectedComposed: types.ZarfComponent{
				Name: "no-import",
			},
		},
		{
			name: "Multiple Components",
			ic: createChainFromSlice(t, []types.ZarfComponent{
				createDummyComponent(t, "hello", firstDirectory, "hello"),
				createDummyComponent(t, "world", secondDirectory, "world"),
				createDummyComponent(t, "today", "", "hello"),
			}),
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

	tests := []struct {
		name           string
		ic             *ImportChain
		existingVars   []variables.InteractiveVariable
		existingConsts []variables.Constant
		expectedVars   []variables.InteractiveVariable
		expectedConsts []variables.Constant
	}{
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

func createChainFromSlice(t *testing.T, components []types.ZarfComponent) (ic *ImportChain) {
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

func createDummyComponent(t *testing.T, name, importDir, subName string) types.ZarfComponent {
	t.Helper()

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
