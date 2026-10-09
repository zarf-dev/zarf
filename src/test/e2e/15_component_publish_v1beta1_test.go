// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
)

func TestComponentPublish(t *testing.T) {
	componentPath := filepath.Join("src", "test", "packages", "15-component-publish-v1beta1", "component.yaml")

	registryURL := testutil.SetupInMemoryRegistryDynamic(testutil.TestContext(t), t)
	privateKey := filepath.Join("src", "test", "packages", "zarf-test.prv-key")
	publicKey := filepath.Join("src", "test", "packages", "zarf-test.pub")
	stdOut, stdErr, err := e2e.Zarf(t, "component", "publish", componentPath, "oci://"+registryURL, "--plain-http", "--signing-key", privateKey)
	require.NoError(t, err, stdOut, stdErr)

	componentSource := registryURL + "/published-component:0.0.1"
	t.Run("published resource archive", func(t *testing.T) {
		remote, err := zoci.NewRemoteWithOptions(t.Context(), componentSource, ocispec.Platform{}, zoci.RemoteClientOptions{
			RemoteOptions: types.RemoteOptions{PlainHTTP: true},
		})
		require.NoError(t, err)
		manifest, err := remote.FetchRoot(t.Context())
		require.NoError(t, err)
		require.Equal(t, layout.ZarfComponentConfigMediaType, manifest.Config.MediaType)
		require.Len(t, manifest.Layers, 1)
		require.Equal(t, layout.ZarfComponentLayerMediaTypeTar, manifest.Layers[0].MediaType)
		require.Equal(t, layout.ComponentTar, manifest.Layers[0].Annotations[ocispec.AnnotationTitle])
	})
	t.Run("verify published signature", func(t *testing.T) {
		stdOut, stdErr, err := e2e.Zarf(t, "component", "verify", componentSource, "--plain-http", "--key", publicKey)
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdErr, "component signature verification")
		require.Contains(t, stdErr, "PASSED")
	})

	t.Run("re-sign published component", func(t *testing.T) {
		stdOut, stdErr, err := e2e.Zarf(t, "component", "sign", componentSource, "--plain-http", "--signing-key", privateKey, "--confirm")
		require.NoError(t, err, stdOut, stdErr)

		stdOut, stdErr, err = e2e.Zarf(t, "component", "verify", componentSource, "--plain-http", "--key", publicKey)
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdErr, "component signature verification")
		require.Contains(t, stdErr, "PASSED")
	})

	t.Run("remote import content", func(t *testing.T) {
		packageDir := t.TempDir()
		packageTemplatePath := filepath.Join("src", "test", "packages", "15-component-publish-v1beta1", "zarf.tpl.yaml")
		packageTemplate, err := os.ReadFile(packageTemplatePath)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(packageDir, "zarf.tpl.yaml"), packageTemplate, 0o600))
		stdOut, stdErr, err := e2e.ZarfInDir(t, packageDir, "dev", "template", "--set", "registryURL="+registryURL)
		require.NoError(t, err, stdOut, stdErr)

		packageOutput := t.TempDir()
		stdOut, stdErr, err = e2e.Zarf(t, "package", "create", packageDir, "-o", packageOutput, "--plain-http", "--skip-sbom", "--confirm")
		require.NoError(t, err, stdOut, stdErr)

		packagePath := filepath.Join(packageOutput, fmt.Sprintf("zarf-package-component-remote-import-%s.tar.zst", e2e.Arch))
		pkgLayout, err := layout.LoadFromTar(t.Context(), packagePath, layout.PackageLayoutOptions{})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })
		require.FileExists(t, filepath.Join(pkgLayout.GetImageDirPath(), "index.json"))

		componentExtractDir := t.TempDir()
		chartsDir, err := pkgLayout.GetComponentDir(t.Context(), componentExtractDir, "imported-component", layout.ChartsComponentDir)
		require.NoError(t, err)
		require.FileExists(t, filepath.Join(chartsDir, "local-chart.tgz"))

		valuesDir, err := pkgLayout.GetComponentDir(t.Context(), componentExtractDir, "imported-component", layout.ValuesComponentDir)
		require.NoError(t, err)
		require.FileExists(t, filepath.Join(valuesDir, "local-chart-0"))

		filesDir, err := pkgLayout.GetComponentDir(t.Context(), componentExtractDir, "imported-component", layout.FilesComponentDir)
		require.NoError(t, err)
		require.FileExists(t, filepath.Join(filesDir, "0", "local-file.txt"))

		manifestsDir, err := pkgLayout.GetComponentDir(t.Context(), componentExtractDir, "imported-component", layout.ManifestsComponentDir)
		require.NoError(t, err)
		require.FileExists(t, filepath.Join(manifestsDir, "local-manifest-0.yaml"))
		require.FileExists(t, filepath.Join(manifestsDir, "kustomization-local-kustomization-0.yaml"))
	})
	t.Run("image archive layers", func(t *testing.T) {
		componentDir := t.TempDir()
		imageArchive := filepath.Join(componentDir, "images.tar")
		imageLayout := filepath.Join("src", "pkg", "images", "testdata", "oras-oci-layout", "images")
		require.NoError(t, archive.Compress(t.Context(), []string{imageLayout}, imageArchive, archive.CompressOpts{}))
		componentPath := filepath.Join(componentDir, "component.yaml")
		require.NoError(t, os.WriteFile(componentPath, []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: published-image-archive
  version: 0.0.1
component:
  imageArchives:
    - path: images.tar
      images:
        - ghcr.io/zarf-dev/images/hello-world:latest
`), 0o600))
		stdOut, stdErr, err := e2e.Zarf(t, "component", "publish", componentPath, "oci://"+registryURL, "--plain-http")
		require.NoError(t, err, stdOut, stdErr)
		source := registryURL + "/published-image-archive:0.0.1"
		remote, err := zoci.NewRemoteWithOptions(t.Context(), source, ocispec.Platform{}, zoci.RemoteClientOptions{RemoteOptions: types.RemoteOptions{PlainHTTP: true}})
		require.NoError(t, err)
		manifest, err := remote.FetchRoot(t.Context())
		require.NoError(t, err)
		layerNames := make([]string, 0, len(manifest.Layers))
		for _, layer := range manifest.Layers {
			layerNames = append(layerNames, layer.Annotations[ocispec.AnnotationTitle])
		}
		require.NotContains(t, layerNames, layout.ComponentTar)
		require.Contains(t, layerNames, "images/index.json")
		require.Contains(t, layerNames, "images/oci-layout")
		imageManifest := "blobs/sha256/03b62250a3cb1abd125271d393fc08bf0cc713391eda6b57c02d1ef85efcc25c"
		require.Contains(t, layerNames, "images/"+imageManifest)

		packageDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(packageDir, layout.ZarfYAML), []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: image-archive-import
components:
  - name: imported
    import:
      remote:
        - url: oci://`+source+"\n"), 0o600))
		output := t.TempDir()
		stdOut, stdErr, err = e2e.Zarf(t, "package", "create", packageDir, "-o", output, "--architecture", "amd64", "--plain-http", "--skip-sbom", "--confirm")
		require.NoError(t, err, stdOut, stdErr)
		pkgLayout, err := layout.LoadFromTar(t.Context(), filepath.Join(output, "zarf-package-image-archive-import-amd64.tar.zst"), layout.PackageLayoutOptions{})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })
		original, err := os.ReadFile(filepath.Join(imageLayout, filepath.FromSlash(imageManifest)))
		require.NoError(t, err)
		assembled, err := os.ReadFile(filepath.Join(pkgLayout.GetImageDirPath(), filepath.FromSlash(imageManifest)))
		require.NoError(t, err)
		require.Equal(t, original, assembled)
	})
}
