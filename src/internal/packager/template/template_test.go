// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package template provides functions for templating yaml files.
package template

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

func TestGetSanitizedTemplateMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]*variables.TextTemplate
		expected map[string]string
	}{
		{
			name: "Sensitive entry",
			input: map[string]*variables.TextTemplate{
				"###SENSITIVE###": {Sensitive: true, Value: "secret"},
			},
			expected: map[string]string{
				"###SENSITIVE###": "**sanitized**",
			},
		},
		{
			name: "Non-sensitive entries",
			input: map[string]*variables.TextTemplate{
				"###VARIABLE###": {Sensitive: false, Value: "value"},
			},
			expected: map[string]string{
				"###VARIABLE###": "value",
			},
		},
		{
			name: "Sensitive and non-sensitive entries",
			input: map[string]*variables.TextTemplate{
				"###ZARF_GIT_AUTH_PULL###": {Sensitive: true, Value: "secret1"},
				"###ZARF_GIT_AUTH_PUSH###": {Sensitive: true, Value: "secret2"},
				"###ZARF_GIT_PUSH###":      {Sensitive: false, Value: "zarf-git-user"},
				"###ZARF_GIT_PULL###":      {Sensitive: false, Value: "zarf-git-read-user"},
			},
			expected: map[string]string{
				"###ZARF_GIT_AUTH_PULL###": "**sanitized**",
				"###ZARF_GIT_AUTH_PUSH###": "**sanitized**",
				"###ZARF_GIT_PULL###":      "zarf-git-read-user",
				"###ZARF_GIT_PUSH###":      "zarf-git-user",
			},
		},
		{
			name:     "Nil map",
			input:    nil,
			expected: map[string]string{},
		},
		{
			name:     "Empty map",
			input:    map[string]*variables.TextTemplate{},
			expected: map[string]string{},
		},
		{
			name: "Map with nil value",
			input: map[string]*variables.TextTemplate{
				"###ZARF_GIT_AUTH_PULL###": nil,
			},
			expected: map[string]string{
				"###ZARF_GIT_AUTH_PULL###": "",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := getSanitizedTemplateMap(test.input)
			require.Equal(t, test.expected, output)
		})
	}
}

func TestGetZarfTemplates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		state       state.State
		expectedMap map[string]*variables.TextTemplate
	}{
		{
			name: "architecture",
			expectedMap: map[string]*variables.TextTemplate{
				"###ZARF_ARCHITECTURE###": {Sensitive: false, Value: "amd64"},
			},
			state: state.State{
				Architecture: "amd64",
			},
		},
		{
			name: "storage-class",
			expectedMap: map[string]*variables.TextTemplate{
				"###ZARF_STORAGE_CLASS###": {Sensitive: false, Value: "local-path"},
			},
			state: state.State{
				StorageClass: "local-path",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			output, err := GetZarfTemplates(ctx, "test-component", &test.state)
			require.NoError(t, err)
			for key, value := range test.expectedMap {
				mapValue, exists := output[key]
				require.True(t, exists)
				require.Equal(t, value, mapValue)
			}
		})
	}
}
