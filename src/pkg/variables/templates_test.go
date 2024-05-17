// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024-Present Defense Unicorns

package variables

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

var start = `
This is a test file for templating.

  ###PREFIX_VAR_REPLACE_ME###
  ###PREFIX_CONST_REPLACE_ME###
  ###PREFIX_APP_REPLACE_ME###
  ###PREFIX_NON_EXIST###
`
var simple = `
This is a test file for templating.

  VAR_REPLACED
  CONST_REPLACED
  APP_REPLACED
  ###PREFIX_NON_EXIST###
`
var multiline = `
This is a test file for templating.

  VAR_REPLACED
VAR_SECOND
  CONST_REPLACED
CONST_SECOND
  APP_REPLACED
APP_SECOND
  ###PREFIX_NON_EXIST###
`
var autoIndent = `
This is a test file for templating.

  VAR_REPLACED
  VAR_SECOND
  CONST_REPLACED
  CONST_SECOND
  APP_REPLACED
  APP_SECOND
  ###PREFIX_NON_EXIST###
`
var file = `
This is a test file for templating.

  The contents of this file become the template value
  CONSTs Don't Support File
  The contents of this file become the template value
  ###PREFIX_NON_EXIST###
`

func TestReplaceTextTemplate(t *testing.T) {
	type test struct {
		vc           VariableConfig
		path         string
		wantErr      error
		wantContents string
	}

	tests := []test{
		{
			vc:           VariableConfig{setVariableMap: SetVariableMap{}, applicationTemplates: map[string]*TextTemplate{}},
			path:         "non-existent.test",
			wantErr:      errors.New("open non-existent.test: no such file or directory"),
			wantContents: start,
		},
		{
			vc: VariableConfig{
				templatePrefix: "PREFIX",
				setVariableMap: SetVariableMap{
					"REPLACE_ME": {Value: "VAR_REPLACED"},
				},
				constants: []Constant{{Name: "REPLACE_ME", Value: "CONST_REPLACED"}},
				applicationTemplates: map[string]*TextTemplate{
					"###PREFIX_APP_REPLACE_ME###": {Value: "APP_REPLACED"},
				},
			},
			wantErr:      nil,
			wantContents: simple,
		},
		{
			vc: VariableConfig{
				templatePrefix: "PREFIX",
				setVariableMap: SetVariableMap{
					"REPLACE_ME": {Value: "VAR_REPLACED\nVAR_SECOND"},
				},
				constants: []Constant{{Name: "REPLACE_ME", Value: "CONST_REPLACED\nCONST_SECOND"}},
				applicationTemplates: map[string]*TextTemplate{
					"###PREFIX_APP_REPLACE_ME###": {Value: "APP_REPLACED\nAPP_SECOND"},
				},
			},
			wantErr:      nil,
			wantContents: multiline,
		},
		{
			vc: VariableConfig{
				templatePrefix: "PREFIX",
				setVariableMap: SetVariableMap{
					"REPLACE_ME": {Value: "VAR_REPLACED\nVAR_SECOND", Variable: Variable{AutoIndent: true}},
				},
				constants: []Constant{{Name: "REPLACE_ME", Value: "CONST_REPLACED\nCONST_SECOND", AutoIndent: true}},
				applicationTemplates: map[string]*TextTemplate{
					"###PREFIX_APP_REPLACE_ME###": {Value: "APP_REPLACED\nAPP_SECOND", AutoIndent: true},
				},
			},
			wantErr:      nil,
			wantContents: autoIndent,
		},
		{
			vc: VariableConfig{
				templatePrefix: "PREFIX",
				setVariableMap: SetVariableMap{
					"REPLACE_ME": {Value: "testdata/file.txt", Variable: Variable{Type: FileVariableType}},
				},
				constants: []Constant{{Name: "REPLACE_ME", Value: "CONSTs Don't Support File"}},
				applicationTemplates: map[string]*TextTemplate{
					"###PREFIX_APP_REPLACE_ME###": {Value: "testdata/file.txt", Type: FileVariableType},
				},
			},
			wantErr:      nil,
			wantContents: file,
		},
	}

	for _, tc := range tests {
		tmpDir := t.TempDir()
		tc.path = filepath.Join(tmpDir, "templates.test")

		f, _ := os.Create(tc.path)
		defer f.Close()

		f.WriteString(start)

		gotErr := tc.vc.ReplaceTextTemplate(tc.path)
		if gotErr != nil && tc.wantErr != nil {
			if gotErr.Error() != tc.wantErr.Error() {
				t.Fatalf("wanted err: %s, got err: %s", tc.wantErr, gotErr)
			}
		} else if gotErr != nil {
			t.Fatalf("got unexpected err: %s", gotErr)
		} else {
			gotContents, _ := os.ReadFile(tc.path)
			if string(gotContents) != tc.wantContents {
				t.Fatalf("wanted contents: %s, got contents: %s", tc.wantContents, string(gotContents))
			}
		}

	}
}
