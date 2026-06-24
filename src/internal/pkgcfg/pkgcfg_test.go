// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package pkgcfg

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

// newer is a future apiVersion this binary does not understand.
const newer = "zarf.dev/v1beta999"

func TestParseDefinition(t *testing.T) {
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
			name: "unknown apiVersion errors without silent fallback",
			yaml: `
apiVersion: ` + newer + `
kind: ZarfPackageConfig
metadata:
  name: from-future
`,
			wantErr: `unsupported apiVersion "` + newer + `"`,
		},
		{
			name: "multi-document input errors",
			yaml: `
apiVersion: zarf.dev/v1alpha1
kind: ZarfPackageConfig
metadata:
  name: first
---
apiVersion: zarf.dev/v1alpha1
kind: ZarfPackageConfig
metadata:
  name: second
`,
			wantErr: "single YAML document",
		},
		{
			name: "leading document separator is accepted",
			yaml: `---
apiVersion: zarf.dev/v1alpha1
kind: ZarfPackageConfig
metadata:
  name: leading-sep
`,
			wantName: "leading-sep",
		},
		{
			name: "leading and trailing separators are accepted",
			yaml: `---
apiVersion: zarf.dev/v1alpha1
kind: ZarfPackageConfig
metadata:
  name: both-sep
---
`,
			wantName: "both-sep",
		},
		{
			name:    "empty input errors",
			yaml:    "",
			wantErr: "no package definition found",
		},
		{
			name:    "whitespace-only input errors",
			yaml:    "\n  \n",
			wantErr: "no package definition found",
		},
		{
			name:    "malformed yaml bubbles up from the parser",
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

func TestParseBuiltPackageDefinition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		yaml     string
		wantName string
		wantErr  string
	}{
		{
			name: "single v1alpha1 doc parses",
			yaml: `
apiVersion: zarf.dev/v1alpha1
kind: ZarfPackageConfig
metadata:
  name: single
`,
			wantName: "single",
		},
		{
			name: "picks v1alpha1 when newer doc is unrecognized",
			yaml: `
apiVersion: zarf.dev/v1alpha1
kind: ZarfPackageConfig
metadata:
  name: from-v1alpha1
---
apiVersion: ` + newer + `
kind: ZarfPackageConfig
metadata:
  name: from-future
`,
			wantName: "from-v1alpha1",
		},
		{
			name: "tolerates reverse order",
			yaml: `
apiVersion: ` + newer + `
kind: ZarfPackageConfig
metadata:
  name: from-future
---
apiVersion: zarf.dev/v1alpha1
kind: ZarfPackageConfig
metadata:
  name: from-v1alpha1
`,
			wantName: "from-v1alpha1",
		},
		{
			name: "errors when no known version present",
			yaml: `
apiVersion: ` + newer + `
kind: ZarfPackageConfig
metadata:
  name: from-future
`,
			wantErr: "no supported apiVersion found",
		},
		{
			name: "errors on duplicate same-version docs",
			yaml: `
apiVersion: zarf.dev/v1alpha1
kind: ZarfPackageConfig
metadata:
  name: first
---
apiVersion: zarf.dev/v1alpha1
kind: ZarfPackageConfig
metadata:
  name: second
`,
			wantErr: `duplicate apiVersion "zarf.dev/v1alpha1"`,
		},
		{
			name: "trailing document separator is ignored",
			yaml: `
apiVersion: zarf.dev/v1alpha1
kind: ZarfPackageConfig
metadata:
  name: trailing
---
`,
			wantName: "trailing",
		},
		{
			name:    "empty input errors",
			yaml:    "",
			wantErr: "no package definition found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pkg, err := ParseMultiDoc(context.Background(), []byte(tt.yaml))
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

// TestParseDefinitionAndParseBuiltPackageAgreeOnSingleDoc confirms that a
// single-doc v1alpha1 yaml decodes identically through both entry points.
func TestParseDefinitionAndParseBuiltPackageAgreeOnSingleDoc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	body := []byte("apiVersion: " + v1alpha1.APIVersion + "\nkind: ZarfPackageConfig\nmetadata:\n  name: agree\ncomponents:\n  - name: c\n")

	fromDef, err := Parse(ctx, body)
	require.NoError(t, err)
	fromPkg, err := ParseMultiDoc(ctx, body)
	require.NoError(t, err)
	require.Equal(t, fromDef, fromPkg)
}

func TestAPIVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		want    string
		wantErr string
	}{
		{
			name: "explicit v1alpha1",
			yaml: "apiVersion: zarf.dev/v1alpha1\nkind: ZarfPackageConfig\nmetadata:\n  name: a\n",
			want: "zarf.dev/v1alpha1",
		},
		{
			name: "explicit v1beta1",
			yaml: "apiVersion: zarf.dev/v1beta1\nkind: ZarfPackageConfig\nmetadata:\n  name: b\n",
			want: "zarf.dev/v1beta1",
		},
		{
			name: "omitted apiVersion returns empty string",
			yaml: "kind: ZarfPackageConfig\nmetadata:\n  name: c\n",
			want: "",
		},
		{
			name:    "malformed apiVersion errors",
			yaml:    "apiVersion: [not, a, string]\n",
			wantErr: "apiVersion",
		},
		{
			name:    "empty input errors",
			yaml:    "",
			wantErr: "no package definition found",
		},
		{
			name:    "multi-document input errors",
			yaml:    "apiVersion: zarf.dev/v1alpha1\nkind: ZarfPackageConfig\nmetadata:\n  name: a\n---\napiVersion: zarf.dev/v1beta1\nkind: ZarfPackageConfig\nmetadata:\n  name: b\n",
			wantErr: "single YAML document",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := APIVersion([]byte(tt.yaml))
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseV1Beta1(t *testing.T) {
	t.Parallel()

	yaml := `
apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: beta-pkg
  description: a v1beta1 package
components:
  - name: first
    description: a component
`
	pkg, err := ParseV1Beta1(context.Background(), []byte(yaml))
	require.NoError(t, err)
	require.Equal(t, v1beta1.APIVersion, pkg.APIVersion)
	require.Equal(t, "beta-pkg", pkg.Metadata.Name)
	require.Equal(t, "a v1beta1 package", pkg.Metadata.Description)
	require.Len(t, pkg.Components, 1)
	require.Equal(t, "first", pkg.Components[0].Name)
}

func TestParseV1Beta1Errors(t *testing.T) {
	t.Parallel()

	_, err := ParseV1Beta1(context.Background(), []byte(""))
	require.ErrorContains(t, err, "no package definition found")

	multiDoc := "apiVersion: zarf.dev/v1beta1\nkind: ZarfPackageConfig\nmetadata:\n  name: a\n---\napiVersion: zarf.dev/v1beta1\nkind: ZarfPackageConfig\nmetadata:\n  name: b\n"
	_, err = ParseV1Beta1(context.Background(), []byte(multiDoc))
	require.ErrorContains(t, err, "single YAML document")
}

func TestHandlerFor(t *testing.T) {
	t.Parallel()

	// Empty apiVersion and explicit v1alpha1 must resolve to the same handler.
	emptyHandler, emptyOK := handlerFor("")
	require.True(t, emptyOK)
	v1Handler, v1OK := handlerFor(v1alpha1.APIVersion)
	require.True(t, v1OK)
	require.Equal(t, v1Handler.version, emptyHandler.version)
	require.Equal(t, v1Handler.priority, emptyHandler.priority)

	_, unknownOK := handlerFor("zarf.dev/v1beta999")
	require.False(t, unknownOK)

	// Duplicate priorities would make "latest" ambiguous.
	priorities := map[int]string{}
	for _, h := range knownAPIVersions {
		if existing, dup := priorities[h.priority]; dup {
			t.Fatalf("duplicate priority %d shared by %q and %q", h.priority, existing, h.version)
		}
		priorities[h.priority] = h.version
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
