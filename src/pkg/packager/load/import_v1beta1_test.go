// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry"

	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/pkgcfg"
	"github.com/zarf-dev/zarf/src/pkg/packager/component"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
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
	pkg, err := pkgcfg.ParseAs(ctx, b, pkgcfg.V1Beta1)
	require.NoError(t, err)
	return pkg
}

func writeV1Beta1File(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}

func writeV1Beta1Package(t *testing.T, dir, body string) {
	t.Helper()
	writeV1Beta1File(t, filepath.Join(dir, layout.ZarfYAML), body)
}

type componentPullMock struct {
	paths                 map[string]string
	calls                 []string
	cleanups              map[string]int
	expectedCachePath     string
	expectedRemoteOptions types.RemoteOptions
}

func installComponentPullMock(t *testing.T, mock *componentPullMock) {
	t.Helper()
	if mock.cleanups == nil {
		mock.cleanups = map[string]int{}
	}
	orig := componentPull
	componentPull = mock.pull
	t.Cleanup(func() {
		componentPull = orig
	})
}

func (m *componentPullMock) pull(_ context.Context, ref registry.Reference, opts component.PullOptions) (string, func() error, error) {
	m.calls = append(m.calls, ref.String())
	if m.expectedCachePath != "" && opts.CachePath != m.expectedCachePath {
		return "", nil, fmt.Errorf("unexpected cache path %q", opts.CachePath)
	}
	if opts.RemoteOptions != m.expectedRemoteOptions {
		return "", nil, fmt.Errorf("unexpected remote options %#v", opts.RemoteOptions)
	}
	localPath, ok := m.paths[ref.String()]
	if !ok {
		return "", nil, fmt.Errorf("unexpected pull for %s", ref.String())
	}
	return localPath, func() error {
		m.cleanups[ref.String()]++
		return nil
	}, nil
}

func TestResolveImportsV1Beta1(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	t.Run("single local import rebases paths and collects values", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "single")
		pkg := loadV1Beta1Package(t, dir)

		resolved, schemas, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", "", types.RemoteOptions{})
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

		resolved, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", "", types.RemoteOptions{})
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

		resolved, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", "", types.RemoteOptions{})
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

		_, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", "", types.RemoteOptions{})
		require.ErrorContains(t, err, "cycle")
	})

	t.Run("variant selection picks the compatible flavor", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "variants")
		pkg := loadV1Beta1Package(t, dir)

		resolved, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "apache", "", types.RemoteOptions{})
		require.NoError(t, err)

		require.Len(t, resolved.Components, 1)
		require.Equal(t, []v1beta1.Image{{Name: "httpd:2.4"}}, resolved.Components[0].Images)
	})

	t.Run("variant selection errors when no variant is compatible", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "variants")
		pkg := loadV1Beta1Package(t, dir)

		_, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", "", types.RemoteOptions{})
		require.ErrorContains(t, err, "no imported component")
	})

	t.Run("package component overrides imported component", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "merge")
		pkg := loadV1Beta1Package(t, dir)

		resolved, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", "", types.RemoteOptions{})
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

func TestResolveRemoteImportsV1Beta1(t *testing.T) {
	ctx := testutil.TestContext(t)

	t.Run("single remote import rebases paths and collects values", func(t *testing.T) {
		root := t.TempDir()
		pkgDir := filepath.Join(root, "pkg")
		remotePath := filepath.Join(root, "remote", "logging", "component.yaml")
		cachePath := filepath.Join(root, "cache")
		remoteOptions := types.RemoteOptions{PlainHTTP: true}
		writeV1Beta1Package(t, pkgDir, `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: remote-single
components:
  - name: logging
    import:
      remote:
        - url: oci://registry.test/components/logging:1.0.0
`)
		writeV1Beta1File(t, remotePath, `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: logging
component:
  charts:
    - name: loki
      namespace: logging
      local:
        path: loki-chart
      valuesFiles:
        - path: loki-values.yaml
  files:
    - source: motd.txt
      destination: /etc/motd
  images:
    - name: grafana/loki:2.9.0
values:
  files:
    - logging-values.yaml
  schema: logging.schema.json
`)
		mock := &componentPullMock{
			paths:                 map[string]string{"registry.test/components/logging:1.0.0": remotePath},
			expectedCachePath:     cachePath,
			expectedRemoteOptions: remoteOptions,
		}
		installComponentPullMock(t, mock)

		pkg := loadV1Beta1Package(t, pkgDir)
		resolved, schemas, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, pkgDir), "amd64", "", cachePath, remoteOptions)
		require.NoError(t, err)

		remoteRel, err := filepath.Rel(pkgDir, remoteComponentImportCacheDir(cachePath, "registry.test/components/logging:1.0.0"))
		require.NoError(t, err)
		remoteRel = filepath.ToSlash(remoteRel)
		require.Len(t, resolved.Components, 1)
		comp := resolved.Components[0]
		require.Empty(t, comp.Import.Remote)
		require.Equal(t, filepath.ToSlash(filepath.Join(remoteRel, "loki-chart")), comp.Charts[0].Local.Path)
		require.Equal(t, []v1beta1.ValuesFile{{Path: filepath.ToSlash(filepath.Join(remoteRel, "loki-values.yaml"))}}, comp.Charts[0].ValuesFiles)
		require.Equal(t, filepath.ToSlash(filepath.Join(remoteRel, "motd.txt")), comp.Files[0].Source)
		require.Equal(t, []v1beta1.Image{{Name: "grafana/loki:2.9.0"}}, comp.Images)
		require.Equal(t, []string{filepath.ToSlash(filepath.Join(remoteRel, "logging-values.yaml"))}, resolved.Values.Files)
		require.Equal(t, []string{filepath.ToSlash(filepath.Join(remoteRel, "logging.schema.json"))}, schemas)
		require.Equal(t, []string{"registry.test/components/logging:1.0.0"}, mock.calls)
		require.Equal(t, map[string]int{"registry.test/components/logging:1.0.0": 1}, mock.cleanups)
	})

	t.Run("mixed local and remote variants select compatible component", func(t *testing.T) {
		for _, tc := range []struct {
			name             string
			flavor           string
			expectedImage    string
			expectedCalls    []string
			expectedCleanups map[string]int
		}{
			{
				name:             "local",
				flavor:           "nginx",
				expectedImage:    "nginx:1.27",
				expectedCalls:    []string{"registry.test/components/web:1.0.0"},
				expectedCleanups: map[string]int{"registry.test/components/web:1.0.0": 1},
			},
			{
				name:             "remote",
				flavor:           "apache",
				expectedImage:    "httpd:2.4",
				expectedCalls:    []string{"registry.test/components/web:1.0.0"},
				expectedCleanups: map[string]int{"registry.test/components/web:1.0.0": 1},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				root := t.TempDir()
				pkgDir := filepath.Join(root, "pkg")
				remotePath := filepath.Join(root, "remote", "web", "component.yaml")
				writeV1Beta1Package(t, pkgDir, `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: mixed-variants
components:
  - name: web
    import:
      local:
        - path: local.yaml
      remote:
        - url: oci://registry.test/components/web:1.0.0
`)
				writeV1Beta1File(t, filepath.Join(pkgDir, "local.yaml"), `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: web
component:
  selector:
    flavor: nginx
  images:
    - name: nginx:1.27
`)
				writeV1Beta1File(t, remotePath, `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: web
component:
  selector:
    flavor: apache
  images:
    - name: httpd:2.4
`)
				mock := &componentPullMock{
					paths: map[string]string{"registry.test/components/web:1.0.0": remotePath},
				}
				installComponentPullMock(t, mock)

				cachePath := filepath.Join(root, "cache")
				pkg := loadV1Beta1Package(t, pkgDir)
				resolved, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, pkgDir), "amd64", tc.flavor, cachePath, types.RemoteOptions{})
				require.NoError(t, err)

				require.Equal(t, []v1beta1.Image{{Name: tc.expectedImage}}, resolved.Components[0].Images)
				require.Equal(t, tc.expectedCalls, mock.calls)
				require.Equal(t, tc.expectedCleanups, mock.cleanups)
			})
		}
	})

	t.Run("remote nested import rebases transitively", func(t *testing.T) {
		root := t.TempDir()
		pkgDir := filepath.Join(root, "pkg")
		appPath := filepath.Join(root, "remote", "app", "component.yaml")
		basePath := filepath.Join(root, "remote", "base", "component.yaml")
		cachePath := filepath.Join(root, "cache")
		writeV1Beta1Package(t, pkgDir, `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: remote-nested
components:
  - name: app
    import:
      remote:
        - url: oci://registry.test/components/app:1.0.0
`)
		writeV1Beta1File(t, appPath, `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: app
component:
  import:
    remote:
      - url: oci://registry.test/components/base:1.0.0
  charts:
    - name: app
      namespace: app
      local:
        path: chart
`)
		writeV1Beta1File(t, basePath, `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: base
component:
  files:
    - source: base.txt
      destination: /etc/base.txt
`)
		mock := &componentPullMock{
			paths: map[string]string{
				"registry.test/components/app:1.0.0":  appPath,
				"registry.test/components/base:1.0.0": basePath,
			},
		}
		installComponentPullMock(t, mock)

		pkg := loadV1Beta1Package(t, pkgDir)
		resolved, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, pkgDir), "amd64", "", cachePath, types.RemoteOptions{})
		require.NoError(t, err)

		appRel, err := filepath.Rel(pkgDir, remoteComponentImportCacheDir(cachePath, "registry.test/components/app:1.0.0"))
		require.NoError(t, err)
		baseRel, err := filepath.Rel(pkgDir, remoteComponentImportCacheDir(cachePath, "registry.test/components/base:1.0.0"))
		require.NoError(t, err)

		comp := resolved.Components[0]
		require.Equal(t, filepath.ToSlash(filepath.Join(appRel, "chart")), comp.Charts[0].Local.Path)
		require.Equal(t, filepath.ToSlash(filepath.Join(baseRel, "base.txt")), comp.Files[0].Source)
		require.Equal(t, []string{"registry.test/components/app:1.0.0", "registry.test/components/base:1.0.0"}, mock.calls)
		require.Equal(t, map[string]int{"registry.test/components/app:1.0.0": 1, "registry.test/components/base:1.0.0": 1}, mock.cleanups)
	})

	t.Run("remote cycle error uses canonical ref", func(t *testing.T) {
		root := t.TempDir()
		pkgDir := filepath.Join(root, "pkg")
		aPath := filepath.Join(root, "remote", "a", "component.yaml")
		bPath := filepath.Join(root, "remote", "b", "component.yaml")
		cachePath := filepath.Join(root, "cache")
		writeV1Beta1Package(t, pkgDir, `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: remote-cycle
components:
  - name: cycle
    import:
      remote:
        - url: oci://registry.test/components/a:1.0.0
`)
		writeV1Beta1File(t, aPath, `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: a
component:
  import:
    remote:
      - url: oci://registry.test/components/b:1.0.0
`)
		writeV1Beta1File(t, bPath, `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: b
component:
  import:
    remote:
      - url: oci://registry.test/components/a:1.0.0
`)
		mock := &componentPullMock{
			paths: map[string]string{
				"registry.test/components/a:1.0.0": aPath,
				"registry.test/components/b:1.0.0": bPath,
			},
		}
		installComponentPullMock(t, mock)

		pkg := loadV1Beta1Package(t, pkgDir)
		_, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, pkgDir), "amd64", "", cachePath, types.RemoteOptions{})
		require.ErrorContains(t, err, "registry.test/components/a:1.0.0")
		require.NotContains(t, err.Error(), "oci://registry.test/components/a:1.0.0")
		require.Equal(t, []string{"registry.test/components/a:1.0.0", "registry.test/components/b:1.0.0"}, mock.calls)
		require.Equal(t, map[string]int{"registry.test/components/a:1.0.0": 1, "registry.test/components/b:1.0.0": 1}, mock.cleanups)
	})
}

func TestResolveImportsV1Beta1Errors(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	writePkg := func(t *testing.T, dir, body string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), []byte(body), 0o600))
	}

	t.Run("remote import without tag or digest errors before pull", func(t *testing.T) {
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
        - url: oci://example.com/component
`)
		pkg := loadV1Beta1Package(t, dir)
		_, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", "", types.RemoteOptions{})
		require.ErrorContains(t, err, "tag or digest")
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
		_, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", "", types.RemoteOptions{})
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
		_, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", "", types.RemoteOptions{})
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
		_, _, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", "", types.RemoteOptions{})
		require.ErrorContains(t, err, "multiple")
	})
}
