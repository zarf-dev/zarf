// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	postRenderFixture    = "16-helm-post-renderers"
	postRenderedImage    = "ghcr.io/zarf-dev/images/alpine:3.21.3"
	postRenderChartImage = "ghcr.io/zarf-dev/images/alpine:chart-default"
	postRenderTeamKey    = "post-render.zarf.dev/team"
	postRenderSMPKey     = "post-render.zarf.dev/smp"
	postRenderHookKey    = "post-render.zarf.dev/hook"
)

func TestHelmPostRenderers(t *testing.T) {
	t.Log("E2E: Helm chart post-renderers")
	fixture := filepath.Join("src", "test", "packages", postRenderFixture)

	t.Run("inspect manifests from the definition", func(t *testing.T) {
		stdOut, stdErr, err := e2e.Zarf(t, "dev", "inspect", "manifests", fixture)
		require.NoError(t, err, stdOut, stdErr)
		requirePostRenderedCharts(t, stdOut, "from-package")

		stdOut, stdErr, err = e2e.Zarf(t, "dev", "inspect", "manifests", fixture, "--set-values", "team=from-cli")
		require.NoError(t, err, stdOut, stdErr)
		requirePostRenderedCharts(t, stdOut, "from-cli")
	})

	t.Run("find-images sees the retargeted image", func(t *testing.T) {
		stdOut, stdErr, err := e2e.Zarf(t, "dev", "find-images", fixture, "--skip-cosign")
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdOut, postRenderedImage)
		require.NotContains(t, stdOut, postRenderChartImage)

		stdOut, stdErr, err = e2e.Zarf(t, "dev", "find-images", fixture, "--skip-cosign", "--why", postRenderedImage, "--set-values", "team=from-why")
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdOut, "chart: nohooks-app")
		require.Contains(t, stdOut, postRenderTeamKey+": from-why")

		// The chart's original image no longer exists after post-rendering.
		stdOut, stdErr, err = e2e.Zarf(t, "dev", "find-images", fixture, "--skip-cosign", "--why", postRenderChartImage)
		require.Error(t, err, stdOut, stdErr)
		require.Contains(t, stdErr, "not found in any charts or manifests")
	})

	t.Run("built package", func(t *testing.T) {
		tmpdir := t.TempDir()
		stdOut, stdErr, err := e2e.Zarf(t, "package", "create", fixture, "-o", tmpdir, "--skip-sbom", "--confirm")
		require.NoError(t, err, stdOut, stdErr)
		packagePath := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-helm-post-renderers-%s.tar.zst", e2e.Arch))
		require.FileExists(t, packagePath)

		stdOut, stdErr, err = e2e.Zarf(t, "package", "inspect", "definition", packagePath)
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdOut, "version: v1.1.0")
		require.Contains(t, stdOut, "reason: This package uses Helm chart post-renderers, which require v1.1.0+")
		// The post-renderers are kept in the built definition so deploy can apply them.
		require.Contains(t, stdOut, "postRenderStrategy: separate")
		require.Contains(t, stdOut, "enableTemplating: true")

		stdOut, stdErr, err = e2e.Zarf(t, "package", "inspect", "manifests", packagePath)
		require.NoError(t, err, stdOut, stdErr)
		requirePostRenderedCharts(t, stdOut, "from-package")

		stdOut, stdErr, err = e2e.Zarf(t, "package", "inspect", "manifests", packagePath, "--set-values", "team=from-package-cli")
		require.NoError(t, err, stdOut, stdErr)
		requirePostRenderedCharts(t, stdOut, "from-package-cli")
	})

	t.Run("invalid post-renderers are rejected", func(t *testing.T) {
		tests := []struct {
			name         string
			postRenderer string
			expected     string
		}{
			{
				name: "image without an override",
				postRenderer: `
        postRenderers:
          - kustomize:
              images:
                - name: ghcr.io/zarf-dev/images/alpine`,
				expected: `chart "app" postRenderers[0].kustomize.images[0] ("ghcr.io/zarf-dev/images/alpine") must set at least one of newName, newTag, or digest`,
			},
			{
				name: "patch that is not YAML",
				postRenderer: `
        postRenderers:
          - kustomize:
              patches:
                - patch: "op: add: path"`,
				expected: `chart "app" postRenderers[0].kustomize.patches[0] is not a valid Kustomize patch: unable to parse SM or JSON patch`,
			},
			{
				name: "JSON 6902 patch without a target",
				postRenderer: `
        postRenderers:
          - kustomize:
              patches:
                - patch: |
                    - op: add
                      path: /metadata/labels/team
                      value: platform`,
				expected: `chart "app" postRenderers[0].kustomize.patches[0] is not a valid Kustomize patch: must specify a target for JSON patch`,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				dir := t.TempDir()
				require.NoError(t, os.CopyFS(filepath.Join(dir, "chart"), os.DirFS(filepath.Join(fixture, "chart"))))
				definition := `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: invalid-post-renderers
components:
  - name: app
    charts:
      - name: app
        namespace: app
        local:
          path: chart` + tt.postRenderer + "\n"
				require.NoError(t, os.WriteFile(filepath.Join(dir, "zarf.yaml"), []byte(definition), 0o600))

				stdOut, stdErr, err := e2e.Zarf(t, "dev", "lint", dir)
				require.Error(t, err, stdOut, stdErr)
				require.Contains(t, stdErr, tt.expected)

				stdOut, stdErr, err = e2e.Zarf(t, "package", "create", dir, "-o", dir, "--skip-sbom", "--confirm")
				require.Error(t, err, stdOut, stdErr)
				require.Contains(t, stdErr, tt.expected)
			})
		}
	})
}

// requirePostRenderedCharts checks inspect output for the fixture's two charts, which are printed in definition order.
func requirePostRenderedCharts(t *testing.T, output, team string) {
	t.Helper()
	charts := splitInspectedCharts(t, output)
	require.Len(t, charts, 2, output)

	// nohooks-app: every post-renderer applies to the Deployment, and the hook passes through untouched.
	deployment := findPostRenderedDoc(t, charts[0], "Deployment", "app")
	require.Equal(t, "patched", deployment.GetLabels()[postRenderSMPKey])
	require.Equal(t, team, deployment.GetAnnotations()[postRenderTeamKey])
	seccomp, _, err := unstructured.NestedString(deployment.Object, "spec", "template", "spec", "securityContext", "seccompProfile", "type")
	require.NoError(t, err)
	require.Equal(t, "RuntimeDefault", seccomp)
	container := firstContainer(t, deployment)
	require.Equal(t, postRenderedImage, container["image"])
	require.Equal(t, []any{map[string]any{"name": "PATCHED_BY", "value": "json6902"}}, container["env"])
	hook := findPostRenderedDoc(t, charts[0], "Job", "app-hook")
	require.NotContains(t, hook.GetLabels(), postRenderHookKey)
	require.Equal(t, "post-install,post-upgrade", hook.GetAnnotations()["helm.sh/hook"])

	// separate-app: the hook gets its own pass, so it is patched and its image is retargeted too.
	deployment = findPostRenderedDoc(t, charts[1], "Deployment", "app")
	require.Equal(t, "patched", deployment.GetLabels()[postRenderSMPKey])
	require.NotContains(t, deployment.GetAnnotations(), postRenderTeamKey)
	require.Equal(t, postRenderedImage, firstContainer(t, deployment)["image"])
	hook = findPostRenderedDoc(t, charts[1], "Job", "app-hook")
	require.Equal(t, "patched", hook.GetLabels()[postRenderHookKey])
	require.Equal(t, "post-install,post-upgrade", hook.GetAnnotations()["helm.sh/hook"])
	require.Equal(t, postRenderedImage, firstContainer(t, hook)["image"])
}

// splitInspectedCharts splits `inspect manifests` output into the documents of each chart.
func splitInspectedCharts(t *testing.T, output string) [][]*unstructured.Unstructured {
	t.Helper()
	var charts [][]*unstructured.Unstructured
	for _, block := range strings.Split(output, "#type: chart\n")[1:] {
		docs, err := utils.SplitYAML([]byte(block))
		require.NoError(t, err)
		charts = append(charts, docs)
	}
	return charts
}

func findPostRenderedDoc(t *testing.T, docs []*unstructured.Unstructured, kind, name string) *unstructured.Unstructured {
	t.Helper()
	for _, doc := range docs {
		if doc.GetKind() == kind && doc.GetName() == name {
			return doc
		}
	}
	require.Failf(t, "document not found", "%s/%s not found", kind, name)
	return nil
}

func firstContainer(t *testing.T, obj *unstructured.Unstructured) map[string]any {
	t.Helper()
	containers, found, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
	require.NoError(t, err)
	require.True(t, found)
	require.NotEmpty(t, containers)
	container, ok := containers[0].(map[string]any)
	require.True(t, ok)
	return container
}
