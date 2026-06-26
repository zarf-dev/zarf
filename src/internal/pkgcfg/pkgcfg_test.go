// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package pkgcfg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/api/types"
	internalv1alpha1 "github.com/zarf-dev/zarf/src/internal/api/v1alpha1"
)

// newer is a future apiVersion this binary does not understand.
const newer = "zarf.dev/v1beta999"

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
				require.Equal(t, types.Package{}, pkg)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantName, pkg.Metadata.Name)
		})
	}
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
		{
			name: "multi-doc prefers higher-priority v1beta1 over v1alpha1",
			yaml: "apiVersion: zarf.dev/v1alpha1\nkind: ZarfPackageConfig\nmetadata:\n  name: a\n---\napiVersion: zarf.dev/v1beta1\nkind: ZarfPackageConfig\nmetadata:\n  name: b\n",
			want: "zarf.dev/v1beta1",
		},
		{
			name: "multi-doc prefers v1beta1 regardless of document order",
			yaml: "apiVersion: zarf.dev/v1beta1\nkind: ZarfPackageConfig\nmetadata:\n  name: b\n---\napiVersion: zarf.dev/v1alpha1\nkind: ZarfPackageConfig\nmetadata:\n  name: a\n",
			want: "zarf.dev/v1beta1",
		},
		{
			name: "multi-doc with only v1beta1 and an unknown version picks v1beta1",
			yaml: "apiVersion: zarf.dev/v1beta1\nkind: ZarfPackageConfig\nmetadata:\n  name: b\n---\napiVersion: " + newer + "\nkind: ZarfPackageConfig\nmetadata:\n  name: x\n",
			want: "zarf.dev/v1beta1",
		},
		{
			name:    "only unknown versions errors",
			yaml:    "apiVersion: " + newer + "\nkind: ZarfPackageConfig\nmetadata:\n  name: x\n",
			wantErr: "no supported apiVersion found",
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

func TestParseAsV1Alpha1(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// A document with no apiVersion is treated as v1alpha1, matching handlerFor.
	omitted := "kind: ZarfPackageConfig\nmetadata:\n  name: no-api-version\ncomponents:\n  - name: c\n"
	pkg, err := ParseAs[v1alpha1.ZarfPackage](ctx, []byte(omitted), v1alpha1.APIVersion)
	require.NoError(t, err)
	require.Equal(t, "no-api-version", pkg.Metadata.Name)

	// v1alpha1 deprecation migrations run as part of decoding: a deprecated script becomes an
	// action and the migration is recorded on the build data.
	withScripts := `
apiVersion: zarf.dev/v1alpha1
kind: ZarfPackageConfig
metadata:
  name: migrate-me
components:
  - name: c
    scripts:
      prepare:
        - "echo hello"
`
	pkg, err = ParseAs[v1alpha1.ZarfPackage](ctx, []byte(withScripts), v1alpha1.APIVersion)
	require.NoError(t, err)
	require.Contains(t, pkg.Build.Migrations, internalv1alpha1.ScriptsToActionsMigrated)
	require.Equal(t, "echo hello", pkg.Components[0].Actions.OnCreate.Before[0].Cmd)
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

	// An unknown requested apiVersion is rejected up front.
	_, err = ParseAs[v1beta1.Package](ctx, []byte(alphaOnly), newer)
	require.ErrorContains(t, err, `unsupported apiVersion "`+newer+`"`)
}

func TestParseDecodesV1Beta1ToGeneric(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	beta := "apiVersion: zarf.dev/v1beta1\nkind: ZarfPackageConfig\nmetadata:\n  name: beta\ncomponents:\n  - name: c\n"
	// ParseMultiDoc prefers the higher-priority v1beta1 document when both are present.
	mixed := beta + "---\napiVersion: zarf.dev/v1alpha1\nkind: ZarfPackageConfig\nmetadata:\n  name: alpha\ncomponents:\n  - name: c\n"
	pkg, err := ParseMultiDoc(ctx, []byte(mixed))
	require.NoError(t, err)
	require.Equal(t, v1beta1.APIVersion, pkg.APIVersion)
	require.Equal(t, "beta", pkg.Metadata.Name)

	// With only a v1beta1 document, ParseMultiDoc decodes it into the generic representation.
	pkg, err = ParseMultiDoc(ctx, []byte(beta))
	require.NoError(t, err)
	require.Equal(t, v1beta1.APIVersion, pkg.APIVersion)
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
