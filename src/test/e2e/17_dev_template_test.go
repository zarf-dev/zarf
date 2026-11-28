// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDevTemplateRecursive(t *testing.T) {
	t.Log("E2E: Render nested local component templates")
	dir := t.TempDir()
	files := map[string]string{
		"zarf.tpl.yaml": `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: recursive-template
components:
  - name: app
    import:
      local:
        - path: child.gen.yaml
`,
		"child.tpl.yaml": `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: child
component:
  import:
    local:
      - path: leaf.gen.yaml
`,
		"leaf.tpl.yaml": `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: leaf
  description: [[ .description ]]
component: {}
`,
		"values.yaml": "description: from-file\n",
	}
	for name, contents := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644))
	}
	stdOut, stdErr, err := e2e.Zarf(t, "dev", "template", dir)
	require.NoError(t, err, stdOut, stdErr)
	require.NoFileExists(t, filepath.Join(dir, "child.gen.yaml"))

	stdOut, stdErr, err = e2e.Zarf(t, "dev", "template", dir, "--recursive",
		"--set-file", filepath.Join(dir, "values.yaml"), "--set", "description=from-set")
	require.NoError(t, err, stdOut, stdErr)
	require.FileExists(t, filepath.Join(dir, "child.gen.yaml"))
	leaf, err := os.ReadFile(filepath.Join(dir, "leaf.gen.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(leaf), "description: from-set")
}
