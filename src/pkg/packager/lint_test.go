// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
package packager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/lint"
)

func TestLintPackageWithImports(t *testing.T) {
	testCases := []struct {
		name, fixture string
		opts          LintOptions
		images        []string
	}{
		{
			name: "composition", fixture: "compose",
			images: []string{"busybox:0.0.1", "busybox@sha256:3fbc632167424a6d997e74f52b878d7cc478225cffac6bc977eedfe51c7f4e79"},
		},
		{
			name: "create variables", fixture: "variables",
			opts:   LintOptions{CreateSetVariables: map[string]string{"BUSYBOX_TAG": "1.0.0"}},
			images: []string{"busybox:1.0.0", "busybox@sha256:3fbc632167424a6d997e74f52b878d7cc478225cffac6bc977eedfe51c7f4e79"},
		},
		{
			name: "flavor", fixture: "flavor", opts: LintOptions{Flavor: "good-flavor"},
			images: []string{"image-in-good-flavor-component:unpinned"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lintErr := Lint(t.Context(), filepath.Join("testdata", "lint-with-imports", tc.fixture), tc.opts)
			var got, want []string
			for _, finding := range lintErr.Findings {
				require.Equal(t, "composed definition", finding.Source)
				got = append(got, finding.RuleID+":"+finding.YqPath+":"+finding.Item)
			}
			want = append(want, "image-unpinned:.components.[0].images.[0]:"+tc.images[0])
			for i, image := range tc.images {
				want = append(want, fmt.Sprintf("image-registry-missing:.components.[0].images.[%d]:%s", i, image))
			}
			require.ElementsMatch(t, want, got)
			require.NoError(t, lintErr.Err)
			require.Empty(t, lintErr.NotEvaluated)
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
          - name: example.com/app:1.0
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
        - name: example.com/app:1.0
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
			lintErr := Lint(context.Background(), path, LintOptions{})
			require.ElementsMatch(t, []lint.PackageFinding{
				{RuleID: "repository-unpinned", Source: "composed definition", YqPath: tc.prefix + ".repositories.[0]", Description: "Repository is not pinned to a commit", Item: "https://example.com/repo.git", Severity: lint.SevWarn},
				{RuleID: "image-unpinned", Source: "composed definition", YqPath: tc.prefix + ".images.[0]", Description: "Image not pinned with digest", Item: "busybox:1.0", Severity: lint.SevInfo},
				{RuleID: "image-registry-missing", Source: "composed definition", YqPath: tc.prefix + ".images.[0]", Description: "Image reference does not specify a registry domain", Item: "busybox:1.0", Severity: lint.SevWarn},
				{RuleID: "file-checksum-missing", Source: "composed definition", YqPath: tc.prefix + ".files.[0]", Description: "No checksum for remote file", Item: "https://example.com/file.zip", Severity: lint.SevWarn},
				{RuleID: "image-archive-domain", Source: "composed definition", YqPath: tc.prefix + ".imageArchives.[0].images.[0].name", Description: "Image archive image should use a .internal domain to avoid resolving to a public registry", Item: "example.com/app:1.0", Severity: lint.SevWarn},
			}, lintErr.Findings)
			require.NoError(t, lintErr.Err)
			require.False(t, lintErr.FailsAt(lint.SevErr))
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

	lintErr := Lint(context.Background(), parentPath, LintOptions{})
	require.Equal(t, "parent", lintErr.PackageName)
	require.ElementsMatch(t, []lint.PackageFinding{
		{RuleID: "image-unpinned", Source: "composed definition", YqPath: ".component.images.[0]", Description: "Image not pinned with digest", Item: "busybox:1.0", Severity: lint.SevInfo},
		{RuleID: "image-registry-missing", Source: "composed definition", YqPath: ".component.images.[0]", Description: "Image reference does not specify a registry domain", Item: "busybox:1.0", Severity: lint.SevWarn},
	}, lintErr.Findings)
	require.NoError(t, lintErr.Err)
}
