// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config"
)

func TestDevTemplateRendersValuesAndCLIVersion(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, packageTemplateFilename), []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: app
  version: [[ .cli.version ]]
  description: [[ .environment ]]
components:
  - name: app
    images:
      - name: [[ .image ]]
`), 0o644))
	valuesPath := filepath.Join(dir, "values.yaml")
	require.NoError(t, os.WriteFile(valuesPath, []byte("environment: development\nimage: registry.example/app:from-file\n"), 0o644))

	o := devTemplateOptions{
		set:     map[string]string{"image": "registry.example/app:from-set"},
		setFile: valuesPath,
	}
	require.NoError(t, o.run(context.Background(), []string{dir}))

	generated, err := os.ReadFile(filepath.Join(dir, "zarf.gen.yaml"))
	require.NoError(t, err)
	require.Equal(t, `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: app
  version: `+config.CLIVersion+`
  description: development
components:
  - name: app
    images:
      - name: registry.example/app:from-set
`, string(generated))
}

func TestDevTemplateFollowsAndRewritesLocalTemplateImports(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"zarf.tpl.yaml": `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: app
components:
  - name: app
    import:
      local:
        - path: components/app.tpl.yaml
`,
		"components/app.tpl.yaml": `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: app
component:
  import:
    local:
      - path: nested.tpl.yaml
`,
		"components/nested.tpl.yaml": `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: nested
component:
  images:
    - name: [[ .image ]]
`,
	}
	for path, contents := range files {
		path = filepath.Join(dir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	}

	o := devTemplateOptions{set: map[string]string{"image": "registry.example/nested:v1"}}
	require.NoError(t, o.run(context.Background(), []string{dir}))

	root, err := os.ReadFile(filepath.Join(dir, "zarf.gen.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(root), "path: components/app.gen.yaml")
	child, err := os.ReadFile(filepath.Join(dir, "components", "app.gen.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(child), "path: nested.gen.yaml")
	grandchild, err := os.ReadFile(filepath.Join(dir, "components", "nested.gen.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(grandchild), "name: registry.example/nested:v1")
}

func TestDevTemplateTraversesImportsGeneratedByBlockTemplate(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"zarf.tpl.yaml": `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: app
components:
[[ block "components" . ]]
  - name: app
    import:
      local:
        - path: components/[[ .component ]].tpl.yaml
[[ end ]]
`,
		"components/app.tpl.yaml": `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: app
component:
  images:
    - name: [[ .image ]]
`,
	}
	for path, contents := range files {
		path = filepath.Join(dir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	}

	o := devTemplateOptions{set: map[string]string{
		"component": "app",
		"image":     "registry.example/nested:v1",
	}}
	require.NoError(t, o.run(context.Background(), []string{dir}))

	root, err := os.ReadFile(filepath.Join(dir, "zarf.gen.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(root), "path: components/app.gen.yaml")
	require.NotContains(t, string(root), "[[")

	component, err := os.ReadFile(filepath.Join(dir, "components", "app.gen.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(component), "name: registry.example/nested:v1")
}

func TestDevTemplateRequiresAllValues(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, packageTemplateFilename)
	require.NoError(t, os.WriteFile(source, []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: [[ .missing ]]
components:
  - name: app
`), 0o644))

	err := (&devTemplateOptions{}).run(context.Background(), []string{source})
	require.ErrorContains(t, err, "map has no entry for key \"missing\"")
	_, statErr := os.Stat(filepath.Join(dir, "zarf.gen.yaml"))
	require.ErrorIs(t, statErr, os.ErrNotExist)
}
