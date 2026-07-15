// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/pkgcfg"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func mustPackagePath(t *testing.T, dir string) layout.PackagePath {
	t.Helper()
	pkgPath, err := layout.ResolvePackagePath(filepath.Join(dir, layout.ZarfYAML))
	require.NoError(t, err)
	return pkgPath
}

func loadV1Beta1Package(t *testing.T, dir string) v1beta1.Package {
	t.Helper()
	ctx := testutil.TestContext(t)
	b, err := os.ReadFile(filepath.Join(dir, layout.ZarfYAML))
	require.NoError(t, err)
	pkg, err := pkgcfg.ParseAs[v1beta1.Package](ctx, b, v1beta1.APIVersion)
	require.NoError(t, err)
	return pkg
}

func TestResolveImportsV1Beta1(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	t.Run("single local import rebases paths and collects values", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "single")
		pkg := loadV1Beta1Package(t, dir)

		resolved, schemas, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "")
		require.NoError(t, err)

		require.Len(t, resolved.Components, 1)
		comp := resolved.Components[0]
		require.Equal(t, "logging", comp.Name)
		require.Empty(t, comp.Import.Local)

		require.Len(t, comp.Charts, 1)
		require.NotNil(t, comp.Charts[0].Local)
		require.Equal(t, "components/loki-chart", comp.Charts[0].Local.Path)
		require.Equal(t, []v1beta1.ValuesFile{{Path: "components/loki-values.yaml"}}, comp.Charts[0].ValuesFiles)

		require.Len(t, comp.Files, 1)
		require.Equal(t, "components/motd.txt", comp.Files[0].Source)

		require.Equal(t, []v1beta1.Image{{Name: "grafana/loki:2.9.0"}}, comp.Images)

		require.Equal(t, []string{"components/logging-values.yaml"}, resolved.Values.Files)
		require.Equal(t, []string{"components/logging.schema.json"}, schemas)
	})

	t.Run("non-importing components are preserved alongside an importing one", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "mixed")
		pkg := loadV1Beta1Package(t, dir)

		resolved, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "")
		require.NoError(t, err)

		require.Len(t, resolved.Components, 3)
		require.Equal(t, "first", resolved.Components[0].Name)
		require.Equal(t, []v1beta1.Image{{Name: "alpine:3.20"}}, resolved.Components[0].Images)
		require.Equal(t, "middle", resolved.Components[1].Name)
		require.Equal(t, []v1beta1.Image{{Name: "nginx:1.27"}}, resolved.Components[1].Images)
		require.Empty(t, resolved.Components[1].Import.Local)
		require.Equal(t, "last", resolved.Components[2].Name)
		require.Equal(t, []v1beta1.Image{{Name: "busybox:1.36"}}, resolved.Components[2].Images)
	})

	t.Run("nested imports merge and rebase transitively", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "nested")
		pkg := loadV1Beta1Package(t, dir)

		resolved, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "")
		require.NoError(t, err)

		require.Len(t, resolved.Components, 1)
		comp := resolved.Components[0]
		require.Equal(t, "app", comp.Name)

		require.Len(t, comp.Charts, 1)
		require.NotNil(t, comp.Charts[0].Local)
		require.Equal(t, "components/app-chart", comp.Charts[0].Local.Path)

		require.Len(t, comp.Files, 1)
		require.Equal(t, "components/base/base.txt", comp.Files[0].Source)
	})

	t.Run("cyclic imports error", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "cycle")
		pkg := loadV1Beta1Package(t, dir)

		_, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "")
		require.ErrorContains(t, err, "cycle")
	})

	t.Run("variant selection picks the compatible flavor", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "variants")
		pkg := loadV1Beta1Package(t, dir)

		resolved, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "apache")
		require.NoError(t, err)

		require.Len(t, resolved.Components, 1)
		require.Equal(t, []v1beta1.Image{{Name: "httpd:2.4"}}, resolved.Components[0].Images)
	})

	t.Run("variant selection errors when no variant is compatible", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "variants")
		pkg := loadV1Beta1Package(t, dir)

		_, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "")
		require.ErrorContains(t, err, "no imported component")
	})

	t.Run("package component overrides imported component", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "merge")
		pkg := loadV1Beta1Package(t, dir)

		resolved, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "")
		require.NoError(t, err)

		comp := resolved.Components[0]
		require.Equal(t, []v1beta1.Image{
			{Name: "redis:7", Source: "daemon"},
			{Name: "nginx:1.27"},
		}, comp.Images)

		require.Len(t, comp.Charts, 1)
		require.Equal(t, "app", comp.Charts[0].Name)
		require.Equal(t, "app", comp.Charts[0].Namespace)
		require.Equal(t, "custom-release", comp.Charts[0].ReleaseName)
		require.NotNil(t, comp.Charts[0].Local)
		require.Equal(t, "components/app-chart", comp.Charts[0].Local.Path)
	})
}

func TestResolveImportsV1Beta1Errors(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	writePkg := func(t *testing.T, dir, body string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), []byte(body), 0o600))
	}

	t.Run("remote imports are not yet supported", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writePkg(t, dir, `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: remote
components:
  - name: remote
    import:
      remote:
        - url: oci://example.com/component:1.0.0
`)
		pkg := loadV1Beta1Package(t, dir)
		_, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "")
		require.ErrorContains(t, err, "remote")
	})

	t.Run("missing import file errors", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writePkg(t, dir, `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: missing
components:
  - name: missing
    import:
      local:
        - path: does-not-exist.yaml
`)
		pkg := loadV1Beta1Package(t, dir)
		_, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "")
		require.ErrorContains(t, err, "does-not-exist.yaml")
	})

	t.Run("directory import path errors", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, "child"), 0o700))
		writePkg(t, dir, `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: dir
components:
  - name: dir
    import:
      local:
        - path: child
`)
		pkg := loadV1Beta1Package(t, dir)
		_, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "")
		require.Error(t, err)
	})

	t.Run("multiple compatible variants error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: web
component: {}
`), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "b.yaml"), []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: web
component: {}
`), 0o600))
		writePkg(t, dir, `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: ambiguous
components:
  - name: web
    import:
      local:
        - path: a.yaml
        - path: b.yaml
`)
		pkg := loadV1Beta1Package(t, dir)
		_, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "")
		require.ErrorContains(t, err, "multiple")
	})
}
