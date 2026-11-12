// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
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

func TestDevTemplateRecursive(t *testing.T) {
	t.Parallel()
	for _, recursive := range []bool{false, true} {
		t.Run(fmt.Sprint(recursive), func(t *testing.T) {
			t.Parallel()
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
        - path: child/[[ .component ]].gen.yaml
  - name: shared
    import:
      local:
        - path: leaf.gen.yaml
  - name: remote
    import:
      remote:
        - url: oci://registry.invalid/component:1.0.0
`,
				"child/app.tpl.yaml": `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: child
  version: "[[ .cli.version ]]"
component:
  import:
    local:
      - path: ../static.yaml
`,
				"static.yaml": `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: static
component:
  import:
    local:
      - path: leaf.gen.yaml
`,
				"leaf.tpl.yaml": `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: leaf
  description: [[ .environment ]]
component:
  images:
    - name: [[ .image ]]
`,
				"leaf.gen.yaml":   "stale generated content\n",
				"unused.tpl.yaml": "[[ .missing ]]",
				"values.yaml":     "component: app\nenvironment: development\nimage: registry.example/app:from-file\n",
			}
			for name, contents := range files {
				path := filepath.Join(dir, name)
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
			}
			v := viper.New()
			cmd := newDevTemplateCommand(v)
			args := []string{dir, "--set-file", filepath.Join(dir, "values.yaml"), "--set", "image=registry.example/app:from-set"}
			if recursive {
				args = append(args, "--recursive")
			}
			cmd.SetArgs(args)
			require.NoError(t, cmd.ExecuteContext(t.Context()))
			require.FileExists(t, filepath.Join(dir, "zarf.gen.yaml"))
			leaf, err := os.ReadFile(filepath.Join(dir, "leaf.gen.yaml"))
			require.NoError(t, err)
			if recursive {
				require.Contains(t, string(leaf), "description: development")
				require.Contains(t, string(leaf), "name: registry.example/app:from-set")
				child, err := os.ReadFile(filepath.Join(dir, "child/app.gen.yaml"))
				require.NoError(t, err)
				require.Contains(t, string(child), config.CLIVersion)
			} else {
				require.Equal(t, files["leaf.gen.yaml"], string(leaf))
				require.NoFileExists(t, filepath.Join(dir, "child/app.gen.yaml"))
			}
			static, err := os.ReadFile(filepath.Join(dir, "static.yaml"))
			require.NoError(t, err)
			require.Equal(t, files["static.yaml"], string(static))
			require.NoFileExists(t, filepath.Join(dir, "unused.gen.yaml"))
		})
	}
}

func TestDevTemplateRecursiveErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		child          string
		skipValidation bool
		wantError      string
	}{
		{name: "missing source", wantError: "child.tpl.yaml"},
		{name: "missing value", child: "[[ .missing ]]", wantError: "map has no entry"},
		{name: "invalid api", child: "apiVersion: zarf.dev/v1alpha1", wantError: "must use apiVersion"},
		{name: "invalid schema", child: "apiVersion: zarf.dev/v1beta1\nkind: ZarfComponentConfig", wantError: "linting error"},
		{name: "skip schema", child: "apiVersion: zarf.dev/v1beta1\nkind: ZarfComponentConfig", skipValidation: true},
		{name: "cycle", child: `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: child
component:
  import:
    local:
      - path: ./child.gen.yaml
`, wantError: "imported in cycle"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "zarf.tpl.yaml"), []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfInitConfig
metadata:
  name: init
components:
  - name: child
    import:
      local:
        - path: child.gen.yaml
`), 0o644))
			if tc.child != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "child.tpl.yaml"), []byte(tc.child), 0o644))
			}
			o := devTemplateOptions{recursive: true, skipValidation: tc.skipValidation}
			err := o.run(t.Context(), []string{dir})
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
				require.NoFileExists(t, filepath.Join(dir, "zarf.gen.yaml"))
			} else {
				require.NoError(t, err)
				require.FileExists(t, filepath.Join(dir, "child.gen.yaml"))
			}
		})
	}
}

func TestDevTemplateRendersComponentConfig(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "component.tpl.yaml")
	require.NoError(t, os.WriteFile(source, []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: [[ .componentName ]]
component: {}
`), 0o644))

	o := devTemplateOptions{set: map[string]string{"componentName": "app"}}
	require.NoError(t, o.run(context.Background(), []string{source}))

	generated, err := os.ReadFile(filepath.Join(dir, "component.gen.yaml"))
	require.NoError(t, err)
	require.Equal(t, `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: app
component: {}
`, string(generated))
}

func TestDevTemplateRendersInitPackage(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, packageTemplateFilename)
	definition := `apiVersion: zarf.dev/v1beta1
kind: ZarfInitConfig
metadata:
  name: custom-init
components:
  - name: registry
    service:
      name: registry
`
	require.NoError(t, os.WriteFile(source, []byte(definition), 0o644))

	require.NoError(t, (&devTemplateOptions{}).run(context.Background(), []string{source}))
	generated, err := os.ReadFile(filepath.Join(dir, "zarf.gen.yaml"))
	require.NoError(t, err)
	require.Equal(t, definition, string(generated))
}

func TestDevTemplateRequiresV1Beta1API(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, packageTemplateFilename)
	require.NoError(t, os.WriteFile(source, []byte(`apiVersion: zarf.dev/v1alpha1
kind: ZarfPackageConfig
metadata:
  name: app
`), 0o644))

	err := (&devTemplateOptions{}).run(context.Background(), []string{source})
	require.ErrorContains(t, err, `must use apiVersion "zarf.dev/v1beta1"`)
	_, statErr := os.Stat(filepath.Join(dir, "zarf.gen.yaml"))
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestDevTemplateRendersAnnotation(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, packageTemplateFilename), []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: app
  annotations:
    [[ .annotation ]]
components:
  - name: app
`), 0o644))

	o := devTemplateOptions{set: map[string]string{
		"annotation": "example.com/owner: platform-team",
	}}
	require.NoError(t, o.run(context.Background(), []string{dir}))

	generated, err := os.ReadFile(filepath.Join(dir, "zarf.gen.yaml"))
	require.NoError(t, err)
	require.Equal(t, `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: app
  annotations:
    example.com/owner: platform-team
components:
  - name: app
`, string(generated))
}

func TestDevTemplateRejectsInvalidSchema(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, packageTemplateFilename)
	require.NoError(t, os.WriteFile(source, []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: app
`), 0o644))

	err := (&devTemplateOptions{}).run(context.Background(), []string{source})
	require.ErrorContains(t, err, "linting error found")
	_, statErr := os.Stat(filepath.Join(dir, "zarf.gen.yaml"))
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestDevTemplateSkipsSchemaValidation(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, packageTemplateFilename)
	require.NoError(t, os.WriteFile(source, []byte(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: app
`), 0o644))

	o := devTemplateOptions{skipValidation: true}
	require.NoError(t, o.run(context.Background(), []string{source}))
	_, err := os.Stat(filepath.Join(dir, "zarf.gen.yaml"))
	require.NoError(t, err)
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
