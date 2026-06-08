// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/value"
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
			name:       "variable references are not validated",
			components: []v1alpha1.ZarfComponent{componentWithCmd("a", "echo {{ .Variables.FOO }}")},
		},
		{
			name:       "constant references are not validated",
			components: []v1alpha1.ZarfComponent{componentWithCmd("a", "echo {{ .Constants.BAR }}")},
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
			pkgLayout := &layout.PackageLayout{Pkg: v1alpha1.ZarfPackage{Components: tt.components}}
			err := validateTemplateRefs(t.Context(), pkgLayout, tt.vals)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
