// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package actions

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

func Test_actionCmdMutation(t *testing.T) {
	zarfCmd, err := utils.GetFinalExecutableCommand()
	require.NoError(t, err)
	tests := []struct {
		name      string
		cmd       string
		shellPref v1alpha1.Shell
		goos      string
		want      string
		wantErr   error
	}{
		{
			name:      "linux without zarf",
			cmd:       "echo \"this is zarf\"",
			shellPref: v1alpha1.Shell{},
			goos:      "linux",
			want:      "echo \"this is zarf\"",
			wantErr:   nil,
		},
		{
			name:      "linux including zarf",
			cmd:       "./zarf deploy",
			shellPref: v1alpha1.Shell{},
			goos:      "linux",
			want:      fmt.Sprintf("%s deploy", zarfCmd),
			wantErr:   nil,
		},
		{
			name:      "windows including zarf",
			cmd:       "./zarf deploy",
			shellPref: v1alpha1.Shell{},
			goos:      "windows",
			want:      fmt.Sprintf("%s deploy", zarfCmd),
			wantErr:   nil,
		},
		{
			name:      "windows env",
			cmd:       "echo ${ZARF_VAR_ENV1}",
			shellPref: v1alpha1.Shell{},
			goos:      "windows",
			want:      "echo $Env:ZARF_VAR_ENV1",
			wantErr:   nil,
		},
		{
			name: "windows env pwsh",
			cmd:  "echo ${ZARF_VAR_ENV1}",
			shellPref: v1alpha1.Shell{
				Windows: "pwsh",
			},
			goos:    "windows",
			want:    "echo $Env:ZARF_VAR_ENV1",
			wantErr: nil,
		},
		{
			name: "windows env powershell",
			cmd:  "echo ${ZARF_VAR_ENV1}",
			shellPref: v1alpha1.Shell{
				Windows: "powershell",
			},
			goos:    "windows",
			want:    "echo $Env:ZARF_VAR_ENV1",
			wantErr: nil,
		},
		{
			name:      "windows multiple env",
			cmd:       "echo ${ZARF_VAR_ENV1} ${ZARF_VAR_ENV2}",
			shellPref: v1alpha1.Shell{},
			goos:      "windows",
			want:      "echo $Env:ZARF_VAR_ENV1 $Env:ZARF_VAR_ENV2",
			wantErr:   nil,
		},
		{
			name:      "windows constants",
			cmd:       "echo ${ZARF_CONST_ENV1}",
			shellPref: v1alpha1.Shell{},
			goos:      "windows",
			want:      "echo $Env:ZARF_CONST_ENV1",
			wantErr:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := actionCmdMutation(context.Background(), tt.cmd, tt.shellPref, tt.goos)
			require.Equal(t, tt.wantErr, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_parseAndSetValue(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		setValue v1alpha1.SetValue
		expect   value.Values
	}{
		{
			name:   "string type sets value directly",
			output: "my-string-value",
			setValue: v1alpha1.SetValue{
				Key:  ".key1",
				Type: v1alpha1.SetValueString,
			},
			expect: value.Values{"key1": "my-string-value"},
		},
		{
			name:   "json type parses object",
			output: `{"myKey":"myValue"}`,
			setValue: v1alpha1.SetValue{
				Key:  ".json",
				Type: v1alpha1.SetValueJSON,
			},
			expect: value.Values{"json": map[string]any{"myKey": "myValue"}},
		},
		{
			name:   "json type parses nested object",
			output: `{"outer":{"inner":"value"}}`,
			setValue: v1alpha1.SetValue{
				Key:  ".nested",
				Type: v1alpha1.SetValueJSON,
			},
			expect: value.Values{"nested": map[string]any{"outer": map[string]any{"inner": "value"}}},
		},
		{
			name:   "json type parses array",
			output: `[1,2,3]`,
			setValue: v1alpha1.SetValue{
				Key:  ".array",
				Type: v1alpha1.SetValueJSON,
			},
			expect: value.Values{"array": []any{float64(1), float64(2), float64(3)}},
		},
		{
			name:   "yaml type parses simple object",
			output: "myKey: myValue",
			setValue: v1alpha1.SetValue{
				Key:  ".yaml",
				Type: v1alpha1.SetValueYAML,
			},
			expect: value.Values{"yaml": map[string]any{"myKey": "myValue"}},
		},
		{
			name: "yaml type parses nested object",
			output: `outer:
  inner: value`,
			setValue: v1alpha1.SetValue{
				Key:  ".nested",
				Type: v1alpha1.SetValueYAML,
			},
			expect: value.Values{"nested": map[string]any{"outer": map[string]any{"inner": "value"}}},
		},
		{
			name: "yaml type parses array",
			output: `- item1
- item2
- item3`,
			setValue: v1alpha1.SetValue{
				Key:  ".array",
				Type: v1alpha1.SetValueYAML,
			},
			expect: value.Values{"array": []any{"item1", "item2", "item3"}},
		},
		{
			name:   "sets value at nested path",
			output: "nested-value",
			setValue: v1alpha1.SetValue{
				Key:  ".app.config.value",
				Type: v1alpha1.SetValueString,
			},
			expect: value.Values{
				"app": map[string]any{
					"config": map[string]any{
						"value": "nested-value",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			vals := make(value.Values)

			err := parseAndSetValue(tt.output, tt.setValue, vals)
			require.NoError(t, err)
			require.Equal(t, tt.expect, vals)
		})
	}
}

func Test_templateString(t *testing.T) {
	t.Parallel()
	templates := map[string]*variables.TextTemplate{
		"###ZARF_VAR_NAME###":      {Value: "agent-hook"},
		"###ZARF_VAR_NAMESPACE###": {Value: "zarf"},
		"###ZARF_CONST_FOO###":     {Value: "bar"},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no variables",
			input:    "app=podinfo",
			expected: "app=podinfo",
		},
		{
			name:     "shell-style var syntax",
			input:    "app=${ZARF_VAR_NAME}",
			expected: "app=agent-hook",
		},
		{
			name:     "shell-style const syntax",
			input:    "${ZARF_CONST_FOO}",
			expected: "bar",
		},
		{
			name:     "multiple variables",
			input:    "${ZARF_VAR_NAME} in ${ZARF_VAR_NAMESPACE}",
			expected: "agent-hook in zarf",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no match",
			input:    "${ZARF_VAR_UNKNOWN}",
			expected: "${ZARF_VAR_UNKNOWN}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := templateString(tt.input, templates)
			require.Equal(t, tt.expected, got)
		})
	}
}

func Test_parseAndSetValue_Errors(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		setValue  v1alpha1.SetValue
		errSubstr string
	}{
		{
			name:   "json parse error",
			output: `{invalid json}`,
			setValue: v1alpha1.SetValue{
				Key:  ".json",
				Type: v1alpha1.SetValueJSON,
			},
			errSubstr: "failed to parse JSON",
		},
		{
			name:   "yaml parse error",
			output: "invalid: yaml: with: bad: indentation",
			setValue: v1alpha1.SetValue{
				Key:  ".yaml",
				Type: v1alpha1.SetValueYAML,
			},
			errSubstr: "failed to parse YAML",
		},
		{
			name:   "invalid path format",
			output: "value",
			setValue: v1alpha1.SetValue{
				Key:  "no-leading-dot",
				Type: v1alpha1.SetValueString,
			},
			errSubstr: "invalid path format",
		},
		{
			name:   "unknown setValue type",
			output: "value",
			setValue: v1alpha1.SetValue{
				Key:  ".key",
				Type: "unknown",
			},
			errSubstr: "unknown setValue type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			vals := make(value.Values)

			err := parseAndSetValue(tt.output, tt.setValue, vals)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errSubstr)
		})
	}
}
