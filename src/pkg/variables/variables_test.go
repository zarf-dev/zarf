// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package variables

import (
	"errors"
	"reflect"
	"testing"
)

func TestPopulateVariables(t *testing.T) {
	type test struct {
		vc       VariableConfig
		vars     []InteractiveVariable
		presets  map[string]string
		wantErr  error
		wantVars SetVariableMap
	}

	prompt := func(_ InteractiveVariable) (value string, err error) { return "Prompt", nil }

	tests := []test{
		{
			vc:       VariableConfig{setVariableMap: SetVariableMap{}},
			vars:     []InteractiveVariable{{Variable: Variable{Name: "NAME"}}},
			presets:  map[string]string{},
			wantErr:  nil,
			wantVars: SetVariableMap{"NAME": {Variable: Variable{Name: "NAME"}}},
		},
		{
			vc: VariableConfig{setVariableMap: SetVariableMap{}},
			vars: []InteractiveVariable{
				{Variable: Variable{Name: "NAME"}, Default: "Default"},
			},
			presets: map[string]string{},
			wantErr: nil,
			wantVars: SetVariableMap{
				"NAME": {Variable: Variable{Name: "NAME"}, Value: "Default"},
			},
		},
		{
			vc: VariableConfig{setVariableMap: SetVariableMap{}},
			vars: []InteractiveVariable{
				{Variable: Variable{Name: "NAME"}, Default: "Default"},
			},
			presets: map[string]string{"NAME": "Set"},
			wantErr: nil,
			wantVars: SetVariableMap{
				"NAME": {Variable: Variable{Name: "NAME"}, Value: "Set"},
			},
		},
		{
			vc: VariableConfig{setVariableMap: SetVariableMap{}},
			vars: []InteractiveVariable{
				{Variable: Variable{Name: "NAME", Sensitive: true, AutoIndent: true, Type: FileVariableType}},
			},
			presets: map[string]string{},
			wantErr: nil,
			wantVars: SetVariableMap{
				"NAME": {Variable: Variable{Name: "NAME", Sensitive: true, AutoIndent: true, Type: FileVariableType}},
			},
		},
		{
			vc: VariableConfig{setVariableMap: SetVariableMap{}},
			vars: []InteractiveVariable{
				{Variable: Variable{Name: "NAME", Sensitive: true, AutoIndent: true, Type: FileVariableType}},
			},
			presets: map[string]string{"NAME": "Set"},
			wantErr: nil,
			wantVars: SetVariableMap{
				"NAME": {Variable: Variable{Name: "NAME", Sensitive: true, AutoIndent: true, Type: FileVariableType}, Value: "Set"},
			},
		},
		{
			vc: VariableConfig{setVariableMap: SetVariableMap{}, prompt: prompt},
			vars: []InteractiveVariable{
				{Variable: Variable{Name: "NAME"}, Prompt: true},
			},
			presets: map[string]string{},
			wantErr: nil,
			wantVars: SetVariableMap{
				"NAME": {Variable: Variable{Name: "NAME"}, Value: "Prompt"},
			},
		},
		{
			vc: VariableConfig{setVariableMap: SetVariableMap{}, prompt: prompt},
			vars: []InteractiveVariable{
				{Variable: Variable{Name: "NAME"}, Default: "Default", Prompt: true},
			},
			presets: map[string]string{},
			wantErr: nil,
			wantVars: SetVariableMap{
				"NAME": {Variable: Variable{Name: "NAME"}, Value: "Prompt"},
			},
		},
		{
			vc: VariableConfig{setVariableMap: SetVariableMap{}, prompt: prompt},
			vars: []InteractiveVariable{
				{Variable: Variable{Name: "NAME"}, Prompt: true},
			},
			presets: map[string]string{"NAME": "Set"},
			wantErr: nil,
			wantVars: SetVariableMap{
				"NAME": {Variable: Variable{Name: "NAME"}, Value: "Set"},
			},
		},
	}

	for _, tc := range tests {
		gotErr := tc.vc.PopulateVariables(tc.vars, tc.presets)
		if gotErr != nil && tc.wantErr != nil {
			if gotErr.Error() != tc.wantErr.Error() {
				t.Fatalf("wanted err: %s, got err: %s", tc.wantErr, gotErr)
			}
		} else if gotErr != nil {
			t.Fatalf("got unexpected err: %s", gotErr)
		}

		gotVars := tc.vc.setVariableMap

		if len(gotVars) != len(tc.wantVars) {
			t.Fatalf("wanted vars len: %d, got vars len: %d", len(tc.wantVars), len(gotVars))
		}

		for key := range gotVars {
			if !reflect.DeepEqual(gotVars[key], tc.wantVars[key]) {
				t.Fatalf("for key %s: wanted var: %v, got var: %v", key, tc.wantVars[key], gotVars[key])
			}
		}
	}
}

func TestCheckVariablePattern(t *testing.T) {
	type test struct {
		vc      VariableConfig
		name    string
		pattern string
		want    error
	}

	tests := []test{
		{
			vc: VariableConfig{setVariableMap: SetVariableMap{}}, name: "NAME", pattern: "n[a-z]me",
			want: errors.New("variable \"NAME\" was not found in the current variable map"),
		},
		{
			vc: VariableConfig{
				setVariableMap: SetVariableMap{"NAME": &SetVariable{Value: "name"}},
			}, name: "NAME", pattern: "n[^a]me",
			want: errors.New("provided value for variable \"NAME\" does not match pattern \"n[^a]me\""),
		},
		{
			vc: VariableConfig{
				setVariableMap: SetVariableMap{"NAME": &SetVariable{Value: "name"}},
			}, name: "NAME", pattern: "n[a-z]me", want: nil,
		},
	}

	for _, tc := range tests {
		got := tc.vc.CheckVariablePattern(tc.name, tc.pattern)
		if got != nil && tc.want != nil {
			if got.Error() != tc.want.Error() {
				t.Fatalf("wanted err: %s, got err: %s", tc.want, got)
			}
		} else if got != nil {
			t.Fatalf("got unexpected err: %s", got)
		}
	}
}
