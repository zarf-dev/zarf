// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/internal/packager/postrender"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	tmpl "github.com/zarf-dev/zarf/src/pkg/template"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestChartTemplate(t *testing.T) {
	ctx := context.Background()
	chartPath := filepath.Join("testdata", "template", "simple-chart")
	chart := api.Chart{
		Name:          "simple-chart",
		LegacyVersion: "1.0.0",
		Local:         &api.LocalSource{Path: chartPath},
	}
	tmpdir := t.TempDir()
	paths := layout.ChartPaths{ChartsDir: tmpdir, ValuesDir: tmpdir}
	err := PackageChart(ctx, chart, paths, tmpdir, types.RemoteOptions{})
	require.NoError(t, err)
	kubeVersion := ""
	vc := template.GetZarfVariableConfig(ctx, false)
	vc.SetVariable("image", "nginx:1.0.0", false, false, api.RawVariableType)
	vc.SetVariable("port", "8080", false, false, api.RawVariableType)
	helmChart, values, err := LoadChartData(chart, paths, nil)
	require.NoError(t, err)
	manifest, err := TemplateChart(ctx, chart, helmChart, values, kubeVersion, vc, nil, false, types.RemoteOptions{})
	require.NoError(t, err)
	b, err := os.ReadFile(filepath.Join("testdata", "template", "expected", "manifest.yaml"))
	require.NoError(t, err)
	require.YAMLEq(t, string(b), manifest)
}

func templatePostRenderChart(t *testing.T, renderers []api.PostRenderer, objs tmpl.Objects) (string, error) {
	t.Helper()
	ctx := testutil.TestContext(t)
	chart := api.Chart{
		Name:          "post-render-chart",
		LegacyVersion: "1.0.0",
		Namespace:     "post-render",
		Local:         &api.LocalSource{Path: filepath.Join("testdata", "post-render", "chart")},
		PostRenderers: renderers,
	}
	tmpdir := t.TempDir()
	paths := layout.ChartPaths{ChartsDir: tmpdir, ValuesDir: tmpdir}
	require.NoError(t, PackageChart(ctx, chart, paths, tmpdir, types.RemoteOptions{}))
	helmChart, values, err := LoadChartData(chart, paths, nil)
	require.NoError(t, err)
	// A fresh variable config has no templates, so only post-renderers can cause a post-renderer to be set.
	vc := template.GetZarfVariableConfig(ctx, false)
	require.Empty(t, vc.GetAllTemplates())
	postRenderer, err := postrender.New(ctx, chart, objs)
	require.NoError(t, err)
	return TemplateChart(ctx, chart, helmChart, values, "", vc, postRenderer, false, types.RemoteOptions{})
}

func TestTemplateChart_postRenderersWithoutVariables(t *testing.T) {
	t.Parallel()

	manifest, err := templatePostRenderChart(t, []api.PostRenderer{{Kustomize: &api.KustomizePostRenderer{
		Patches: []api.KustomizePatch{{
			Patch: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  labels:
    patched: "true"
`,
		}},
		Images: []api.KustomizeImage{{Name: "docker.io/library/nginx", NewName: "registry.example.com/nginx", NewTag: "2.0.0"}},
	}}}, nil)
	require.NoError(t, err)

	deployment := findTemplatedDoc(t, manifest, "Deployment", "web")
	require.Equal(t, "true", deployment.GetLabels()["patched"])
	containers, found, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	require.NoError(t, err)
	require.True(t, found)
	container, ok := containers[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "registry.example.com/nginx:2.0.0", container["image"])

	// The default nohooks strategy leaves hooks out of Kustomize but keeps them in the output.
	hook := findTemplatedDoc(t, manifest, "ConfigMap", "web-hook")
	require.Equal(t, "pre-install", hook.GetAnnotations()["helm.sh/hook"])

	// Without Zarf variable templates, manifests keep their real template paths.
	require.Contains(t, manifest, "# Source: post-render-chart/templates/deployment.yaml")
	require.Contains(t, manifest, "# Source: post-render-chart/templates/service.yaml")
	require.NotContains(t, manifest, "# Source: chart.yaml")
	require.NotContains(t, manifest, "postrenderer.helm.sh/postrender-filename")
}

func TestTemplateChart_postRendererTemplating(t *testing.T) {
	t.Parallel()

	objs := tmpl.NewObjects(value.Values{"team": "blue"})
	manifest, err := templatePostRenderChart(t, []api.PostRenderer{{Kustomize: &api.KustomizePostRenderer{
		EnableTemplating: true,
		Patches: []api.KustomizePatch{{
			Patch: `apiVersion: v1
kind: Service
metadata:
  name: web
  labels:
    team: "{{ .Values.team }}"
`,
		}},
	}}}, objs)
	require.NoError(t, err)

	service := findTemplatedDoc(t, manifest, "Service", "web")
	require.Equal(t, "blue", service.GetLabels()["team"])
}

func TestTemplateChart_postRendererErrorIncludesChartName(t *testing.T) {
	t.Parallel()

	_, err := templatePostRenderChart(t, []api.PostRenderer{{Kustomize: &api.KustomizePostRenderer{
		Patches: []api.KustomizePatch{{Patch: `apiVersion: v1
kind: Secret
metadata:
  name: does-not-exist
`}},
	}}}, nil)
	require.ErrorContains(t, err, `chart "post-render-chart": post-renderer at index 0`)
	require.ErrorContains(t, err, "does-not-exist")
}

// findTemplatedDoc returns the document of the given kind and name from a multi-document manifest.
func findTemplatedDoc(t *testing.T, manifest, kind, name string) *unstructured.Unstructured {
	t.Helper()
	docs, err := utils.SplitYAML([]byte(manifest))
	require.NoError(t, err)
	for _, doc := range docs {
		if doc.GetKind() == kind && doc.GetName() == name {
			return doc
		}
	}
	require.Failf(t, "document not found", "%s/%s not found in manifest:\n%s", kind, name, manifest)
	return nil
}
