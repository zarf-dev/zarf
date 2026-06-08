// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

func tmplPtr() *bool {
	b := true
	return &b
}

func componentWithCmd(name, cmd string) v1alpha1.ZarfComponent {
	return v1alpha1.ZarfComponent{
		Name: name,
		Actions: v1alpha1.ZarfComponentActions{
			OnDeploy: v1alpha1.ZarfComponentActionSet{
				After: []v1alpha1.ZarfComponentAction{
					{Cmd: cmd, Template: tmplPtr()},
				},
			},
		},
	}
}

func TestValidateTemplateRefs(t *testing.T) {
	tests := []struct {
		name       string
		components []v1alpha1.ZarfComponent
		vals       value.Values
		variables  []v1alpha1.InteractiveVariable
		wantErr    string
	}{
		{
			name:       "undefined value fails",
			components: []v1alpha1.ZarfComponent{componentWithCmd("a", "echo {{ .Values.missing }}")},
			wantErr:    ".Values.missing",
		},
		{
			name:       "value present in map passes",
			components: []v1alpha1.ZarfComponent{componentWithCmd("a", "echo {{ .Values.app.name }}")},
			vals:       value.Values{"app": map[string]any{"name": "x"}},
		},
		{
			name: "value defined by setValues elsewhere passes",
			components: []v1alpha1.ZarfComponent{
				componentWithCmd("a", "echo {{ .Values.db.host }}"),
				{
					Name: "b",
					Actions: v1alpha1.ZarfComponentActions{
						OnDeploy: v1alpha1.ZarfComponentActionSet{
							Before: []v1alpha1.ZarfComponentAction{
								{Cmd: "get-db", SetValues: []v1alpha1.SetValue{{Key: ".db", Type: v1alpha1.SetValueYAML}}},
							},
						},
					},
				},
			},
		},
		{
			name: "value defined by root setValues passes",
			components: []v1alpha1.ZarfComponent{
				componentWithCmd("a", "echo {{ .Values.anything }}"),
				{
					Name: "b",
					Actions: v1alpha1.ZarfComponentActions{
						OnDeploy: v1alpha1.ZarfComponentActionSet{
							Before: []v1alpha1.ZarfComponentAction{
								{Cmd: "get", SetValues: []v1alpha1.SetValue{{Key: ".", Type: v1alpha1.SetValueYAML}}},
							},
						},
					},
				},
			},
		},
		{
			name:       "partial value path fails",
			components: []v1alpha1.ZarfComponent{componentWithCmd("a", "echo {{ .Values.app.name }}")},
			vals:       value.Values{"app": map[string]any{"other": "x"}},
			wantErr:    ".Values.app.name",
		},
		{
			name:       "undefined variable fails",
			components: []v1alpha1.ZarfComponent{componentWithCmd("a", "echo {{ .Variables.FOO }}")},
			wantErr:    ".Variables.FOO",
		},
		{
			name:       "package variable passes",
			components: []v1alpha1.ZarfComponent{componentWithCmd("a", "echo {{ .Variables.FOO }}")},
			variables:  []v1alpha1.InteractiveVariable{{Variable: v1alpha1.Variable{Name: "FOO"}}},
		},
		{
			name: "variable defined by setVariables passes",
			components: []v1alpha1.ZarfComponent{
				componentWithCmd("a", "echo {{ .Variables.BAR }}"),
				{
					Name: "b",
					Actions: v1alpha1.ZarfComponentActions{
						OnDeploy: v1alpha1.ZarfComponentActionSet{
							Before: []v1alpha1.ZarfComponentAction{
								{Cmd: "get", SetVariables: []v1alpha1.Variable{{Name: "BAR"}}},
							},
						},
					},
				},
			},
		},
		{
			name: "untemplated action is skipped",
			components: []v1alpha1.ZarfComponent{{
				Name: "a",
				Actions: v1alpha1.ZarfComponentActions{
					OnDeploy: v1alpha1.ZarfComponentActionSet{
						After: []v1alpha1.ZarfComponentAction{{Cmd: "echo {{ .Values.missing }}"}},
					},
				},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vc := variables.New("ZARF", nil, nil)
			require.NoError(t, vc.PopulateVariables(tt.variables, nil))

			err := validateTemplateRefs(t.Context(), nil, tt.components, tt.vals, vc)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
