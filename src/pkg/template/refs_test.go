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
		{
			name:     "index of bare Values with string literals",
			template: `{{ index .Values "app" "name" }}`,
			want:     Refs{Values: [][]string{{"app", "name"}}},
		},
		{
			name:     "index of Values field with string literal",
			template: `{{ index .Values.db "host" }}`,
			want:     Refs{Values: [][]string{{"db", "host"}}},
		},
		{
			name:     "dynamic index key is ignored",
			template: "{{ index .Values .key }}",
			want:     Refs{},
		},
		{
			name:     "dot stored in a variable is ignored",
			template: "{{ $v := .Values }}{{ $v.db.host }}",
			want:     Refs{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReferencedKeys(tt.template)
			require.NoError(t, err)
			require.Equal(t, tt.want, got, "deployment returns only exact references")
		})
	}
}

func TestReferencedKeysInvalid(t *testing.T) {
	_, err := ReferencedKeys("{{ .Values.foo ")
	require.Error(t, err)
}

func TestTemplateValueConsumers(t *testing.T) {
	tests := []struct {
		name          string
		template      string
		want          [][]string
		wantExactRefs [][]string
	}{
		{
			name:     "no values usage",
			template: `text {{ .Variables.NAME }} {{ .Constants.NAME }} {{ "Values" }}`,
		},
		{
			name:          "direct path consumes subtree",
			template:      `{{ .Values.app.config | toYaml }}`,
			want:          [][]string{{"app", "config"}},
			wantExactRefs: [][]string{{"app", "config"}},
		},
		{
			name:     "bare values consumes root",
			template: `{{ .Values | toYaml }}`,
			want:     [][]string{{}},
		},
		{
			name:          "literal index consumes precise path",
			template:      `{{ index .Values "a" "b" }}`,
			want:          [][]string{{"a", "b"}},
			wantExactRefs: [][]string{{"a", "b"}},
		},
		{
			name:          "literal index of field",
			template:      `{{ index .Values.a "b" }}`,
			want:          [][]string{{"a", "b"}},
			wantExactRefs: [][]string{{"a", "b"}},
		},
		{
			name:          "dynamic index keeps source and key reads",
			template:      `{{ index .Values.profiles .Values.selectedProfile }}`,
			want:          [][]string{{"profiles"}, {"selectedProfile"}},
			wantExactRefs: [][]string{{"profiles"}, {"selectedProfile"}},
		},
		{
			name:          "dynamic index visits all key expressions",
			template:      `{{ index .Values.profiles (print .Values.selectedProfile) .Values.region }}`,
			want:          [][]string{{"profiles"}, {"selectedProfile"}, {"region"}},
			wantExactRefs: [][]string{{"profiles"}, {"selectedProfile"}, {"region"}},
		},
		{
			name:     "dynamic root index consumes root",
			template: `{{ index .Values .key }}`,
			want:     [][]string{{}},
		},
		{
			name:          "alias consumes source subtree",
			template:      `{{ $v := .Values.db }}{{ $v.host }}`,
			want:          [][]string{{"db"}},
			wantExactRefs: [][]string{{"db"}},
		},
		{
			name:          "indexed alias consumes source subtree",
			template:      `{{ $v := index .Values "db" }}{{ $v.host }}`,
			want:          [][]string{{"db"}},
			wantExactRefs: [][]string{{"db"}},
		},
		{
			name:     "whole values alias consumes root",
			template: `{{ $v := .Values }}{{ $v.db.host }}`,
			want:     [][]string{{}},
		},
		{
			name:          "with consumes rebound source subtree",
			template:      `{{ with .Values.db }}{{ .host }}{{ end }}`,
			want:          [][]string{{"db"}},
			wantExactRefs: [][]string{{"db"}},
		},
		{
			name:          "range consumes rebound source subtree",
			template:      `{{ range .Values.items }}{{ .name }}{{ end }}`,
			want:          [][]string{{"items"}},
			wantExactRefs: [][]string{{"items"}},
		},
		{
			name:          "if visits inactive branches",
			template:      `{{ if false }}{{ .Values.a }}{{ else }}{{ .Values.b }}{{ end }}`,
			want:          [][]string{{"a"}, {"b"}},
			wantExactRefs: [][]string{{"a"}, {"b"}},
		},
		{
			name:          "with and range visit else branches",
			template:      `{{ with .Values.db }}{{ .host }}{{ else }}{{ .Values.fallback }}{{ end }}{{ range .Values.items }}{{ .name }}{{ else }}{{ .Values.empty }}{{ end }}`,
			want:          [][]string{{"db"}, {"fallback"}, {"items"}, {"empty"}},
			wantExactRefs: [][]string{{"db"}, {"fallback"}, {"items"}, {"empty"}},
		},
		{
			name:     "root values reference",
			template: `{{ $.Values.app.name }}`,
			want:     [][]string{{"app", "name"}},
		},
		{
			name:     "root values consumes root",
			template: `{{ $.Values | toYaml }}`,
			want:     [][]string{{}},
		},
		{
			name:     "literal index of root values",
			template: `{{ index $.Values "a" "b" }}`,
			want:     [][]string{{"a", "b"}},
		},
		{
			name:     "dynamic index of root values",
			template: `{{ index $.Values.profiles $.Values.selectedProfile }}`,
			want:     [][]string{{"profiles"}, {"selectedProfile"}},
		},
		{
			name:          "root values inside rebound dot",
			template:      `{{ with .Values.db }}{{ $.Values.app.name }}{{ end }}`,
			want:          [][]string{{"db"}, {"app", "name"}},
			wantExactRefs: [][]string{{"db"}},
		},
		{
			name:          "parenthesized chain",
			template:      `{{ (.Values.db).host }}`,
			want:          [][]string{{"db", "host"}},
			wantExactRefs: [][]string{{"db"}},
		},
		{
			name:     "parenthesized root chain",
			template: `{{ ($.Values).db.host }}`,
			want:     [][]string{{"db", "host"}},
		},
		{
			name:     "parenthesized dollar root",
			template: `{{ ($).Values.db.host }}`,
			want:     [][]string{{"db", "host"}},
		},
		{
			name:     "parenthesized dot root",
			template: `{{ (.).Values.db.host }}`,
			want:     [][]string{{"db", "host"}},
		},
		{
			name:     "parenthesized other object is not values usage",
			template: `{{ (.Variables).NAME }}`,
		},
		{
			name:          "parenthesized indexed chain",
			template:      `{{ (index .Values "db").host }}`,
			want:          [][]string{{"db", "host"}},
			wantExactRefs: [][]string{{"db"}},
		},
		{
			name:          "parenthesized dynamic chain preserves reads",
			template:      `{{ (index .Values.profiles .Values.selectedProfile).host }}`,
			want:          [][]string{{"profiles"}, {"selectedProfile"}},
			wantExactRefs: [][]string{{"profiles"}, {"selectedProfile"}},
		},
		{
			name:          "named templates and invocation arguments",
			template:      `{{ define "db" }}{{ $.Values.db }}{{ end }}{{ template "db" .Values.context }}`,
			want:          [][]string{{"db"}, {"context"}},
			wantExactRefs: [][]string{{"context"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := Parse(tt.template)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.want, parsed.ValueConsumers())
			got, err := ReferencedKeys(tt.template)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.wantExactRefs, got.Values, "deployment references must remain unchanged")
		})
	}
}

func TestParseInvalid(t *testing.T) {
	for _, s := range []string{
		`{{ .Values.foo `,
		`{{ if .Values.enabled }}{{ .Values.foo }}`,
		`{{ unknownFunction .Values.foo }}`,
		`{{ __zarf_template_lookup_guard . "Values" "spoof" 1 1 }}`,
	} {
		t.Run(s, func(t *testing.T) {
			parsed, err := Parse(s)
			require.Error(t, err)
			require.Nil(t, parsed)
			var diagnostic *Error
			require.ErrorAs(t, err, &diagnostic)
			require.Equal(t, Syntax, diagnostic.Kind)
		})
	}
}
