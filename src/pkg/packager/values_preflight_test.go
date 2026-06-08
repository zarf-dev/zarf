// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/test/testutil"
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

// assembleLayout assembles a real package layout from a testdata package directory so the
// manifest/file extraction path in validateTemplateRefs can be exercised end to end.
func assembleLayout(t *testing.T, srcDir string) *layout.PackageLayout {
	t.Helper()
	ctx := testutil.TestContext(t)
	defined, err := load.PackageDefinition(ctx, srcDir, load.DefinitionOptions{})
	require.NoError(t, err)
	pkgLayout, err := layout.AssemblePackage(ctx, defined.Pkg, srcDir, nil, layout.AssembleOptions{SkipSBOM: true})
	require.NoError(t, err)
	return pkgLayout
}

func TestValidateTemplateRefsManifestsAndFiles(t *testing.T) {
	// Each testdata package has a component with a templated manifest, a templated file, and a
	// setValues action declaring `.fromAction`. Only `.Values.present` is supplied at deploy time.
	tests := []struct {
		name     string
		dir      string
		wantErrs []string
	}{
		{
			name: "defined manifest and file values pass",
			dir:  "valid",
		},
		{
			name:     "undefined manifest value fails",
			dir:      "undefined-manifest",
			wantErrs: []string{`manifest "cm"`, ".Values.absentManifest"},
		},
		{
			name:     "undefined file value fails",
			dir:      "undefined-file",
			wantErrs: []string{`file "data.txt"`, ".Values.absentFile"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgLayout := assembleLayout(t, filepath.Join("testdata", "template-refs", tt.dir))
			err := validateTemplateRefs(testutil.TestContext(t), pkgLayout, value.Values{"present": "x"})
			if len(tt.wantErrs) == 0 {
				require.NoError(t, err)
				return
			}
			for _, want := range tt.wantErrs {
				require.ErrorContains(t, err, want)
			}
		})
	}
}
