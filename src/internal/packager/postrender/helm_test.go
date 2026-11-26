// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package postrender

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	"helm.sh/helm/v4/pkg/postrenderer"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage"
	"helm.sh/helm/v4/pkg/storage/driver"
)

var _ postrenderer.PostRenderer = (*Renderer)(nil)

func testChart(withDuplicateHook, withTestPod bool) *chartv2.Chart {
	file := func(name, data string) *common.File { return &common.File{Name: name, Data: []byte(data)} }
	ch := &chartv2.Chart{
		Metadata: &chartv2.Metadata{APIVersion: chartv2.APIVersionV2, Name: "podinfo", Version: "0.1.0"},
		Templates: []*common.File{
			file("templates/deployment.yaml", deployment),
			file("templates/service.yaml", service),
			file("templates/serviceaccount.yaml", serviceAccount),
			file("templates/hooks/migrate-job.yaml", hookJob),
			file("templates/configmaps.yaml", `apiVersion: v1
kind: List
items:
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: podinfo-config
      labels:
        app.kubernetes.io/instance: podinfo
    data: {}
`),
			file("templates/NOTES.txt", "podinfo installed"),
		},
		Files: []*common.File{
			file("crds/widgets.yaml", `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
  labels:
    app.kubernetes.io/instance: podinfo
spec:
  group: example.com
  names:
    kind: Widget
    plural: widgets
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
`),
		},
	}
	if withDuplicateHook {
		ch.Templates = append(ch.Templates, file("templates/hooks/serviceaccount.yaml", hookServiceAccount))
	}
	if withTestPod {
		ch.Templates = append(ch.Templates, file("templates/tests/test-connection.yaml", testPod))
	}
	redisChart := &chartv2.Chart{
		Metadata:  &chartv2.Metadata{APIVersion: chartv2.APIVersionV2, Name: "redis", Version: "0.1.0"},
		Templates: []*common.File{file("templates/statefulset.yaml", redis)},
	}
	ch.AddDependency(redisChart)
	return ch
}

func helmTemplate(t *testing.T, ch *chartv2.Chart, pr *Renderer) (*releasev1.Release, error) {
	t.Helper()
	cfg := action.NewConfiguration()
	cfg.Releases = storage.Init(driver.NewMemory())
	cfg.KubeClient = &kubefake.PrintingKubeClient{Out: io.Discard, LogOutput: io.Discard}
	cfg.Capabilities = common.DefaultCapabilities
	client := action.NewInstall(cfg)
	client.DryRunStrategy = action.DryRunClient
	client.Replace = true
	client.IncludeCRDs = true
	client.ReleaseName = "podinfo"
	client.Namespace = "podinfo"
	client.PostRenderer = pr
	rel, err := client.RunWithContext(context.Background(), ch, map[string]any{})
	if err != nil {
		return nil, err
	}
	r, ok := rel.(*releasev1.Release)
	require.True(t, ok)
	return r, nil
}

// TestRun_helmInstall drives a Renderer through Helm v4's real post-render plumbing (default combined Helm strategy) to verify
// stream compatibility, filename re-association, CRD exclusion, and hook handling per Zarf strategy.
func TestRun_helmInstall(t *testing.T) {
	t.Parallel()
	renderers := kustomizeRenderers(api.KustomizePostRenderer{
		Patches: []api.KustomizePatch{{Patch: instanceLabelPatch, Target: instanceTarget}},
		Images: []api.KustomizeImage{
			{Name: "ghcr.io/stefanprodan/podinfo", NewTag: "6.6.0"},
			{Name: "docker.io/library/redis", NewTag: "7.4"},
		},
	})

	tests := []struct {
		name              string
		strategy          api.PostRenderStrategy
		withDuplicateHook bool
		withTestPod       bool
		wantErr           string
		wantHooksPatched  bool
	}{
		{name: "nohooks with hostile hooks", strategy: api.PostRenderStrategyNoHooks, withDuplicateHook: true, withTestPod: true},
		{name: "combined", strategy: api.PostRenderStrategyCombined, wantHooksPatched: true},
		{name: "combined duplicate hook", strategy: api.PostRenderStrategyCombined, withDuplicateHook: true, wantErr: "already registered id"},
		{name: "combined generateName hook", strategy: api.PostRenderStrategyCombined, withTestPod: true, wantErr: "missing metadata.name"},
		{name: "separate duplicate hook", strategy: api.PostRenderStrategySeparate, withDuplicateHook: true, wantHooksPatched: true},
		{name: "separate generateName hook", strategy: api.PostRenderStrategySeparate, withTestPod: true, wantErr: "missing metadata.name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pr, err := New(context.Background(), api.Chart{Name: "podinfo", PostRenderStrategy: tt.strategy, PostRenderers: renderers}, nil)
			require.NoError(t, err)
			rel, err := helmTemplate(t, testChart(tt.withDuplicateHook, tt.withTestPod), pr)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.ErrorContains(t, err, `chart "podinfo": post-renderer at index 0`)
				return
			}
			require.NoError(t, err)

			// Helm re-associates every document with its original template and strips the annotation.
			require.NotContains(t, rel.Manifest, filenameAnnotation)
			require.NotContains(t, rel.Manifest, "generated-by-postrender")
			for _, src := range []string{
				"# Source: podinfo/templates/deployment.yaml",
				"# Source: podinfo/templates/service.yaml",
				"# Source: podinfo/templates/serviceaccount.yaml",
				"# Source: podinfo/templates/configmaps.yaml",
				"# Source: podinfo/charts/redis/templates/statefulset.yaml",
			} {
				require.Contains(t, rel.Manifest, src)
			}
			require.Contains(t, rel.Manifest, "image: ghcr.io/stefanprodan/podinfo:6.6.0")
			require.Contains(t, rel.Manifest, "image: docker.io/library/redis:7.4")
			require.Equal(t, 4, strings.Count(rel.Manifest, `patched: "true"`), "deployment, service, serviceaccount and configmap")

			// CRDs from crds/ are never post-rendered.
			require.Contains(t, rel.Manifest, "name: widgets.example.com")
			crd := rel.Manifest[strings.Index(rel.Manifest, "kind: CustomResourceDefinition"):]
			crd = crd[:strings.Index(crd, "---")]
			require.NotContains(t, crd, "patched")

			wantHooks := 1
			if tt.withDuplicateHook {
				wantHooks++
			}
			if tt.withTestPod {
				wantHooks++
			}
			require.Len(t, rel.Hooks, wantHooks)
			for _, h := range rel.Hooks {
				require.NotContains(t, h.Manifest, filenameAnnotation)
				require.True(t, strings.HasPrefix(h.Path, "podinfo/templates/"), h.Path)
				require.Equal(t, tt.wantHooksPatched, strings.Contains(h.Manifest, `patched: "true"`), h.Path)
				if h.Kind == "Job" {
					require.Equal(t, tt.wantHooksPatched, strings.Contains(h.Manifest, "podinfo:6.6.0"), h.Manifest)
				}
			}
		})
	}
}
