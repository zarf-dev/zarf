// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package variables

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

func TestPopulateVariables(t *testing.T) {
	type test struct {
		vc       VariableConfig
		vars     []v1alpha1.InteractiveVariable
		presets  map[string]string
		wantErr  bool
		wantVars SetVariableMap
	}

	prompt := func(_ v1alpha1.InteractiveVariable) (value string, err error) { return "Prompt", nil }

	tests := []test{
		{
			vc:       VariableConfig{setVariableMap: SetVariableMap{}},
			vars:     []v1alpha1.InteractiveVariable{{Variable: v1alpha1.Variable{Name: "NAME"}}},
			presets:  map[string]string{},
			wantVars: SetVariableMap{"NAME": {Variable: v1alpha1.Variable{Name: "NAME"}}},
		},
		{
			vc: VariableConfig{setVariableMap: SetVariableMap{}},
			vars: []v1alpha1.InteractiveVariable{
				{Variable: v1alpha1.Variable{Name: "NAME"}, Default: "Default"},
			},
			presets: map[string]string{},
			wantVars: SetVariableMap{
				"NAME": {Variable: v1alpha1.Variable{Name: "NAME"}, Value: "Default"},
			},
		},
		{
			vc: VariableConfig{setVariableMap: SetVariableMap{}},
			vars: []v1alpha1.InteractiveVariable{
				{Variable: v1alpha1.Variable{Name: "NAME"}, Default: "Default"},
			},
			presets: map[string]string{"NAME": "Set"},
			wantVars: SetVariableMap{
				"NAME": {Variable: v1alpha1.Variable{Name: "NAME"}, Value: "Set"},
			},
		},
		{
			vc: VariableConfig{setVariableMap: SetVariableMap{}},
			vars: []v1alpha1.InteractiveVariable{
				{Variable: v1alpha1.Variable{Name: "NAME", Sensitive: true, AutoIndent: true, Type: v1alpha1.FileVariableType}},
			},
			presets: map[string]string{},
			wantVars: SetVariableMap{
				"NAME": {Variable: v1alpha1.Variable{Name: "NAME", Sensitive: true, AutoIndent: true, Type: v1alpha1.FileVariableType}},
			},
		},
		{
			vc: VariableConfig{setVariableMap: SetVariableMap{}},
			vars: []v1alpha1.InteractiveVariable{
				{Variable: v1alpha1.Variable{Name: "NAME", Sensitive: true, AutoIndent: true, Type: v1alpha1.FileVariableType}},
			},
			presets: map[string]string{"NAME": "Set"},
			wantVars: SetVariableMap{
				"NAME": {Variable: v1alpha1.Variable{Name: "NAME", Sensitive: true, AutoIndent: true, Type: v1alpha1.FileVariableType}, Value: "Set"},
			},
		},
		{
			vc: VariableConfig{setVariableMap: SetVariableMap{}, prompt: prompt},
			vars: []v1alpha1.InteractiveVariable{
				{Variable: v1alpha1.Variable{Name: "NAME"}, Prompt: true},
			},
			presets: map[string]string{},
			wantVars: SetVariableMap{
				"NAME": {Variable: v1alpha1.Variable{Name: "NAME"}, Value: "Prompt"},
			},
		},
		{
			vc: VariableConfig{setVariableMap: SetVariableMap{}, prompt: prompt},
			vars: []v1alpha1.InteractiveVariable{
				{Variable: v1alpha1.Variable{Name: "NAME"}, Default: "Default", Prompt: true},
			},
			presets: map[string]string{},
			wantVars: SetVariableMap{
				"NAME": {Variable: v1alpha1.Variable{Name: "NAME"}, Value: "Prompt"},
			},
		},
		{
			vc: VariableConfig{setVariableMap: SetVariableMap{}, prompt: prompt},
			vars: []v1alpha1.InteractiveVariable{
				{Variable: v1alpha1.Variable{Name: "NAME"}, Prompt: true},
			},
			presets: map[string]string{"NAME": "Set"},
			wantVars: SetVariableMap{
				"NAME": {Variable: v1alpha1.Variable{Name: "NAME"}, Value: "Set"},
			},
		},
	}

	for _, tc := range tests {
		gotErr := tc.vc.PopulateVariables(tc.vars, tc.presets)
		if tc.wantErr {
			require.Error(t, gotErr)
		} else {
			require.NoError(t, gotErr)
		}

		gotVars := tc.vc.setVariableMap

		require.Equal(t, len(gotVars), len(tc.wantVars))

		for key := range gotVars {
			require.Equal(t, gotVars[key], tc.wantVars[key])
		}
	}
}

func TestCheckVariablePattern(t *testing.T) {
	type test struct {
		vc         VariableConfig
		name       string
		pattern    string
		wantErrMsg string
	}

	tests := []test{
		{
			vc: VariableConfig{setVariableMap: SetVariableMap{}}, name: "NAME", pattern: "n[a-z]me",
			wantErrMsg: "variable \"NAME\" was not found in the current variable map",
		},
		{
			vc: VariableConfig{
				setVariableMap: SetVariableMap{"NAME": &v1alpha1.SetVariable{Value: "name"}},
			}, name: "NAME", pattern: "n[^a]me",
			wantErrMsg: "provided value for variable \"NAME\" does not match pattern \"n[^a]me\"",
		},
		{
			vc: VariableConfig{
				setVariableMap: SetVariableMap{"NAME": &v1alpha1.SetVariable{Value: "name"}},
			}, name: "NAME", pattern: "n[a-z]me", wantErrMsg: "",
		},
		{
			vc: VariableConfig{
				setVariableMap: SetVariableMap{"NAME": &v1alpha1.SetVariable{Value: "name"}},
			}, name: "NAME", pattern: "n[a-z-bad-pattern", wantErrMsg: "error parsing regexp: missing closing ]: `[a-z-bad-pattern`",
		},
	}

	for _, tc := range tests {
		got := tc.vc.CheckVariablePattern(tc.name, tc.pattern)
		if tc.wantErrMsg != "" {
			require.EqualError(t, got, tc.wantErrMsg)
		} else {
			require.NoError(t, got)
		}
	}
}
