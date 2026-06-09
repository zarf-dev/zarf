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

func componentWithChartValue(name, sourcePath string) v1alpha1.ZarfComponent {
	return v1alpha1.ZarfComponent{
		Name: name,
		Charts: []v1alpha1.ZarfChart{
			{Name: "chart", Values: []v1alpha1.ZarfChartValue{{SourcePath: sourcePath, TargetPath: ".dst"}}},
		},
	}
}

func componentWithWait(name string, wait *v1alpha1.ZarfComponentActionWait) v1alpha1.ZarfComponent {
	return v1alpha1.ZarfComponent{
		Name: name,
		Actions: v1alpha1.ZarfComponentActions{
			OnDeploy: v1alpha1.ZarfComponentActionSet{
				After: []v1alpha1.ZarfComponentAction{
					{Wait: wait, Template: tmplPtr()},
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
			name: "value defined by a prior component's setValues passes",
			components: []v1alpha1.ZarfComponent{
				{
					Name: "a",
					Actions: v1alpha1.ZarfComponentActions{
						OnDeploy: v1alpha1.ZarfComponentActionSet{
							Before: []v1alpha1.ZarfComponentAction{
								{Cmd: "get-db", SetValues: []v1alpha1.SetValue{{Key: ".db", Type: v1alpha1.SetValueYAML}}},
							},
						},
					},
				},
				componentWithCmd("b", "echo {{ .Values.db.host }}"),
			},
		},
		{
			name: "value set by a later component fails",
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
			wantErr: ".Values.db.host",
		},
		{
			name: "value set by an earlier action in the same component passes",
			components: []v1alpha1.ZarfComponent{{
				Name: "a",
				Actions: v1alpha1.ZarfComponentActions{
					OnDeploy: v1alpha1.ZarfComponentActionSet{
						Before: []v1alpha1.ZarfComponentAction{
							{Cmd: "get-db", SetValues: []v1alpha1.SetValue{{Key: ".db", Type: v1alpha1.SetValueYAML}}},
						},
						After: []v1alpha1.ZarfComponentAction{
							{Cmd: "echo {{ .Values.db.host }}", Template: tmplPtr()},
						},
					},
				},
			}},
		},
		{
			name: "value defined by a prior component's root setValues passes",
			components: []v1alpha1.ZarfComponent{
				{
					Name: "a",
					Actions: v1alpha1.ZarfComponentActions{
						OnDeploy: v1alpha1.ZarfComponentActionSet{
							Before: []v1alpha1.ZarfComponentAction{
								{Cmd: "get", SetValues: []v1alpha1.SetValue{{Key: ".", Type: v1alpha1.SetValueYAML}}},
							},
						},
					},
				},
				componentWithCmd("b", "echo {{ .Values.anything }}"),
			},
		},
		{
			name: "root setValues in a later component fails",
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
			wantErr: ".Values.anything",
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
		{
			name:       "chart value source present in map passes",
			components: []v1alpha1.ZarfComponent{componentWithChartValue("a", ".registry.port")},
			vals:       value.Values{"registry": map[string]any{"port": 5000}},
		},
		{
			name:       "chart value source undefined fails",
			components: []v1alpha1.ZarfComponent{componentWithChartValue("a", ".registry.port")},
			wantErr:    "maps undefined value .registry.port",
		},
		{
			name: "chart value source defined by a prior component's setValues passes",
			components: []v1alpha1.ZarfComponent{
				{
					Name: "a",
					Actions: v1alpha1.ZarfComponentActions{
						OnDeploy: v1alpha1.ZarfComponentActionSet{
							Before: []v1alpha1.ZarfComponentAction{
								{Cmd: "get", SetValues: []v1alpha1.SetValue{{Key: ".registry", Type: v1alpha1.SetValueYAML}}},
							},
						},
					},
				},
				componentWithChartValue("b", ".registry.port"),
			},
		},
		{
			name: "chart value source set by a later component fails",
			components: []v1alpha1.ZarfComponent{
				componentWithChartValue("a", ".registry.port"),
				{
					Name: "b",
					Actions: v1alpha1.ZarfComponentActions{
						OnDeploy: v1alpha1.ZarfComponentActionSet{
							Before: []v1alpha1.ZarfComponentAction{
								{Cmd: "get", SetValues: []v1alpha1.SetValue{{Key: ".registry", Type: v1alpha1.SetValueYAML}}},
							},
						},
					},
				},
			},
			wantErr: "maps undefined value .registry.port",
		},
		{
			name: "wait cluster condition references undefined value fails",
			components: []v1alpha1.ZarfComponent{componentWithWait("a", &v1alpha1.ZarfComponentActionWait{
				Cluster: &v1alpha1.ZarfComponentActionWaitCluster{
					Kind: "Pod", Name: "x", Condition: "{{ .Values.missing }}",
				},
			})},
			wantErr: ".Values.missing",
		},
		{
			name: "wait network address references undefined value fails",
			components: []v1alpha1.ZarfComponent{componentWithWait("a", &v1alpha1.ZarfComponentActionWait{
				Network: &v1alpha1.ZarfComponentActionWaitNetwork{
					Protocol: "http", Address: "{{ .Values.host }}:8080",
				},
			})},
			wantErr: ".Values.host",
		},
		{
			name: "scalar setValues key satisfies a deeper reference",
			components: []v1alpha1.ZarfComponent{
				{
					Name: "a",
					Actions: v1alpha1.ZarfComponentActions{
						OnDeploy: v1alpha1.ZarfComponentActionSet{
							Before: []v1alpha1.ZarfComponentAction{
								{Cmd: "get-db", SetValues: []v1alpha1.SetValue{{Key: ".db", Type: v1alpha1.SetValueString}}},
							},
						},
					},
				},
				componentWithCmd("b", "echo {{ .Values.db.host }}"),
			},
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

// TestValidateTemplateRefsAccumulatesErrors verifies that independent undefined references across
// components are all reported in one pass via errors.Join rather than short-circuiting on the first.
func TestValidateTemplateRefsAccumulatesErrors(t *testing.T) {
	components := []v1alpha1.ZarfComponent{
		componentWithCmd("a", "echo {{ .Values.alpha }}"),
		componentWithCmd("b", "echo {{ .Values.beta }}"),
	}
	pkgLayout := &layout.PackageLayout{Pkg: v1alpha1.ZarfPackage{Components: components}}
	err := validateTemplateRefs(t.Context(), pkgLayout, nil)
	require.Error(t, err)
	require.ErrorContains(t, err, ".Values.alpha")
	require.ErrorContains(t, err, ".Values.beta")
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
	// Only `.Values.present` is supplied at deploy time, so any other reference is undefined. An empty
	// wantErrs means the package must validate clean (including the known under-catches below).
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
		{
			name:     "undefined value inside a directory-target file fails",
			dir:      "dir-file",
			wantErrs: []string{`file "confdir"`, ".Values.absentInDir"},
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
