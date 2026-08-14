// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package component

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sigstore/cosign/v3/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/v3/cmd/cosign/cli/verify"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/packager/assemble"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/signing"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
	"oras.land/oras-go/v2/registry"
	registryremote "oras.land/oras-go/v2/registry/remote"
)

func defaultTestRemoteOptions() types.RemoteOptions {
	return types.RemoteOptions{PlainHTTP: true}
}

func createRegistry(ctx context.Context, t *testing.T) registry.Reference {
	t.Helper()
	return registry.Reference{
		Registry:   testutil.SetupInMemoryRegistryDynamic(ctx, t),
		Repository: "my-namespace",
	}
}

func TestPublishComponentAndAssembleRemoteImportResources(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	componentDir := t.TempDir()
	for relativePath, contents := range map[string]string{
		"values.yaml":                    "enabled: true\n",
		"schema.json":                    `{"type":"object"}`,
		"chart/Chart.yaml":               "apiVersion: v2\nname: test\nversion: 0.1.0\n",
		"chart/templates/configmap.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: chart\n",
		"chart-values.yaml":              "replicas: 1\n",
		"manifest.yaml":                  "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: manifest\n",
		"kustomize/kustomization.yaml":   "resources:\n  - resource.yaml\n",
		"kustomize/resource.yaml":        "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: kustomize\n",
		"file.txt":                       "file\n",
	} {
		resourcePath := filepath.Join(componentDir, relativePath)
		require.NoError(t, os.MkdirAll(filepath.Dir(resourcePath), 0o700))
		require.NoError(t, os.WriteFile(resourcePath, []byte(contents), 0o600))
	}
	component := v1beta1.ComponentConfig{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfComponentConfig,
		Metadata:   v1beta1.ComponentMetadata{Name: "all-resources", Version: "0.0.1"},
		Values:     v1beta1.Values{Files: []string{"values.yaml"}, Schema: "schema.json"},
		Component: v1beta1.ComponentSpec{
			Charts:    []v1beta1.Chart{{Name: "test", Namespace: "default", Local: &v1beta1.LocalSource{Path: "chart"}, ValuesFiles: []v1beta1.ValuesFile{{Path: "chart-values.yaml"}}}},
			Manifests: []v1beta1.Manifest{{Name: "manifest", Files: []string{"manifest.yaml"}}, {Name: "kustomize", Kustomize: &v1beta1.KustomizeManifest{Files: []string{"kustomize"}}}},
			Files:     []v1beta1.File{{Source: "file.txt", Destination: "/tmp/file.txt"}},
		},
	}
	componentYAML, err := goyaml.Marshal(component)
	require.NoError(t, err)
	componentPath := filepath.Join(componentDir, "component.yaml")
	require.NoError(t, os.WriteFile(componentPath, componentYAML, 0o600))

	published, err := Publish(ctx, componentPath, createRegistry(ctx, t), PublishOptions{RemoteOptions: defaultTestRemoteOptions()})
	require.NoError(t, err)

	packageDir := t.TempDir()
	packageYAML := fmt.Sprintf(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: remote-component-import
components:
  - name: imported
    import:
      remote:
        - url: oci://%s
`, published.String())
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, layout.ZarfYAML), []byte(packageYAML), 0o600))
	cachePath := t.TempDir()
	loaded, err := load.Package(ctx, packageDir, load.PackageOptions{DefinitionOptions: load.DefinitionOptions{CachePath: cachePath, RemoteOptions: defaultTestRemoteOptions()}})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, loaded.Close()) })
	resourcePath, err := loaded.Resources.Path(loaded.Definition.AsV1alpha1().Components[0].Files[0].Source)
	require.NoError(t, err)

	pkgLayout, err := assemble.AssemblePackage(ctx, loaded, assemble.AssembleOptions{CachePath: cachePath, SkipSBOM: true, RemoteOptions: defaultTestRemoteOptions()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })
	require.NoFileExists(t, resourcePath)

	tarFile, err := os.Open(filepath.Join(pkgLayout.DirPath(), "components", "imported.tar"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, tarFile.Close()) })
	entries := map[string]bool{}
	// FIXME: is there a helper we can use here ?
	reader := tar.NewReader(tarFile)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		entries[header.Name] = true
	}
	for _, entry := range []string{
		"imported/charts/test.tgz",
		"imported/values/test-0",
		"imported/files/0/file.txt",
		"imported/manifests/manifest-0.yaml",
		"imported/manifests/kustomization-kustomize-0.yaml",
	} {
		require.Truef(t, entries[entry], "missing assembled resource %s", entry)
	}
}

func TestPublishComponent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	published, err := Publish(ctx, filepath.Join("testdata", "publish-component-v1beta1", "component.yaml"), createRegistry(ctx, t), PublishOptions{
		RemoteOptions: defaultTestRemoteOptions(),
	})
	require.NoError(t, err)

	component, manifest := getPublishedComponent(ctx, t, published)
	require.Equal(t, ComponentConfigMediaType, manifest.Config.MediaType)
	require.NotEmpty(t, manifest.Layers)
	require.Equal(t, []string{"resources/0/component-values.yaml"}, component.Values.Files)
	require.Equal(t, "resources/1/component-values.schema.json", component.Values.Schema)
	require.Equal(t, "resources/2/local-chart", component.Component.Charts[0].Local.Path)
	require.Equal(t, "resources/3/local-chart-values.yaml", component.Component.Charts[0].ValuesFiles[0].Path)
	require.Equal(t, "https://example.com/remote-chart-values.yaml", component.Component.Charts[0].ValuesFiles[1].Path)
	require.Equal(t, "resources/4/local-manifest.yaml", component.Component.Manifests[0].Files[0])
	require.Equal(t, "https://example.com/remote-manifest.yaml", component.Component.Manifests[1].Files[0])
	require.Equal(t, "resources/5/local-file.txt", component.Component.Files[0].Source)
	require.Equal(t, "https://example.com/remote-file.txt", component.Component.Files[1].Source)

	layerTitles := make([]string, 0, len(manifest.Layers))
	for _, layer := range manifest.Layers {
		layerTitles = append(layerTitles, layer.Annotations[ocispec.AnnotationTitle])
		require.Equal(t, layer.Annotations[ocispec.AnnotationTitle], layer.Annotations[componentResourceMountPathAnnotation])
		require.NotEmpty(t, layer.Annotations[componentResourceKindAnnotation])
	}
	for _, localPath := range []string{
		"resources/0/component-values.yaml",
		"resources/1/component-values.schema.json",
		"resources/2/local-chart/Chart.yaml",
		"resources/2/local-chart/templates/configmap.yaml",
		"resources/3/local-chart-values.yaml",
		"resources/4/local-manifest.yaml",
		"resources/5/local-file.txt",
	} {
		require.Contains(t, layerTitles, localPath)
	}
	for _, remotePath := range []string{
		"https://example.com/remote-chart-values.yaml",
		"https://example.com/remote-file.txt",
		"https://example.com/remote-manifest.yaml",
	} {
		require.NotContains(t, layerTitles, remotePath)
	}
}

func TestPublishComponentFlavor(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	published, err := Publish(ctx, filepath.Join("testdata", "publish-component-v1beta1", "component-flavor.yaml"), createRegistry(ctx, t), PublishOptions{
		Flavor:        "test",
		RemoteOptions: defaultTestRemoteOptions(),
	})
	require.NoError(t, err)
	require.Equal(t, "0.0.1-test", published.Reference)

	component, _ := getPublishedComponent(ctx, t, published)
	require.Empty(t, component.Component.Selector.Flavor)
}

func TestPublishComponentSigning(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	signOpts := signing.DefaultSignBlobOptions()
	signOpts.Key = filepath.Join("..", "packager", "testdata", "publish", "cosign.key")
	signOpts.Password = "password"
	signOpts.SkipConfirmation = true
	published, err := Publish(ctx, filepath.Join("testdata", "publish-component-v1beta1", "component-flavor.yaml"), createRegistry(ctx, t), PublishOptions{
		Flavor:          "test",
		SignBlobOptions: signOpts,
		RemoteOptions:   defaultTestRemoteOptions(),
	})
	require.NoError(t, err)

	require.NoError(t, verifyPublishedComponentSignature(ctx, published.String(), filepath.Join("..", "packager", "testdata", "publish", "cosign.pub")))
}

func TestPublishComponentRejectsNegativeRetries(t *testing.T) {
	t.Parallel()

	_, err := Publish(context.Background(), filepath.Join("testdata", "publish-component-v1beta1", "component.yaml"), createRegistry(context.Background(), t), PublishOptions{
		Retries:       -1,
		RemoteOptions: defaultTestRemoteOptions(),
	})
	require.EqualError(t, err, "component publish failed: retries cannot be negative")
}

func TestPublishComponentImageArchivesUseOCILayout(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	archivePath := filepath.Join(t.TempDir(), "images.tar")
	imageLayout := filepath.Join("..", "images", "testdata", "oras-oci-layout", "images")
	require.NoError(t, archive.Compress(ctx, []string{imageLayout}, archivePath, archive.CompressOpts{}))
	componentPath := filepath.Join(t.TempDir(), "component.yaml")
	componentYAML := []byte(fmt.Sprintf(`apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: image-archive-component
  version: 0.0.1
component:
  imageArchives:
    - path: %q
      images:
        - ghcr.io/zarf-dev/images/hello-world:latest
`, archivePath))
	require.NoError(t, os.WriteFile(componentPath, componentYAML, 0o600))

	published, err := Publish(ctx, componentPath, createRegistry(ctx, t), PublishOptions{RemoteOptions: defaultTestRemoteOptions()})
	require.NoError(t, err)

	component, manifest := getPublishedComponent(ctx, t, published)
	require.Equal(t, "images", component.Component.ImageArchives[0].Path)

	layerTitles := make([]string, 0, len(manifest.Layers))
	for _, layer := range manifest.Layers {
		layerTitles = append(layerTitles, layer.Annotations[ocispec.AnnotationTitle])
	}
	require.Contains(t, layerTitles, "images/oci-layout")
	require.Contains(t, layerTitles, "images/index.json")
	require.NotContains(t, layerTitles, "images.tar")
	require.Contains(t, layerTitles, "images/blobs/sha256/03b62250a3cb1abd125271d393fc08bf0cc713391eda6b57c02d1ef85efcc25c")
}

func TestPublishComponentResolvesLocalImports(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	childDir := filepath.Join(root, "child")
	require.NoError(t, os.Mkdir(childDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "child-file.txt"), []byte("child file"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "child-values.yaml"), []byte("child: value"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "root-values.yaml"), []byte("root: value"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(childDir, "component.yaml"), []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: imported-component
  version: 0.0.1
values:
  files:
    - child-values.yaml
component:
  files:
    - source: child-file.txt
      destination: /tmp/child-file.txt
`), 0o600))
	componentPath := filepath.Join(root, "component.yaml")
	require.NoError(t, os.WriteFile(componentPath, []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: importing-component
  version: 0.0.1
values:
  files:
    - root-values.yaml
component:
  import:
    local:
      - path: child/component.yaml
`), 0o600))

	published, err := Publish(ctx, componentPath, createRegistry(ctx, t), PublishOptions{RemoteOptions: defaultTestRemoteOptions()})
	require.NoError(t, err)

	component, manifest := getPublishedComponent(ctx, t, published)
	require.Empty(t, component.Component.Import)
	require.Equal(t, []string{"resources/0/child-values.yaml", "resources/1/root-values.yaml"}, component.Values.Files)
	require.Equal(t, []v1beta1.File{{Source: "resources/2/child-file.txt", Destination: "/tmp/child-file.txt"}}, component.Component.Files)

	layerTitles := make([]string, 0, len(manifest.Layers))
	for _, layer := range manifest.Layers {
		layerTitles = append(layerTitles, layer.Annotations[ocispec.AnnotationTitle])
	}
	require.Contains(t, layerTitles, "resources/0/child-values.yaml")
	require.Contains(t, layerTitles, "resources/1/root-values.yaml")
	require.Contains(t, layerTitles, "resources/2/child-file.txt")
	require.NotContains(t, layerTitles, "child/component.yaml")
}

func TestPublishComponentSelectsImportsForComponentArchitecture(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	for architecture := range map[string]struct{}{"amd64": {}, "arm64": {}} {
		componentYAML := []byte(fmt.Sprintf(`apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: %s-component
  version: 0.0.1
component:
  selector:
    architecture: %s
  images:
    - name: example.com/%s:latest
`, architecture, architecture, architecture))
		require.NoError(t, os.WriteFile(filepath.Join(root, architecture+".yaml"), componentYAML, 0o600))
	}
	componentPath := filepath.Join(root, "component.yaml")
	require.NoError(t, os.WriteFile(componentPath, []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: architecture-component
  version: 0.0.1
component:
  selector:
    architecture: arm64
  import:
    local:
      - path: amd64.yaml
      - path: arm64.yaml
`), 0o600))

	published, err := Publish(ctx, componentPath, createRegistry(ctx, t), PublishOptions{RemoteOptions: defaultTestRemoteOptions()})
	require.NoError(t, err)

	component, _ := getPublishedComponent(ctx, t, published)
	require.Equal(t, "arm64", component.Component.Selector.Architecture)
	require.Equal(t, []v1beta1.Image{{Name: "example.com/arm64:latest"}}, component.Component.Images)
}

func TestPublishComponentNormalizesExternalResources(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	componentDir := filepath.Join(root, "component")
	externalDir := filepath.Join(root, "external")
	require.NoError(t, os.MkdirAll(componentDir, 0o700))
	for relativePath := range map[string]string{
		"values.yaml":                  "values: true",
		"schema.json":                  `{}`,
		"chart/Chart.yaml":             "apiVersion: v2\nname: test\nversion: 0.1.0",
		"chart-values.yaml":            "chart: true",
		"manifest.yaml":                "apiVersion: v1\nkind: ConfigMap",
		"kustomize/kustomization.yaml": "resources: []",
		"file.txt":                     "file",
	} {
		path := filepath.Join(externalDir, relativePath)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte("resource"), 0o600))
	}
	component := v1beta1.ComponentConfig{
		APIVersion: "zarf.dev/v1beta1",
		Kind:       "ZarfComponentConfig",
		Metadata:   v1beta1.ComponentMetadata{Name: "external-resources", Version: "0.0.1"},
		Values:     v1beta1.Values{Files: []string{"../external/values.yaml"}, Schema: filepath.Join(externalDir, "schema.json")},
		Component: v1beta1.ComponentSpec{
			Charts: []v1beta1.Chart{{
				Local:       &v1beta1.LocalSource{Path: filepath.Join(externalDir, "chart")},
				ValuesFiles: []v1beta1.ValuesFile{{Path: "../external/chart-values.yaml"}},
			}},
			Manifests: []v1beta1.Manifest{{
				Files:     []string{filepath.Join(externalDir, "manifest.yaml")},
				Kustomize: &v1beta1.KustomizeManifest{Files: []string{"../external/kustomize"}},
			}},
			Files: []v1beta1.File{{Source: filepath.Join(externalDir, "file.txt"), Destination: "/tmp/file.txt"}},
		},
	}
	componentYAML, err := goyaml.Marshal(component)
	require.NoError(t, err)
	componentPath := filepath.Join(componentDir, "component.yaml")
	require.NoError(t, os.WriteFile(componentPath, componentYAML, 0o600))

	published, err := Publish(ctx, componentPath, createRegistry(ctx, t), PublishOptions{RemoteOptions: defaultTestRemoteOptions()})
	require.NoError(t, err)

	publishedComponent, manifest := getPublishedComponent(ctx, t, published)
	require.Equal(t, "resources/0/values.yaml", publishedComponent.Values.Files[0])
	require.Equal(t, "resources/1/schema.json", publishedComponent.Values.Schema)
	require.Equal(t, "resources/2/chart", publishedComponent.Component.Charts[0].Local.Path)
	require.Equal(t, "resources/3/chart-values.yaml", publishedComponent.Component.Charts[0].ValuesFiles[0].Path)
	require.Equal(t, "resources/4/manifest.yaml", publishedComponent.Component.Manifests[0].Files[0])
	require.Equal(t, "resources/5/kustomize", publishedComponent.Component.Manifests[0].Kustomize.Files[0])
	require.Equal(t, "resources/6/file.txt", publishedComponent.Component.Files[0].Source)

	layerMounts := make([]string, 0, len(manifest.Layers))
	for _, layer := range manifest.Layers {
		layerMounts = append(layerMounts, layer.Annotations[componentResourceMountPathAnnotation])
	}
	for _, mountPath := range []string{
		"resources/0/values.yaml",
		"resources/1/schema.json",
		"resources/2/chart/Chart.yaml",
		"resources/3/chart-values.yaml",
		"resources/4/manifest.yaml",
		"resources/5/kustomize/kustomization.yaml",
		"resources/6/file.txt",
	} {
		require.Contains(t, layerMounts, mountPath)
	}
}

func TestPublishComponentRejectsOnCreateActions(t *testing.T) {
	t.Parallel()

	componentPath := filepath.Join(t.TempDir(), "component.yaml")
	componentYAML := []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: on-create-component
  version: 0.0.1
component:
  actions:
    onCreate:
      before:
        - cmd: echo prepare
`)
	require.NoError(t, os.WriteFile(componentPath, componentYAML, 0o600))

	_, err := Publish(context.Background(), componentPath, createRegistry(context.Background(), t), PublishOptions{RemoteOptions: defaultTestRemoteOptions()})
	require.EqualError(t, err, "onCreate actions are not supported for published remote components")
}

func getPublishedComponent(ctx context.Context, t *testing.T, published registry.Reference) (v1beta1.ComponentConfig, ocispec.Manifest) {
	t.Helper()

	repo, err := registryremote.NewRepository(published.Registry + "/" + published.Repository)
	require.NoError(t, err)
	repo.PlainHTTP = true
	descriptor, err := repo.Resolve(ctx, published.Reference)
	require.NoError(t, err)
	require.Equal(t, ocispec.MediaTypeImageManifest, descriptor.MediaType)
	require.NotZero(t, descriptor.Size)

	manifestReader, err := repo.Manifests().Fetch(ctx, descriptor)
	require.NoError(t, err)
	manifestBytes, err := io.ReadAll(manifestReader)
	require.NoError(t, err)
	require.NoError(t, manifestReader.Close())
	var manifest ocispec.Manifest
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))

	componentReader, err := repo.Blobs().Fetch(ctx, manifest.Config)
	require.NoError(t, err)
	componentBytes, err := io.ReadAll(componentReader)
	require.NoError(t, err)
	require.NoError(t, componentReader.Close())
	var component v1beta1.ComponentConfig
	require.NoError(t, json.Unmarshal(componentBytes, &component))
	return component, manifest
}

func verifyPublishedComponentSignature(ctx context.Context, componentRef, publicKeyPath string) error {
	cmd := &verify.VerifyCommand{
		RegistryOptions: options.RegistryOptions{AllowHTTPRegistry: true},
		CommonVerifyOptions: options.CommonVerifyOptions{
			IgnoreTlog:      true,
			NewBundleFormat: true,
		},
		CheckClaims:     true,
		KeyRef:          publicKeyPath,
		IgnoreTlog:      true,
		NewBundleFormat: true,
	}
	return cmd.Exec(ctx, []string{componentRef})
}

func TestComponentResourcesRejectUnsupportedRemoteSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		component v1beta1.ComponentConfig
		wantErr   string
	}{
		{
			name: "values file",
			component: v1beta1.ComponentConfig{
				Values: v1beta1.Values{Files: []string{"https://example.com/values.yaml"}},
			},
			wantErr: "resource kind values-file cannot be pulled from remote sources",
		},
		{
			name: "values schema",
			component: v1beta1.ComponentConfig{
				Values: v1beta1.Values{Schema: "https://example.com/values.schema.json"},
			},
			wantErr: "resource kind values-schema cannot be pulled from remote sources",
		},
		{
			name: "local chart",
			component: v1beta1.ComponentConfig{
				Component: v1beta1.ComponentSpec{Charts: []v1beta1.Chart{{Local: &v1beta1.LocalSource{Path: "https://example.com/chart.tgz"}}}},
			},
			wantErr: "resource kind chart cannot be pulled from remote sources",
		},
		{
			name: "component import",
			component: v1beta1.ComponentConfig{
				Component: v1beta1.ComponentSpec{Import: v1beta1.ComponentImport{Remote: []v1beta1.ComponentImportRemote{{URL: "oci://example.com/components/foo:1.0.0"}}}},
			},
			wantErr: "remote component imports are not yet supported for v1beta1 packages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := normalizeComponentResources(filepath.Join(t.TempDir(), "component.yaml"), tt.component)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestComponentResourcesAllowsSupportedRemoteSources(t *testing.T) {
	t.Parallel()

	component := v1beta1.ComponentConfig{
		Component: v1beta1.ComponentSpec{
			Charts: []v1beta1.Chart{{ValuesFiles: []v1beta1.ValuesFile{{Path: "https://example.com/chart-values.yaml"}}}},
			Manifests: []v1beta1.Manifest{{
				Files:     []string{"https://example.com/manifest.yaml"},
				Kustomize: &v1beta1.KustomizeManifest{Files: []string{"https://example.com/kustomization.yaml"}},
			}},
			Files: []v1beta1.File{{Source: "https://example.com/file.txt"}},
		},
	}
	_, resources, err := normalizeComponentResources(filepath.Join(t.TempDir(), "component.yaml"), component)
	require.NoError(t, err)
	require.Empty(t, resources.resources)
}
