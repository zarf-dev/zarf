// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/pkgcfg"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry"
)

func TestMetadataMatchesOCIPlatform(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		metadata v1beta1.ComponentMetadata
		platform *ocispec.Platform
		want     bool
	}{
		{name: "generic direct manifest", want: true},
		{name: "specific index manifest", metadata: v1beta1.ComponentMetadata{Variant: v1beta1.ComponentVariant{Architecture: "arm64"}}, platform: &ocispec.Platform{Architecture: "arm64"}, want: true},
		{name: "specific metadata on direct manifest", metadata: v1beta1.ComponentMetadata{Variant: v1beta1.ComponentVariant{Architecture: "arm64"}}, want: false},
		{name: "generic metadata in index", platform: &ocispec.Platform{Architecture: "arm64"}, want: false},
		{name: "architecture mismatch", metadata: v1beta1.ComponentMetadata{Variant: v1beta1.ComponentVariant{Architecture: "amd64"}}, platform: &ocispec.Platform{Architecture: "arm64"}, want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, metadataMatchesOCIPlatform(tt.metadata, tt.platform))
		})
	}
}

func TestRemoteComponentConfigRejectsPlatformMetadataMismatch(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	ref := registry.Reference{
		Registry:   testutil.SetupInMemoryRegistryDynamic(ctx, t),
		Repository: "components",
		Reference:  "mismatch",
	}
	component := v1beta1.ComponentConfig{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfComponentConfig,
		Metadata: v1beta1.ComponentMetadata{
			Name:    "mismatch",
			Variant: v1beta1.ComponentVariant{Architecture: "arm64"},
		},
	}
	componentJSON, err := json.Marshal(component)
	require.NoError(t, err)

	store := memory.New()
	configDescriptor := content.NewDescriptorFromBytes(layout.ZarfComponentConfigMediaType, componentJSON)
	require.NoError(t, store.Push(ctx, configDescriptor, bytes.NewReader(componentJSON)))
	manifest, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, "", oras.PackManifestOptions{ConfigDescriptor: &configDescriptor})
	require.NoError(t, err)
	require.NoError(t, store.Tag(ctx, manifest, manifest.Digest.String()))

	remote, err := zoci.NewRemote(ctx, ref.String(), ocispec.Platform{}, oci.WithPlainHTTP(true))
	require.NoError(t, err)
	_, err = oras.Copy(ctx, store, manifest.Digest.String(), remote.Repo(), ref.Reference, remote.GetDefaultCopyOpts())
	require.NoError(t, err)

	_, err = remoteComponentConfig(ctx, "oci://"+ref.String(), "arm64", types.RemoteOptions{PlainHTTP: true}, "")
	require.ErrorContains(t, err, "metadata architecture does not match its OCI platform")
}

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

func publishRemoteComponent(ctx context.Context, t *testing.T, reference string, resourcePaths ...string) registry.Reference {
	t.Helper()

	ref := registry.Reference{
		Registry:   testutil.SetupInMemoryRegistryDynamic(ctx, t),
		Repository: "components",
		Reference:  reference,
	}
	component := v1beta1.ComponentConfig{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfComponentConfig,
		Metadata:   v1beta1.ComponentMetadata{Name: reference},
		Component: v1beta1.ComponentSpec{
			Actions: v1beta1.ComponentActions{OnDeploy: v1beta1.ComponentActionSet{Before: []v1beta1.ComponentAction{{Cmd: "echo remote"}}}},
		},
	}
	componentJSON, err := json.Marshal(component)
	require.NoError(t, err)

	store := memory.New()
	configDescriptor := content.NewDescriptorFromBytes(layout.ZarfComponentConfigMediaType, componentJSON)
	require.NoError(t, store.Push(ctx, configDescriptor, bytes.NewReader(componentJSON)))
	layers := make([]ocispec.Descriptor, 0, len(resourcePaths))
	for _, resourcePath := range resourcePaths {
		resourceContents := []byte(resourcePath)
		resourceDescriptor := content.NewDescriptorFromBytes(layout.ZarfLayerMediaTypeBlob, resourceContents)
		resourceDescriptor.Annotations = map[string]string{
			layout.ComponentResourceMountPathAnnotation: resourcePath,
		}
		require.NoError(t, store.Push(ctx, resourceDescriptor, bytes.NewReader(resourceContents)))
		layers = append(layers, resourceDescriptor)
	}
	manifest, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, "", oras.PackManifestOptions{
		ConfigDescriptor: &configDescriptor,
		Layers:           layers,
	})
	require.NoError(t, err)
	require.NoError(t, store.Tag(ctx, manifest, manifest.Digest.String()))

	remote, err := zoci.NewRemote(ctx, ref.String(), ocispec.Platform{}, oci.WithPlainHTTP(true))
	require.NoError(t, err)
	_, err = oras.Copy(ctx, store, manifest.Digest.String(), remote.Repo(), ref.Reference, remote.GetDefaultCopyOpts())
	require.NoError(t, err)
	return ref
}

func resolveRemoteImport(ctx context.Context, t *testing.T, ref registry.Reference) v1beta1ImportResolution {
	t.Helper()

	dir := t.TempDir()
	writePackage := []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: remote
components:
  - name: remote
    import:
      remote:
        - url: oci://` + ref.String() + `
`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), writePackage, 0o600))

	pkg := loadV1Beta1Package(t, dir)
	resolution, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", types.RemoteOptions{PlainHTTP: true}, "")
	require.NoError(t, err)
	require.Len(t, resolution.pkg.Components, 1)
	require.Equal(t, []v1beta1.ComponentAction{{Cmd: "echo remote"}}, resolution.pkg.Components[0].Actions.OnDeploy.Before)
	return resolution
}

func TestResolveImportsV1Beta1(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	t.Run("remote import with resources", func(t *testing.T) {
		t.Parallel()

		ref := publishRemoteComponent(ctx, t, "remote-import", "resources/0/resource.txt")
		resolution := resolveRemoteImport(ctx, t, ref)
		require.Len(t, resolution.remoteResources, 1)
		require.Equal(t, "resources/0/resource.txt", resolution.remoteResources[0].mountPath)
	})

	t.Run("remote import without resources", func(t *testing.T) {
		t.Parallel()

		ref := publishRemoteComponent(ctx, t, "remote-import-no-resources")
		resolution := resolveRemoteImport(ctx, t, ref)
		require.Empty(t, resolution.remoteResources)
	})

	t.Run("single local import rebases paths and collects values", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "single")
		pkg := loadV1Beta1Package(t, dir)

		resolution, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", types.RemoteOptions{}, "")
		require.NoError(t, err)

		require.Len(t, resolution.pkg.Components, 1)
		comp := resolution.pkg.Components[0]
		require.Equal(t, "logging", comp.Name)
		require.Empty(t, comp.Import.Local)

		require.Len(t, comp.Charts, 1)
		require.NotNil(t, comp.Charts[0].Local)
		require.Equal(t, "components/loki-chart", comp.Charts[0].Local.Path)
		require.Equal(t, []v1beta1.ValuesFile{{Path: "components/loki-values.yaml"}}, comp.Charts[0].ValuesFiles)

		require.Len(t, comp.Files, 1)
		require.Equal(t, "components/motd.txt", comp.Files[0].Source)

		require.Equal(t, []v1beta1.Image{{Name: "grafana/loki:2.9.0"}}, comp.Images)

		require.Equal(t, []string{"components/logging-values.yaml"}, resolution.pkg.Values.Files)
		require.Equal(t, []string{"components/logging.schema.json"}, resolution.schemas)
	})

	t.Run("non-importing components are preserved alongside an importing one", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "mixed")
		pkg := loadV1Beta1Package(t, dir)

		resolution, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", types.RemoteOptions{}, "")
		require.NoError(t, err)

		require.Len(t, resolution.pkg.Components, 3)
		require.Equal(t, "first", resolution.pkg.Components[0].Name)
		require.Equal(t, []v1beta1.Image{{Name: "alpine:3.20"}}, resolution.pkg.Components[0].Images)
		require.Equal(t, "middle", resolution.pkg.Components[1].Name)
		require.Equal(t, []v1beta1.Image{{Name: "nginx:1.27"}}, resolution.pkg.Components[1].Images)
		require.Empty(t, resolution.pkg.Components[1].Import.Local)
		require.Equal(t, "last", resolution.pkg.Components[2].Name)
		require.Equal(t, []v1beta1.Image{{Name: "busybox:1.36"}}, resolution.pkg.Components[2].Images)
	})

	t.Run("nested imports merge and rebase transitively", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "nested")
		pkg := loadV1Beta1Package(t, dir)

		resolution, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", types.RemoteOptions{}, "")
		require.NoError(t, err)

		require.Len(t, resolution.pkg.Components, 1)
		comp := resolution.pkg.Components[0]
		require.Equal(t, "app", comp.Name)

		require.Len(t, comp.Charts, 1)
		require.NotNil(t, comp.Charts[0].Local)
		require.Equal(t, "components/app-chart", comp.Charts[0].Local.Path)

		require.Len(t, comp.Files, 1)
		require.Equal(t, "components/child/child.txt", comp.Files[0].Source)

		require.Equal(t, []string{
			"components/child/child-values.yaml",
			"components/app-values.yaml",
		}, resolution.pkg.Values.Files)
		require.Equal(t, []string{
			"components/app.schema.json",
			"components/child/child.schema.json",
		}, resolution.schemas)
	})

	t.Run("cyclic imports error", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "cycle")
		pkg := loadV1Beta1Package(t, dir)

		_, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", types.RemoteOptions{}, "")
		require.ErrorContains(t, err, "cycle")
	})

	t.Run("variant selection picks the compatible flavor", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "variants")
		pkg := loadV1Beta1Package(t, dir)

		resolution, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "apache", types.RemoteOptions{}, "")
		require.NoError(t, err)

		require.Len(t, resolution.pkg.Components, 1)
		require.Equal(t, []v1beta1.Image{{Name: "httpd:2.4"}}, resolution.pkg.Components[0].Images)
	})

	t.Run("variant selection errors when no variant is compatible", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "variants")
		pkg := loadV1Beta1Package(t, dir)

		_, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", types.RemoteOptions{}, "")
		require.ErrorContains(t, err, "no imported component")
	})

	t.Run("single import errors when component config is incompatible", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "single-incompatible")
		pkg := loadV1Beta1Package(t, dir)

		_, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "nginx", types.RemoteOptions{}, "")
		require.ErrorContains(t, err, "no imported component")
	})

	t.Run("package component overrides imported component", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "merge")
		pkg := loadV1Beta1Package(t, dir)

		resolution, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", types.RemoteOptions{}, "")
		require.NoError(t, err)

		comp := resolution.pkg.Components[0]
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

		require.Len(t, comp.Manifests, 1)
		require.Equal(t, "app", comp.Manifests[0].Name)
		require.NotNil(t, comp.Manifests[0].Kustomize)
		require.Equal(t, []string{"components/base-kustomization", "override-kustomization"}, comp.Manifests[0].Kustomize.Files)
		require.True(t, comp.Manifests[0].Kustomize.AllowAnyDirectory)
		require.True(t, comp.Manifests[0].Kustomize.EnablePlugins)

		require.NotNil(t, comp.Actions.OnDeploy.Defaults)
		require.Equal(t, int32(0), comp.Actions.OnDeploy.Defaults.MaxTotalSeconds)
		require.Equal(t, int32(0), comp.Actions.OnDeploy.Defaults.Retries)
		require.Empty(t, comp.Actions.OnDeploy.Defaults.Env)
		require.Equal(t, []v1beta1.ComponentAction{
			{Cmd: "echo base"},
			{Cmd: "echo override"},
		}, comp.Actions.OnDeploy.Before)
	})
}

func TestResolveImportsV1Beta1Errors(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	writePkg := func(t *testing.T, dir, body string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), []byte(body), 0o600))
	}

	writeComponent := func(t *testing.T, dir, name, body string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	}

	requireLintErr := func(t *testing.T, err error, path string) {
		t.Helper()
		var lintErr *lint.LintError
		require.ErrorAs(t, err, &lintErr)
		require.Equal(t, path, lintErr.PackageName)
		require.NotEmpty(t, lintErr.Findings)
	}

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
		_, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", types.RemoteOptions{}, "")
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
		_, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", types.RemoteOptions{}, "")
		require.Error(t, err)
	})

	t.Run("missing component config kind errors", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeComponent(t, dir, "child.yaml", `apiVersion: zarf.dev/v1beta1
metadata:
  name: child
component: {}
`)
		writePkg(t, dir, `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: missing-kind
components:
  - name: child
    import:
      local:
        - path: child.yaml
`)
		pkg := loadV1Beta1Package(t, dir)
		_, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", types.RemoteOptions{}, "")
		require.ErrorContains(t, err, "kind")
	})

	t.Run("component config schema errors", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeComponent(t, dir, "child.yaml", `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: child
component: {}
unknown: value
`)
		writePkg(t, dir, `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: schema-error
components:
  - name: child
    import:
      local:
        - path: child.yaml
`)
		pkg := loadV1Beta1Package(t, dir)
		_, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", types.RemoteOptions{}, "")
		requireLintErr(t, err, filepath.Join(dir, "child.yaml"))
	})

	t.Run("component config selector is rejected", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeComponent(t, dir, "child.yaml", `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: child
component:
  selector:
    architecture: amd64
`)
		writePkg(t, dir, `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: selector
components:
  - name: child
    import:
      local:
        - path: child.yaml
`)
		pkg := loadV1Beta1Package(t, dir)
		_, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", types.RemoteOptions{}, "")
		requireLintErr(t, err, filepath.Join(dir, "child.yaml"))
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
		_, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", types.RemoteOptions{}, "")
		require.ErrorContains(t, err, "multiple")
	})
}
