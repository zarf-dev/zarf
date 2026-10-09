// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager/requirements"
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

func TestVariantMatchesOCIPlatform(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		variant  v1beta1.ComponentVariant
		platform *ocispec.Platform
		want     bool
	}{
		{name: "generic direct manifest", want: true},
		{name: "specific index manifest", variant: v1beta1.ComponentVariant{Architecture: "arm64"}, platform: &ocispec.Platform{Architecture: "arm64"}, want: true},
		{name: "specific variant on direct manifest", variant: v1beta1.ComponentVariant{Architecture: "arm64"}, want: false},
		{name: "generic variant in index", platform: &ocispec.Platform{Architecture: "arm64"}, want: false},
		{name: "architecture mismatch", variant: v1beta1.ComponentVariant{Architecture: "amd64"}, platform: &ocispec.Platform{Architecture: "arm64"}, want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, variantMatchesOCIPlatform(tt.variant, tt.platform))
		})
	}
}

func TestRemoteComponentConfigRejectsPlatformVariantMismatch(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	ref := registry.Reference{
		Registry:   testutil.SetupInMemoryRegistryDynamic(ctx, t),
		Repository: "components",
		Reference:  "mismatch",
	}
	component := v1beta1.ComponentConfig{
		APIVersion:  v1beta1.APIVersion,
		Kind:        v1beta1.ZarfComponentConfig,
		Metadata:    v1beta1.ComponentMetadata{Name: "mismatch", Version: "0.0.1"},
		Variant:     v1beta1.ComponentVariant{Architecture: "arm64"},
		PublishData: v1beta1.ComponentPublishData{ZarfVersion: "test"},
	}
	componentJSON, err := json.Marshal(component)
	require.NoError(t, err)

	store := memory.New()
	configDescriptor := content.NewDescriptorFromBytes(layout.ZarfComponentConfigMediaType, componentJSON)
	require.NoError(t, store.Push(ctx, configDescriptor, bytes.NewReader(componentJSON)))
	manifest, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, "", oras.PackManifestOptions{ConfigDescriptor: &configDescriptor})
	require.NoError(t, err)
	require.NoError(t, store.Tag(ctx, manifest, manifest.Digest.String()))

	remote, err := zoci.NewRemoteWithOptions(ctx, ref.String(), ocispec.Platform{}, zoci.RemoteClientOptions{
		RemoteOptions: types.RemoteOptions{PlainHTTP: true},
	})
	require.NoError(t, err)
	_, err = oras.Copy(ctx, store, manifest.Digest.String(), remote.Repo(), ref.Reference, remote.GetDefaultCopyOpts())
	require.NoError(t, err)

	_, err = remoteComponentConfig(ctx, "oci://"+ref.String(), "arm64", types.RemoteOptions{PlainHTTP: true}, "")
	require.ErrorContains(t, err, "variant architecture does not match its OCI platform")
}

func TestRemoteComponentConfigRejectsOnCreateActions(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	ref := registry.Reference{
		Registry:   testutil.SetupInMemoryRegistryDynamic(ctx, t),
		Repository: "components",
		Reference:  "on-create",
	}
	component := v1beta1.ComponentConfig{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfComponentConfig,
		Metadata:   v1beta1.ComponentMetadata{Name: "on-create"},
		Component: v1beta1.ComponentSpec{Actions: v1beta1.ComponentActions{
			OnCreate: v1beta1.ComponentActionSet{Before: []v1beta1.ComponentAction{{Cmd: "touch unexpected"}}},
		}},
	}
	componentJSON, err := json.Marshal(component)
	require.NoError(t, err)
	store := memory.New()
	configDescriptor := content.NewDescriptorFromBytes(layout.ZarfComponentConfigMediaType, componentJSON)
	require.NoError(t, store.Push(ctx, configDescriptor, bytes.NewReader(componentJSON)))
	manifest, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, "", oras.PackManifestOptions{ConfigDescriptor: &configDescriptor})
	require.NoError(t, err)
	require.NoError(t, store.Tag(ctx, manifest, manifest.Digest.String()))
	remote, err := zoci.NewRemoteWithOptions(ctx, ref.String(), ocispec.Platform{}, zoci.RemoteClientOptions{RemoteOptions: types.RemoteOptions{PlainHTTP: true}})
	require.NoError(t, err)
	_, err = oras.Copy(ctx, store, manifest.Digest.String(), remote.Repo(), ref.Reference, remote.GetDefaultCopyOpts())
	require.NoError(t, err)

	_, err = remoteComponentConfig(ctx, "oci://"+ref.String(), "amd64", types.RemoteOptions{PlainHTTP: true}, "")
	require.ErrorContains(t, err, "unsupported onCreate actions")
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
	return publishRemoteComponentToReference(ctx, t, ref, resourcePaths...)
}

func publishRemoteComponentToReference(ctx context.Context, t *testing.T, ref registry.Reference, resourcePaths ...string) registry.Reference {
	t.Helper()

	component := v1beta1.ComponentConfig{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfComponentConfig,
		Metadata:   v1beta1.ComponentMetadata{Name: ref.Reference},
		Component: v1beta1.ComponentSpec{
			Actions: v1beta1.ComponentActions{OnDeploy: v1beta1.ComponentActionSet{Before: []v1beta1.ComponentAction{{Cmd: "echo remote"}}}},
		},
	}
	for _, resourcePath := range resourcePaths {
		component.Component.Files = append(component.Component.Files, v1beta1.File{Source: resourcePath, Destination: "/tmp/" + filepath.Base(resourcePath)})
	}
	return publishRemoteComponentConfig(ctx, t, ref, component, resourcePaths...)
}

func publishRemoteComponentConfig(ctx context.Context, t *testing.T, ref registry.Reference, component v1beta1.ComponentConfig, resourcePaths ...string) registry.Reference {
	t.Helper()

	store := memory.New()
	var contents bytes.Buffer
	writer := tar.NewWriter(&contents)
	for _, resourcePath := range resourcePaths {
		payload := []byte(resourcePath)
		require.NoError(t, writer.WriteHeader(&tar.Header{Name: resourcePath, Mode: 0o600, Size: int64(len(payload)), Typeflag: tar.TypeReg}))
		_, err := writer.Write(payload)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	descriptor := content.NewDescriptorFromBytes(layout.ZarfComponentLayerMediaTypeTar, contents.Bytes())
	descriptor.Annotations = map[string]string{ocispec.AnnotationTitle: layout.ComponentTar}
	require.NoError(t, store.Push(ctx, descriptor, bytes.NewReader(contents.Bytes())))
	return publishRemoteComponentArtifact(ctx, t, ref, component, store, []ocispec.Descriptor{descriptor})
}

func publishRemoteComponentArtifact(ctx context.Context, t *testing.T, ref registry.Reference, component v1beta1.ComponentConfig, store *memory.Store, layers []ocispec.Descriptor) registry.Reference {
	t.Helper()
	componentJSON, err := json.Marshal(component)
	require.NoError(t, err)

	configDescriptor := content.NewDescriptorFromBytes(layout.ZarfComponentConfigMediaType, componentJSON)
	require.NoError(t, store.Push(ctx, configDescriptor, bytes.NewReader(componentJSON)))
	manifest, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, "", oras.PackManifestOptions{
		ConfigDescriptor: &configDescriptor,
		Layers:           layers,
	})
	require.NoError(t, err)
	require.NoError(t, store.Tag(ctx, manifest, manifest.Digest.String()))

	remote, err := zoci.NewRemoteWithOptions(ctx, ref.String(), ocispec.Platform{}, zoci.RemoteClientOptions{
		RemoteOptions: types.RemoteOptions{PlainHTTP: true},
	})
	require.NoError(t, err)
	_, err = oras.Copy(ctx, store, manifest.Digest.String(), remote.Repo(), ref.Reference, remote.GetDefaultCopyOpts())
	require.NoError(t, err)
	return ref
}

func TestPackageDefinitionEnforcesRemoteComponentVersionRequirements(t *testing.T) {
	originalVersion := config.CLIVersion
	t.Cleanup(func() { config.CLIVersion = originalVersion })
	config.CLIVersion = "v0.88.0"

	ctx := testutil.TestContext(t)
	ref := registry.Reference{
		Registry:   testutil.SetupInMemoryRegistryDynamic(ctx, t),
		Repository: "components",
		Reference:  "requires-newer-cli",
	}
	publishRemoteComponentConfig(ctx, t, ref, v1beta1.ComponentConfig{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfComponentConfig,
		Metadata:   v1beta1.ComponentMetadata{Name: "requires-newer-cli"},
		Component: v1beta1.ComponentSpec{
			Actions: v1beta1.ComponentActions{OnDeploy: v1beta1.ComponentActionSet{
				Before: []v1beta1.ComponentAction{{Cmd: "echo remote"}},
			}},
		},
		PublishData: v1beta1.ComponentPublishData{
			VersionRequirements: []v1beta1.VersionRequirement{
				{Version: "v0.87.0", Reason: "supported feature"},
				{Version: "v0.89.0", Reason: "requires newer feature"},
			},
		},
	})

	dir := t.TempDir()
	definition := []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: remote-requirements
components:
  - name: remote
    import:
      remote:
        - url: oci://` + ref.String() + `
`)
	packagePath := filepath.Join(dir, layout.ZarfYAML)
	require.NoError(t, os.WriteFile(packagePath, definition, 0o600))
	opts := DefinitionOptions{RemoteOptions: types.RemoteOptions{PlainHTTP: true}}

	_, err := PackageDefinition(ctx, packagePath, opts)
	var requirementErr *requirements.VersionRequirementsError
	require.ErrorAs(t, err, &requirementErr)
	require.Equal(t, "v0.89.0", requirementErr.RequiredVersion)
	require.Equal(t, "v0.88.0", requirementErr.CurrentVersion)
	require.ErrorContains(t, err, "requires newer feature")
	require.ErrorContains(t, err, ref.String())

	opts.SkipVersionCheck = true
	_, err = PackageDefinition(ctx, packagePath, opts)
	require.NoError(t, err)

	opts.SkipVersionCheck = false
	config.CLIVersion = "v0.89.0"
	_, err = PackageDefinition(ctx, packagePath, opts)
	require.NoError(t, err)
}

func TestPackageRemoteComponentArchive(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		entryPath string
		source    string
		wantErr   string
	}{
		{name: "bundled file", entryPath: "resources/file.txt", source: "resources/file.txt"},
		{name: "missing file", entryPath: "resources/other.txt", source: "resources/file.txt", wantErr: "absent from artifact layers"},
		{name: "unsafe archive path", entryPath: "../outside.txt", source: "resources/file.txt", wantErr: ".."},
		{name: "unsafe config path", entryPath: "resources/file.txt", source: "../outside.txt", wantErr: "invalid local resource path"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)
			ref := registry.Reference{
				Registry:   testutil.SetupInMemoryRegistryDynamic(ctx, t),
				Repository: "components",
				Reference:  "archive",
			}
			component := v1beta1.ComponentConfig{
				APIVersion: v1beta1.APIVersion,
				Kind:       v1beta1.ZarfComponentConfig,
				Metadata:   v1beta1.ComponentMetadata{Name: "archive"},
				Component:  v1beta1.ComponentSpec{Files: []v1beta1.File{{Source: tt.source, Destination: "/tmp/file.txt"}}},
			}
			var contents bytes.Buffer
			writer := tar.NewWriter(&contents)
			payload := []byte("bundled file contents")
			require.NoError(t, writer.WriteHeader(&tar.Header{Name: tt.entryPath, Mode: 0o600, Size: int64(len(payload)), Typeflag: tar.TypeReg}))
			_, err := writer.Write(payload)
			require.NoError(t, err)
			require.NoError(t, writer.Close())
			store := memory.New()
			descriptor := content.NewDescriptorFromBytes(layout.ZarfComponentLayerMediaTypeTar, contents.Bytes())
			descriptor.Annotations = map[string]string{ocispec.AnnotationTitle: layout.ComponentTar}
			require.NoError(t, store.Push(ctx, descriptor, bytes.NewReader(contents.Bytes())))
			publishRemoteComponentArtifact(ctx, t, ref, component, store, []ocispec.Descriptor{descriptor})

			dir := t.TempDir()
			manifest := []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: remote
components:
  - name: remote
    import:
      remote:
        - url: oci://` + ref.String() + "\n")
			require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), manifest, 0o600))
			loaded, err := Package(ctx, dir, PackageOptions{DefinitionOptions: DefinitionOptions{
				CachePath: t.TempDir(), RemoteOptions: types.RemoteOptions{PlainHTTP: true},
			}})
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, loaded.Close()) })
			data, err := loaded.Resources.ReadFile(loaded.Definition.Components[0].Files[0].Source)
			require.NoError(t, err)
			require.Equal(t, payload, data)
		})
	}
}

func TestRemoteImportRejectsUnbundledLocalResources(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		set    func(*v1beta1.ComponentConfig, string)
		field  string
		reason string
	}{
		{name: "absolute file source", set: func(c *v1beta1.ComponentConfig, source string) {
			c.Component.Files = []v1beta1.File{{Source: source, Destination: "/tmp/target"}}
		}, field: "component.files[0].source", reason: "invalid local resource path"},
		{name: "parent traversal", set: func(c *v1beta1.ComponentConfig, _ string) {
			c.Component.Files = []v1beta1.File{{Source: "../secret.txt", Destination: "/tmp/target"}}
		}, field: "component.files[0].source", reason: "invalid local resource path"},
		{name: "unbundled relative source", set: func(c *v1beta1.ComponentConfig, _ string) {
			c.Component.Files = []v1beta1.File{{Source: "secret.txt", Destination: "/tmp/target"}}
		}, field: "component.files[0].source", reason: "absent from artifact layers"},
		{name: "absolute values schema", set: func(c *v1beta1.ComponentConfig, source string) {
			c.Values.Schema = source
		}, field: "values.schema", reason: "invalid local resource path"},
		{name: "absolute chart path", set: func(c *v1beta1.ComponentConfig, source string) {
			c.Component.Charts = []v1beta1.Chart{{Name: "chart", Local: &v1beta1.LocalSource{Path: source}}}
		}, field: "component.charts[0].local.path", reason: "invalid local resource path"},
		{name: "absolute manifest path", set: func(c *v1beta1.ComponentConfig, source string) {
			c.Component.Manifests = []v1beta1.Manifest{{Name: "manifest", Files: []string{source}}}
		}, field: "component.manifests[0].files[0]", reason: "invalid local resource path"},
		{name: "absolute image archive path", set: func(c *v1beta1.ComponentConfig, source string) {
			c.Component.ImageArchives = []v1beta1.ImageArchive{{Path: source, Images: []string{"example.com/image:1"}}}
		}, field: "component.imageArchives[0].path", reason: "invalid local resource path"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := testutil.TestContext(t)
			dir := t.TempDir()
			secret := filepath.Join(dir, "secret.txt")
			require.NoError(t, os.WriteFile(secret, []byte("private data"), 0o600))
			component := v1beta1.ComponentConfig{
				APIVersion: v1beta1.APIVersion,
				Kind:       v1beta1.ZarfComponentConfig,
				Metadata:   v1beta1.ComponentMetadata{Name: "untrusted"},
			}
			tt.set(&component, secret)
			ref := registry.Reference{
				Registry:   testutil.SetupInMemoryRegistryDynamic(ctx, t),
				Repository: "components",
				Reference:  "untrusted",
			}
			publishRemoteComponentConfig(ctx, t, ref, component)
			manifest := []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: remote
components:
  - name: remote
    import:
      remote:
        - url: oci://` + ref.String() + "\n")
			require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), manifest, 0o600))

			_, err := Package(ctx, dir, PackageOptions{DefinitionOptions: DefinitionOptions{
				CachePath:     t.TempDir(),
				RemoteOptions: types.RemoteOptions{PlainHTTP: true},
			}})
			require.ErrorContains(t, err, tt.field)
			require.ErrorContains(t, err, tt.reason)
		})
	}
}

func TestRemoteImportRejectsAllowAnyDirectory(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	ref := registry.Reference{
		Registry:   testutil.SetupInMemoryRegistryDynamic(ctx, t),
		Repository: "components",
		Reference:  "unrestricted-kustomize",
	}
	component := v1beta1.ComponentConfig{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfComponentConfig,
		Metadata:   v1beta1.ComponentMetadata{Name: "unrestricted-kustomize"},
		Component: v1beta1.ComponentSpec{Manifests: []v1beta1.Manifest{{
			Name: "app",
			Kustomize: v1beta1.KustomizeManifest{
				Files:             []string{"resources/kustomization"},
				AllowAnyDirectory: true,
			},
		}}},
	}
	publishRemoteComponentConfig(ctx, t, ref, component, "resources/kustomization")

	manifest := []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: remote
components:
  - name: remote
    import:
      remote:
        - url: oci://` + ref.String() + "\n")
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), manifest, 0o600))

	_, err := PackageDefinition(ctx, dir, DefinitionOptions{
		CachePath:     t.TempDir(),
		RemoteOptions: types.RemoteOptions{PlainHTTP: true},
	})
	require.ErrorContains(t, err, `manifest "app" uses kustomize.allowAnyDirectory`)
}

func TestPackageRejectsUnsafeComponentArchivePaths(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		mountPath string
	}{
		{name: "parent-traversal", mountPath: "../outside"},
		{name: "embedded-parent-traversal", mountPath: "resources/../outside"},
		{name: "backslash-traversal", mountPath: `..\..\outside`},
		{name: "backslash-separator", mountPath: `resources\payload.txt`},
		{name: "mixed-separators", mountPath: `resources/..\payload.txt`},
		{name: "absolute-path", mountPath: "/absolute/path"},
		{name: "empty-path", mountPath: ""},
		{name: "current-directory", mountPath: "."},
		{name: "windows-drive-path", mountPath: `C:/outside.txt`},
		{name: "windows-drive-with-backslashes", mountPath: `C:\outside.txt`},
		{name: "windows-alternate-data-stream", mountPath: `resources/payload.txt:stream`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := testutil.TestContext(t)
			ref := registry.Reference{Registry: testutil.SetupInMemoryRegistryDynamic(ctx, t), Repository: "components", Reference: tt.name}
			component := v1beta1.ComponentConfig{APIVersion: v1beta1.APIVersion, Kind: v1beta1.ZarfComponentConfig, Metadata: v1beta1.ComponentMetadata{Name: tt.name}}
			publishRemoteComponentConfig(ctx, t, ref, component, tt.mountPath)
			dir := t.TempDir()
			writeRemoteImportPackage(t, dir, ref)
			_, err := Package(ctx, dir, PackageOptions{DefinitionOptions: DefinitionOptions{RemoteOptions: types.RemoteOptions{PlainHTTP: true}}})
			require.ErrorContains(t, err, "extracting remote component archive")
		})
	}
}

func TestRemoteImportResolutionPinsReferencesForOneInvocation(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	ref := registry.Reference{
		Registry:   testutil.SetupInMemoryRegistryDynamic(ctx, t),
		Repository: "components",
		Reference:  "mutable",
	}
	publishRemoteComponentToReference(ctx, t, ref, "resources/0/first.txt")
	imports := v1beta1.ComponentImport{Remote: []v1beta1.ComponentImportRemote{{URL: "oci://" + ref.String()}}}
	cache := remoteReferenceCache{}

	first, err := selectImportVariant(ctx, imports, t.TempDir(), "amd64", "", nil, types.RemoteOptions{PlainHTTP: true}, "", cache)
	require.NoError(t, err)

	publishRemoteComponentToReference(ctx, t, ref, "resources/0/second.txt")
	second, err := selectImportVariant(ctx, imports, t.TempDir(), "amd64", "", nil, types.RemoteOptions{PlainHTTP: true}, "", cache)
	require.NoError(t, err)
	require.Equal(t, first.path, second.path)
	require.Equal(t, first.resources, second.resources)
}

func writeRemoteImportPackage(t *testing.T, dir string, ref registry.Reference) {
	t.Helper()
	writePackage := []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: remote
components:
  - name: remote
    import:
      remote:
        - url: oci://` + ref.String() + "\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), writePackage, 0o600))
}

func loadRemoteImport(ctx context.Context, t *testing.T, ref registry.Reference) *ResolvedPackage {
	t.Helper()
	dir := t.TempDir()
	writeRemoteImportPackage(t, dir, ref)
	loaded, err := Package(ctx, dir, PackageOptions{DefinitionOptions: DefinitionOptions{RemoteOptions: types.RemoteOptions{PlainHTTP: true}}})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, loaded.Close()) })
	require.Len(t, loaded.Definition.Components, 1)
	require.Equal(t, "echo remote", loaded.Definition.Components[0].Actions.OnDeploy.Before[0].Cmd)
	return loaded
}

func TestResolveImportsV1Beta1(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	t.Run("remote import with resources", func(t *testing.T) {
		t.Parallel()

		ref := publishRemoteComponent(ctx, t, "remote-import", "resources/0/resource.txt")
		loaded := loadRemoteImport(ctx, t, ref)
		contents, err := loaded.Resources.ReadFile(loaded.Definition.Components[0].Files[0].Source)
		require.NoError(t, err)
		require.Equal(t, "resources/0/resource.txt", string(contents))
	})

	t.Run("remote import without resources", func(t *testing.T) {
		t.Parallel()

		ref := publishRemoteComponent(ctx, t, "remote-import-no-resources")
		loaded := loadRemoteImport(ctx, t, ref)
		require.Empty(t, loaded.Definition.Components[0].Files)
	})

	t.Run("single local import rebases paths and collects values", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "single")
		pkg := loadV1Beta1Package(t, dir)

		resolution, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", false, types.RemoteOptions{}, "")
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

		resolution, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", false, types.RemoteOptions{}, "")
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

		resolution, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", false, types.RemoteOptions{}, "")
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

		_, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", false, types.RemoteOptions{}, "")
		require.ErrorContains(t, err, "cycle")
	})

	t.Run("variant selection picks the compatible flavor", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "variants")
		pkg := loadV1Beta1Package(t, dir)

		resolution, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "apache", false, types.RemoteOptions{}, "")
		require.NoError(t, err)

		require.Len(t, resolution.pkg.Components, 1)
		require.Equal(t, []v1beta1.Image{{Name: "httpd:2.4"}}, resolution.pkg.Components[0].Images)
	})

	t.Run("variant selection errors when no variant is compatible", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "variants")
		pkg := loadV1Beta1Package(t, dir)

		_, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", false, types.RemoteOptions{}, "")
		require.ErrorContains(t, err, "no imported component")
	})

	t.Run("single import errors when component config is incompatible", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "single-incompatible")
		pkg := loadV1Beta1Package(t, dir)

		_, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "nginx", false, types.RemoteOptions{}, "")
		require.ErrorContains(t, err, "no imported component")
	})

	t.Run("package component overrides imported component", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join("testdata", "import-v1beta1", "merge")
		pkg := loadV1Beta1Package(t, dir)

		resolution, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", false, types.RemoteOptions{}, "")
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
		_, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", false, types.RemoteOptions{}, "")
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
		_, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", false, types.RemoteOptions{}, "")
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
		_, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", false, types.RemoteOptions{}, "")
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
		_, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", false, types.RemoteOptions{}, "")
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
		_, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", false, types.RemoteOptions{}, "")
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
		_, err := resolveImportsV1Beta1(ctx, pkg, mustPackagePath(t, dir), "amd64", "", false, types.RemoteOptions{}, "")
		require.ErrorContains(t, err, "multiple")
	})
}
