// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
package packager

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/lint"
)

func TestLintPackageWithImports(t *testing.T) {
	testCases := []struct {
		name     string
		path     string
		opts     LintOptions
		findings []lint.PackageFinding
	}{
		{
			name: "compose test ",
			path: filepath.Join("testdata", "lint-with-imports", "compose"),
			findings: []lint.PackageFinding{
				{
					YqPath:      ".components.[0].images.[0]",
					Description: "Image not pinned with digest",
					Item:        "busybox:0.0.1",
					Severity:    lint.SevWarn,
				},
				{
					YqPath:      ".components.[0].images.[0]",
					Description: "Image reference does not specify a registry domain",
					Item:        "busybox:0.0.1",
					Severity:    lint.SevWarn,
				},
				{
					YqPath:      ".components.[0].images.[1]",
					Description: "Image reference does not specify a registry domain",
					Item:        "busybox@sha256:3fbc632167424a6d997e74f52b878d7cc478225cffac6bc977eedfe51c7f4e79",
					Severity:    lint.SevWarn,
				},
			},
		},
		{
			name: "variables test",
			path: filepath.Join("testdata", "lint-with-imports", "variables"),
			opts: LintOptions{
				SetVariables: map[string]string{
					"BUSYBOX_TAG": "1.0.0",
				},
			},
			findings: []lint.PackageFinding{
				{
					YqPath:      ".components.[0].images.[0]",
					Description: "Image not pinned with digest",
					Item:        "busybox:1.0.0",
					Severity:    lint.SevWarn,
				},
				{
					YqPath:      ".components.[0].images.[0]",
					Description: "Image reference does not specify a registry domain",
					Item:        "busybox:1.0.0",
					Severity:    lint.SevWarn,
				},
				{
					YqPath:      ".components.[0].images.[1]",
					Description: "Image reference does not specify a registry domain",
					Item:        "busybox@sha256:3fbc632167424a6d997e74f52b878d7cc478225cffac6bc977eedfe51c7f4e79",
					Severity:    lint.SevWarn,
				},
			},
		},
		{
			name: "flavor test",
			path: filepath.Join("testdata", "lint-with-imports", "flavor"),
			opts: LintOptions{
				Flavor: "good-flavor",
			},
			findings: []lint.PackageFinding{
				{
					YqPath:      ".components.[0].images.[0]",
					Description: "Image not pinned with digest",
					Item:        "image-in-good-flavor-component:unpinned",
					Severity:    lint.SevWarn,
				},
				{
					YqPath:      ".components.[0].images.[0]",
					Description: "Image reference does not specify a registry domain",
					Item:        "image-in-good-flavor-component:unpinned",
					Severity:    lint.SevWarn,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			err := Lint(ctx, tc.path, tc.opts)
			var lintErr *lint.LintError
			require.ErrorAs(t, err, &lintErr)
			require.ElementsMatch(t, tc.findings, lintErr.Findings)
		})
	}
}

func TestLintV1Beta1PackageAndComponentConfig(t *testing.T) {
	t.Parallel()

	const packageYAML = `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: v1beta1-package
components:
  - name: app
    images:
      - name: busybox:1.0
    repositories:
      - url: https://example.com/repo.git
    files:
      - source: https://example.com/file.zip
        destination: /tmp/file.zip
    imageArchives:
      - path: images.tar
        images:
          - example.com/app:1.0
`
	const componentYAML = `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: app
component:
  images:
    - name: busybox:1.0
  repositories:
    - url: https://example.com/repo.git
  files:
    - source: https://example.com/file.zip
      destination: /tmp/file.zip
  imageArchives:
    - path: images.tar
      images:
        - example.com/app:1.0
`
	for _, tc := range []struct {
		name     string
		filename string
		contents string
		prefix   string
	}{
		{name: "package directory", filename: "zarf.yaml", contents: packageYAML, prefix: ".components.[0]"},
		{name: "component file", filename: "component.yaml", contents: componentYAML, prefix: ".component"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, tc.filename)
			require.NoError(t, os.WriteFile(path, []byte(tc.contents), 0o600))
			if tc.filename == "zarf.yaml" {
				path = dir
			}
			err := Lint(context.Background(), path, LintOptions{})
			var lintErr *lint.LintError
			require.ErrorAs(t, err, &lintErr)
			require.ElementsMatch(t, []lint.PackageFinding{
				{YqPath: tc.prefix + ".repositories.[0]", Description: "Unpinned repository", Item: "https://example.com/repo.git", Severity: lint.SevWarn},
				{YqPath: tc.prefix + ".images.[0]", Description: "Image not pinned with digest", Item: "busybox:1.0", Severity: lint.SevWarn},
				{YqPath: tc.prefix + ".images.[0]", Description: "Image reference does not specify a registry domain", Item: "busybox:1.0", Severity: lint.SevWarn},
				{YqPath: tc.prefix + ".files.[0]", Description: "No shasum for remote file", Item: "https://example.com/file.zip", Severity: lint.SevWarn},
				{YqPath: tc.prefix + ".imageArchives.[0].images.[0]", Description: "Image archive image should use a .internal domain to avoid resolving to a public registry", Item: "example.com/app:1.0", Severity: lint.SevWarn},
			}, lintErr.Findings)
			require.True(t, lintErr.OnlyWarnings())
		})
	}
}

func TestLintV1Beta1ComponentConfigWithImport(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	child := `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: child
component:
  images:
    - name: busybox:1.0
`
	parent := `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: parent
component:
  import:
    local:
      - path: child.yaml
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "child.yaml"), []byte(child), 0o600))
	parentPath := filepath.Join(dir, "parent.yaml")
	require.NoError(t, os.WriteFile(parentPath, []byte(parent), 0o600))

	err := Lint(context.Background(), parentPath, LintOptions{})
	var lintErr *lint.LintError
	require.ErrorAs(t, err, &lintErr)
	require.Equal(t, "parent", lintErr.PackageName)
	require.ElementsMatch(t, []lint.PackageFinding{
		{YqPath: ".component.images.[0]", Description: "Image not pinned with digest", Item: "busybox:1.0", Severity: lint.SevWarn},
		{YqPath: ".component.images.[0]", Description: "Image reference does not specify a registry domain", Item: "busybox:1.0", Severity: lint.SevWarn},
	}, lintErr.Findings)
}
