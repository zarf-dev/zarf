// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package template provides functions for applying go-templates within Zarf.
package template

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReferencedKeys(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     Refs
	}{
		{
			name:     "no templates",
			template: "echo hello",
			want:     Refs{},
		},
		{
			name:     "simple values reference",
			template: "{{ .Values.app.name }}",
			want:     Refs{Values: [][]string{{"app", "name"}}},
		},
		{
			name:     "single segment value",
			template: "{{ .Values.foo }}",
			want:     Refs{Values: [][]string{{"foo"}}},
		},
		{
			name:     "variables and constants are ignored",
			template: "{{ .Variables.FOO }}-{{ .Constants.BAR }}",
			want:     Refs{},
		},
		{
			name:     "range only records the ranged path",
			template: "{{ range .Values.items }}{{ .name }}{{ end }}",
			want:     Refs{Values: [][]string{{"items"}}},
		},
		{
			name:     "with rebinds dot so inner field is ignored",
			template: "{{ with .Values.db }}{{ .host }}{{ end }}",
			want:     Refs{Values: [][]string{{"db"}}},
		},
		{
			name:     "pipeline into func",
			template: "{{ .Values.x | toYaml }}",
			want:     Refs{Values: [][]string{{"x"}}},
		},
		{
			name:     "if condition",
			template: "{{ if .Values.enabled }}on{{ end }}",
			want:     Refs{Values: [][]string{{"enabled"}}},
		},
		{
			name:     "bare .Values is not recorded",
			template: "{{ .Values | toYaml }}",
			want:     Refs{},
		},
		{
			name:     "multiple references ignore variables",
			template: "{{ .Values.a }}{{ .Values.b.c }}{{ .Variables.V }}",
			want:     Refs{Values: [][]string{{"a"}, {"b", "c"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReferencedKeys(tt.template)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.want.Values, got.Values)
		})
	}
}

func TestReferencedKeysInvalid(t *testing.T) {
	_, err := ReferencedKeys("{{ .Values.foo ")
	require.Error(t, err)
}
