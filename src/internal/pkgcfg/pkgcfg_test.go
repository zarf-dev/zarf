// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package pkgcfg

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		yaml     string
		wantName string
		wantErr  string
	}{
		{
			name: "omitted apiVersion parses as v1alpha1",
			yaml: `
kind: ZarfPackageConfig
metadata:
  name: no-api-version
`,
			wantName: "no-api-version",
		},
		{
			name: "explicit v1alpha1 apiVersion parses",
			yaml: `
apiVersion: zarf.dev/v1alpha1
kind: ZarfPackageConfig
metadata:
  name: explicit-v1alpha1
`,
			wantName: "explicit-v1alpha1",
		},
		{
			name: "unknown apiVersion returns error naming the version",
			yaml: `
apiVersion: zarf.dev/v1beta99
kind: ZarfPackageConfig
metadata:
  name: future
`,
			wantErr: `unknown apiVersion "zarf.dev/v1beta99"`,
		},
		{
			name:    "malformed yaml bubbles up from apiVersion probe",
			yaml:    "apiVersion: [not, a, string]\n",
			wantErr: "apiVersion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pkg, err := Parse(context.Background(), []byte(tt.yaml))
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.Equal(t, v1alpha1.ZarfPackage{}, pkg)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantName, pkg.Metadata.Name)
		})
	}
}

func TestDetectAPIVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		want    string
		wantErr bool
	}{
		{
			name: "returns value when present",
			yaml: "apiVersion: zarf.dev/v1alpha1\nkind: ZarfPackageConfig\n",
			want: "zarf.dev/v1alpha1",
		},
		{
			name: "returns empty string when absent",
			yaml: "kind: ZarfPackageConfig\nmetadata:\n  name: x\n",
			want: "",
		},
		{
			name: "ignores unrelated fields",
			yaml: "kind: ZarfPackageConfig\nmetadata:\n  name: x\napiVersion: future/v2\n",
			want: "future/v2",
		},
		{
			name:    "errors on malformed yaml",
			yaml:    "apiVersion: [bad",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := detectAPIVersion([]byte(tt.yaml))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestMigrateDeprecated(t *testing.T) {
	t.Parallel()

	pkg := v1alpha1.ZarfPackage{
		Components: []v1alpha1.ZarfComponent{
			{
				DeprecatedScripts: v1alpha1.DeprecatedZarfComponentScripts{
					Retry:   true,
					Prepare: []string{"p"},
					Before:  []string{"b"},
					After:   []string{"a"},
				},
				Actions: v1alpha1.ZarfComponentActions{
					OnCreate: v1alpha1.ZarfComponentActionSet{
						After: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "after",
							},
						},
						Before: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "before",
							},
						},
						OnSuccess: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-success",
							},
						},
						OnFailure: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-failure",
							},
						},
					},
					OnDeploy: v1alpha1.ZarfComponentActionSet{
						After: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "after",
							},
						},
						Before: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "before",
							},
						},
						OnSuccess: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-success",
							},
						},
						OnFailure: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-failure",
							},
						},
					},
					OnRemove: v1alpha1.ZarfComponentActionSet{
						After: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "after",
							},
						},
						Before: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "before",
							},
						},
						OnSuccess: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-success",
							},
						},
						OnFailure: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-failure",
							},
						},
					},
				},
			},
		},
	}
	migratedPkg, _ := migrateDeprecated(pkg)

	expectedPkg := v1alpha1.ZarfPackage{
		Build: v1alpha1.ZarfBuildData{
			Migrations: []string{
				ScriptsToActionsMigrated,
				PluralizeSetVariable,
			},
		},
		Components: []v1alpha1.ZarfComponent{
			{
				DeprecatedScripts: v1alpha1.DeprecatedZarfComponentScripts{
					Retry:   true,
					Prepare: []string{"p"},
					Before:  []string{"b"},
					After:   []string{"a"},
				},
				Actions: v1alpha1.ZarfComponentActions{
					OnCreate: v1alpha1.ZarfComponentActionSet{
						Defaults: v1alpha1.ZarfComponentActionDefaults{
							Mute:       true,
							MaxRetries: math.MaxInt,
						},
						Before: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "before",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "before",
									},
								},
							},
							{
								Cmd: "p",
							},
						},
						After: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "after",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "after",
									},
								},
							},
						},
						OnSuccess: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-success",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "on-success",
									},
								},
							},
						},
						OnFailure: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-failure",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "on-failure",
									},
								},
							},
						},
					},
					OnDeploy: v1alpha1.ZarfComponentActionSet{
						Defaults: v1alpha1.ZarfComponentActionDefaults{
							Mute:       true,
							MaxRetries: math.MaxInt,
						},
						Before: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "before",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "before",
									},
								},
							},
							{
								Cmd: "b",
							},
						},
						After: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "after",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "after",
									},
								},
							},
							{
								Cmd: "a",
							},
						},
						OnSuccess: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-success",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "on-success",
									},
								},
							},
						},
						OnFailure: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-failure",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "on-failure",
									},
								},
							},
						},
					},
					OnRemove: v1alpha1.ZarfComponentActionSet{
						Before: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "before",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "before",
									},
								},
							},
						},
						After: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "after",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "after",
									},
								},
							},
						},
						OnSuccess: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-success",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "on-success",
									},
								},
							},
						},
						OnFailure: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-failure",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "on-failure",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	require.Equal(t, expectedPkg, migratedPkg)
}
