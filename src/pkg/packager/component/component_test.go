// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package component

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
	"oras.land/oras-go/v2/registry"
)

func TestPublishPullRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	regAddr := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	dir := t.TempDir()
	writeFile(t, dir, "data.txt", "payload")
	componentPath := writeComponent(t, dir, `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: web
  version: 1.2.3
component:
  files:
    - source: data.txt
      destination: /tmp/data.txt
`)
	dst := mustParseRef(t, regAddr+"/components")

	published, err := Publish(ctx, componentPath, dst, PublishOptions{RemoteOptions: types.RemoteOptions{PlainHTTP: true}})
	require.NoError(t, err)
	require.Equal(t, regAddr+"/components/web:1.2.3", published.String())

	pulledPath, cleanup, err := Pull(ctx, published, PullOptions{RemoteOptions: types.RemoteOptions{PlainHTTP: true}})
	require.NoError(t, err)
	defer func() { require.NoError(t, cleanup()) }()
	require.Equal(t, componentConfigFileName, filepath.Base(pulledPath))
	require.FileExists(t, filepath.Join(filepath.Dir(pulledPath), "data.txt"))

	var pulled v1beta1.ComponentConfig
	b, err := os.ReadFile(pulledPath)
	require.NoError(t, err)
	require.NoError(t, goyaml.Unmarshal(b, &pulled))
	require.Equal(t, "web", pulled.Metadata.Name)
	require.Equal(t, config.CLIVersion, pulled.PublishData.ZarfVersion)
}

func TestPublishRequiresTagOrVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	componentPath := writeComponent(t, dir, `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: web
component: {}
`)
	dst := mustParseRef(t, "registry.example.test/components")

	_, err := Publish(ctx, componentPath, dst, PublishOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "metadata.version or publish tag")
}

func TestPublishRejectsFlavorMismatch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	componentPath := writeComponent(t, dir, `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: web
  version: 1.0.0
component:
  selector:
    flavor: prod
`)
	dst := mustParseRef(t, "registry.example.test/components")

	_, err := Publish(ctx, componentPath, dst, PublishOptions{Flavor: "dev"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires flavor")
}

func TestPublishRejectsWrongKind(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	componentPath := writeComponent(t, dir, `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: web
  version: 1.0.0
component: {}
`)
	dst := mustParseRef(t, "registry.example.test/components")

	_, err := Publish(ctx, componentPath, dst, PublishOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "schema validation")
}

func TestPublishRejectsInvalidImportsAndDirectories(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dst := mustParseRef(t, "registry.example.test/components")

	t.Run("directory input", func(t *testing.T) {
		t.Parallel()
		_, err := Publish(ctx, t.TempDir(), dst, PublishOptions{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "is a directory")
	})

	t.Run("empty local import path", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		componentPath := writeComponent(t, dir, `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: web
  version: 1.0.0
component:
  import:
    local:
      - path: ""
`)
		_, err := Publish(ctx, componentPath, dst, PublishOptions{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing a path")
	})

	t.Run("absolute local import path", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		componentPath := writeComponent(t, dir, `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: web
  version: 1.0.0
component:
  import:
    local:
      - path: /tmp/component.yaml
`)
		_, err := Publish(ctx, componentPath, dst, PublishOptions{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot be absolute")
	})

	t.Run("remote import without tag or digest", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		componentPath := writeComponent(t, dir, `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: web
  version: 1.0.0
component:
  import:
    remote:
      - url: oci://registry.example.test/components/base
`)
		_, err := Publish(ctx, componentPath, dst, PublishOptions{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "must include a tag or digest")
	})
}

func TestPublishPullIncludesLocalResources(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	regAddr := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	dir := t.TempDir()
	writeFile(t, dir, "files/app.txt", "file")
	writeFile(t, dir, "archives/images.tar", "tar")
	writeFile(t, dir, "charts/app/Chart.yaml", "apiVersion: v2\nname: app\nversion: 0.1.0\n")
	writeFile(t, dir, "charts/app/templates/deploy.yaml", "kind: Deployment\n")
	writeFile(t, dir, "values/chart-values.yaml", "replicas: 1\n")
	writeFile(t, dir, "manifests/deploy.yaml", "kind: ConfigMap\n")
	writeFile(t, dir, "kustomize/kustomization.yaml", "resources: []\n")
	writeFile(t, dir, "values/global.yaml", "name: app\n")
	writeFile(t, dir, "values/schema.json", `{"type":"object"}`)
	writeFile(t, dir, "unreferenced.txt", "do not publish")
	componentPath := writeComponent(t, dir, `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: resources
  version: 2.0.0
values:
  files:
    - values/global.yaml
  schema: values/schema.json
component:
  files:
    - source: files/app.txt
      destination: /tmp/app.txt
  imageArchives:
    - path: archives/images.tar
      images:
        - example.com/app:1.0.0
  charts:
    - name: app
      namespace: app
      local:
        path: charts/app
      valuesFiles:
        - path: values/chart-values.yaml
  manifests:
    - name: app
      files:
        - manifests/deploy.yaml
      kustomize:
        files:
          - kustomize
`)
	dst := mustParseRef(t, regAddr+"/components")
	published, err := Publish(ctx, componentPath, dst, PublishOptions{RemoteOptions: types.RemoteOptions{PlainHTTP: true}})
	require.NoError(t, err)

	pulledPath, cleanup, err := Pull(ctx, published, PullOptions{RemoteOptions: types.RemoteOptions{PlainHTTP: true}})
	require.NoError(t, err)
	defer func() { require.NoError(t, cleanup()) }()
	pulledDir := filepath.Dir(pulledPath)

	for _, rel := range []string{
		"files/app.txt",
		"archives/images.tar",
		"charts/app/Chart.yaml",
		"charts/app/templates/deploy.yaml",
		"values/chart-values.yaml",
		"manifests/deploy.yaml",
		"kustomize/kustomization.yaml",
		"values/global.yaml",
		"values/schema.json",
	} {
		require.FileExists(t, filepath.Join(pulledDir, rel), rel)
	}
	require.NoFileExists(t, filepath.Join(pulledDir, "unreferenced.txt"))
}

func writeComponent(t *testing.T, dir, content string) string {
	t.Helper()
	return writeFile(t, dir, "component-source.yaml", content)
}

func writeFile(t *testing.T, dir, rel, content string) string {
	t.Helper()
	p := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}

func mustParseRef(t *testing.T, raw string) registry.Reference {
	t.Helper()
	ref, err := registry.ParseReference(raw)
	require.NoError(t, err)
	return ref
}
