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
