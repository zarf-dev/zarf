// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestComponentPublishRemoteImport(t *testing.T) {
	componentDir := t.TempDir()
	writeComponentPublishResources(t, componentDir)
	imageArchive := filepath.Join(componentDir, "images.tar")
	stdOut, stdErr, err := e2e.Zarf(t, "tools", "archiver", "compress", "src/pkg/images/testdata/oras-oci-layout/images", imageArchive)
	require.NoError(t, err, stdOut, stdErr)

	remoteResources := newComponentResourceServer(t)
	defer remoteResources.Close()

	// FIXME: should be an on disk package, real images, and real files
	componentConfig := fmt.Sprintf(`apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: published-component
  version: 0.0.1
values:
  files:
    - component-values.yaml
  schema: component-values.schema.json
component:
  charts:
    - name: local-chart
      namespace: default
      local:
        path: local-chart
      valuesFiles:
        - path: local-chart-values.yaml
        - path: %q
    - name: remote-chart
      namespace: default
      helmRepository:
        name: remote-chart
        url: %q
        version: 0.1.0
  manifests:
    - name: local-manifest
      namespace: default
      files:
        - local-manifest.yaml
    - name: remote-manifest
      namespace: default
      files:
        - %q
    - name: local-kustomization
      namespace: default
      kustomize:
        files:
          - kustomization
  files:
    - source: local-file.txt
      destination: /tmp/local-file.txt
    - source: %q
      destination: /tmp/remote-file.txt
  imageArchives:
    - path: %q
      images:
        - ghcr.io/zarf-dev/images/hello-world:latest
`, remoteResources.URL+"/remote-chart-values.yaml", remoteResources.URL+"/charts", remoteResources.URL+"/remote-manifest.yaml", remoteResources.URL+"/remote-file.txt", imageArchive)
	componentPath := filepath.Join(componentDir, "component.yaml")
	require.NoError(t, os.WriteFile(componentPath, []byte(componentConfig), 0o600))

	registryURL := testutil.SetupInMemoryRegistryDynamic(testutil.TestContext(t), t)
	stdOut, stdErr, err = e2e.Zarf(t, "component", "publish", componentPath, "oci://"+registryURL, "--plain-http", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	packageDir := t.TempDir()
	packageConfig := fmt.Sprintf(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: component-remote-import
components:
  - name: imported-component
    import:
      remote:
        - url: oci://%s/published-component:0.0.1
`, registryURL)
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, layout.ZarfYAML), []byte(packageConfig), 0o600))

	packageOutput := t.TempDir()
	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", packageDir, "-o", packageOutput, "--plain-http", "--skip-sbom", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	packagePath := filepath.Join(packageOutput, fmt.Sprintf("zarf-package-component-remote-import-%s.tar.zst", e2e.Arch))
	pkgLayout, err := layout.LoadFromTar(t.Context(), packagePath, layout.PackageLayoutOptions{})
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(pkgLayout.GetImageDirPath(), "index.json"))

	componentExtractDir := t.TempDir()
	chartsDir, err := pkgLayout.GetComponentDir(t.Context(), componentExtractDir, "imported-component", layout.ChartsComponentDir)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(chartsDir, "local-chart.tgz"))
	require.FileExists(t, filepath.Join(chartsDir, "remote-chart-0.1.0.tgz"))

	valuesDir, err := pkgLayout.GetComponentDir(t.Context(), componentExtractDir, "imported-component", layout.ValuesComponentDir)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(valuesDir, "local-chart-0"))
	require.FileExists(t, filepath.Join(valuesDir, "local-chart-1"))

	filesDir, err := pkgLayout.GetComponentDir(t.Context(), componentExtractDir, "imported-component", layout.FilesComponentDir)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(filesDir, "0", "local-file.txt"))
	require.FileExists(t, filepath.Join(filesDir, "1", "remote-file.txt"))

	manifestsDir, err := pkgLayout.GetComponentDir(t.Context(), componentExtractDir, "imported-component", layout.ManifestsComponentDir)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(manifestsDir, "local-manifest-0.yaml"))
	require.FileExists(t, filepath.Join(manifestsDir, "remote-manifest-0.yaml"))
	require.FileExists(t, filepath.Join(manifestsDir, "kustomization-local-kustomization-0.yaml"))
}

func writeComponentPublishResources(t *testing.T, dir string) {
	t.Helper()
	resources := map[string]string{
		"component-values.yaml": `component:
  enabled: true
`,
		"component-values.schema.json": `{"type":"object"}`,
		"local-chart-values.yaml":      "replicaCount: 1\n",
		"local-file.txt":               "local file\n",
		"local-manifest.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: local-manifest
`,
		"local-chart/Chart.yaml": `apiVersion: v2
name: local-chart
version: 0.1.0
`,
		"local-chart/templates/configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: local-chart
`,
		"kustomization/kustomization.yaml": "resources:\n  - configmap.yaml\n",
		"kustomization/configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: local-kustomization
`,
	}
	for path, contents := range resources {
		resourcePath := filepath.Join(dir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(resourcePath), 0o700))
		require.NoError(t, os.WriteFile(resourcePath, []byte(contents), 0o600))
	}
}

func newComponentResourceServer(t *testing.T) *httptest.Server {
	t.Helper()
	remoteChart := helmChartArchive(t, "remote-chart")
	resources := map[string][]byte{
		"/charts/index.yaml": []byte(`apiVersion: v1
entries:
  remote-chart:
    - apiVersion: v2
      name: remote-chart
      version: 0.1.0
      urls:
        - remote-chart-0.1.0.tgz
`),
		"/charts/remote-chart-0.1.0.tgz": remoteChart,
		"/remote-chart-values.yaml":      []byte("replicaCount: 2\n"),
		"/remote-file.txt":               []byte("remote file\n"),
		"/remote-manifest.yaml": []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: remote-manifest
`),
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resource, ok := resources[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(resource)
	}))
}

func helmChartArchive(t *testing.T, name string) []byte {
	t.Helper()
	var archive bytes.Buffer
	gzipWriter := gzip.NewWriter(&archive)
	tarWriter := tar.NewWriter(gzipWriter)
	for path, contents := range map[string]string{
		name + "/Chart.yaml": fmt.Sprintf("apiVersion: v2\nname: %s\nversion: 0.1.0\n", name),
		name + "/templates/configmap.yaml": fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
`, name),
	} {
		header := &tar.Header{Name: path, Mode: 0o600, Size: int64(len(contents))}
		require.NoError(t, tarWriter.WriteHeader(header))
		_, err := tarWriter.Write([]byte(contents))
		require.NoError(t, err)
	}
	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())
	return archive.Bytes()
}
