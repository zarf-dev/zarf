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
			name: "omitted apiVersion resolves to v1alpha1",
			yaml: "kind: ZarfPackageConfig\nmetadata:\n  name: c\n",
			want: "zarf.dev/v1alpha1",
		},
		// FIXME: make these tests aware of the priority so they don't have to be updated
		{
			name: "multi-doc prefers higher-priority v1alpha1 over v1beta1",
			yaml: "apiVersion: zarf.dev/v1alpha1\nkind: ZarfPackageConfig\nmetadata:\n  name: a\n---\napiVersion: zarf.dev/v1beta1\nkind: ZarfPackageConfig\nmetadata:\n  name: b\n",
			want: "zarf.dev/v1alpha1",
		},
		{
			name: "multi-doc prefers v1alpha1 regardless of document order",
			yaml: "apiVersion: zarf.dev/v1beta1\nkind: ZarfPackageConfig\nmetadata:\n  name: b\n---\napiVersion: zarf.dev/v1alpha1\nkind: ZarfPackageConfig\nmetadata:\n  name: a\n",
			want: "zarf.dev/v1alpha1",
		},
		{
			name: "multi-doc with only v1beta1 and an unknown version picks v1beta1",
			yaml: "apiVersion: zarf.dev/v1beta1\nkind: ZarfPackageConfig\nmetadata:\n  name: b\n---\napiVersion: " + newer + "\nkind: ZarfPackageConfig\nmetadata:\n  name: x\n",
			want: "zarf.dev/v1beta1",
		},
		{
			name: "unknown version is returned unresolved for the caller to reject",
			yaml: "apiVersion: " + newer + "\nkind: ZarfPackageConfig\nmetadata:\n  name: x\n",
			want: newer,
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

func TestParseAs(t *testing.T) {
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
	pkg, err := ParseAs[v1beta1.Package](context.Background(), []byte(yaml), v1beta1.APIVersion)
	require.NoError(t, err)
	require.Equal(t, v1beta1.APIVersion, pkg.APIVersion)
	require.Equal(t, "beta-pkg", pkg.Metadata.Name)
	require.Equal(t, "a v1beta1 package", pkg.Metadata.Description)
	require.Len(t, pkg.Components, 1)
	require.Equal(t, "first", pkg.Components[0].Name)
}

func TestParseAsSelectsFromMultiDoc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// The requested apiVersion's document is returned regardless of where it sits among others.
	mixed := "apiVersion: zarf.dev/v1alpha1\nkind: ZarfPackageConfig\nmetadata:\n  name: alpha\n---\napiVersion: zarf.dev/v1beta1\nkind: ZarfPackageConfig\nmetadata:\n  name: beta\ncomponents:\n  - name: c\n"
	pkg, err := ParseAs[v1beta1.Package](ctx, []byte(mixed), v1beta1.APIVersion)
	require.NoError(t, err)
	require.Equal(t, v1beta1.APIVersion, pkg.APIVersion)
	require.Equal(t, "beta", pkg.Metadata.Name)

	// The same definition can be read as its v1alpha1 document by naming that apiVersion.
	alpha, err := ParseAs[v1alpha1.ZarfPackage](ctx, []byte(mixed), v1alpha1.APIVersion)
	require.NoError(t, err)
	require.Equal(t, "alpha", alpha.Metadata.Name)
}

func TestParseAsErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, err := ParseAs[v1beta1.Package](ctx, []byte(""), v1beta1.APIVersion)
	require.ErrorContains(t, err, "no package definition found")

	// A definition without a matching document errors rather than falling back.
	alphaOnly := "apiVersion: zarf.dev/v1alpha1\nkind: ZarfPackageConfig\nmetadata:\n  name: alpha\n"
	_, err = ParseAs[v1beta1.Package](ctx, []byte(alphaOnly), v1beta1.APIVersion)
	require.ErrorContains(t, err, `no "zarf.dev/v1beta1" document found`)
}

func TestParseDecodesV1Beta1DownToV1Alpha1(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	beta := "apiVersion: zarf.dev/v1beta1\nkind: ZarfPackageConfig\nmetadata:\n  name: beta\ncomponents:\n  - name: c\n"

	// Parse transparently converts a single v1beta1 document down to v1alpha1.
	pkg, err := Parse(ctx, []byte(beta))
	require.NoError(t, err)
	require.Equal(t, v1alpha1.APIVersion, pkg.APIVersion)
	require.Equal(t, "beta", pkg.Metadata.Name)

	// ParseMultiDoc prefers the higher-priority v1alpha1 document when both are present.
	mixed := beta + "---\napiVersion: zarf.dev/v1alpha1\nkind: ZarfPackageConfig\nmetadata:\n  name: alpha\ncomponents:\n  - name: c\n"
	pkg, err = ParseMultiDoc(ctx, []byte(mixed))
	require.NoError(t, err)
	require.Equal(t, "alpha", pkg.Metadata.Name)

	// With only a v1beta1 document, ParseMultiDoc decodes it via conversion.
	pkg, err = ParseMultiDoc(ctx, []byte(beta))
	require.NoError(t, err)
	require.Equal(t, v1alpha1.APIVersion, pkg.APIVersion)
	require.Equal(t, "beta", pkg.Metadata.Name)
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
