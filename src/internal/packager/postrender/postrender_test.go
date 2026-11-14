// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package postrender

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/template"
	"github.com/zarf-dev/zarf/src/pkg/value"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kyaml/kio"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

type doc struct {
	file    string
	content string
}

// helmStream builds a post-renderer input the way Helm v4's annotateAndMerge does: every document annotated with its
// source filename and joined with kio.
func helmStream(t *testing.T, docs ...doc) []byte {
	t.Helper()
	var nodes []*kyaml.RNode
	for _, d := range docs {
		parsed, err := kio.ParseAll(d.content)
		require.NoError(t, err)
		for _, n := range parsed {
			require.NoError(t, n.PipeE(kyaml.SetAnnotation(filenameAnnotation, d.file)))
			nodes = append(nodes, n)
		}
	}
	s, err := kio.StringAll(nodes)
	require.NoError(t, err)
	return []byte(s)
}

// render runs renderers over in the way Zarf's Helm renderers do: New once, then Run.
func render(t *testing.T, in []byte, strategy api.PostRenderStrategy, renderers []api.PostRenderer, objs template.Objects) ([]byte, error) {
	t.Helper()
	chart := api.Chart{Name: "podinfo", PostRenderStrategy: strategy, PostRenderers: renderers}
	r, err := New(context.Background(), chart, objs)
	if err != nil {
		return nil, err
	}
	out, err := r.Run(bytes.NewBuffer(in))
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func parseOutput(t *testing.T, b []byte) []unstructured.Unstructured {
	t.Helper()
	nodes, err := kio.ParseAll(string(b))
	require.NoError(t, err)
	out := make([]unstructured.Unstructured, 0, len(nodes))
	for _, n := range nodes {
		m, err := n.Map()
		require.NoError(t, err)
		out = append(out, unstructured.Unstructured{Object: m})
	}
	return out
}

func find(t *testing.T, objs []unstructured.Unstructured, kind, name string) *unstructured.Unstructured {
	t.Helper()
	for i := range objs {
		if objs[i].GetKind() == kind && objs[i].GetName() == name {
			return &objs[i]
		}
	}
	require.Failf(t, "object not found", "%s/%s", kind, name)
	return nil
}

func nested(t *testing.T, o *unstructured.Unstructured, fields ...string) string {
	t.Helper()
	v, found, err := unstructured.NestedFieldNoCopy(o.Object, fields...)
	require.NoError(t, err)
	if !found {
		return ""
	}
	return fmt.Sprint(v)
}

func container(t *testing.T, o *unstructured.Unstructured) map[string]any {
	t.Helper()
	containers, found, err := unstructured.NestedSlice(o.Object, "spec", "template", "spec", "containers")
	require.NoError(t, err)
	require.True(t, found)
	require.NotEmpty(t, containers)
	c, ok := containers[0].(map[string]any)
	require.True(t, ok)
	return c
}

func containerImage(t *testing.T, o *unstructured.Unstructured) string {
	t.Helper()
	return fmt.Sprint(container(t, o)["image"])
}

func kustomizeRenderers(k api.KustomizePostRenderer) []api.PostRenderer {
	return []api.PostRenderer{{Kustomize: &k}}
}

const (
	deploymentFile = "podinfo/templates/deployment.yaml"
	deployment     = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: podinfo
  labels:
    app.kubernetes.io/name: podinfo
    app.kubernetes.io/instance: podinfo
  annotations:
    team: web
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: podinfo
  template:
    metadata:
      labels:
        app.kubernetes.io/name: podinfo
    spec:
      containers:
        - name: podinfo
          image: ghcr.io/stefanprodan/podinfo:6.5.0
`
	serviceFile = "podinfo/templates/service.yaml"
	service     = `apiVersion: v1
kind: Service
metadata:
  name: podinfo
  labels:
    app.kubernetes.io/name: podinfo
    app.kubernetes.io/instance: podinfo
spec:
  type: ClusterIP
  ports:
    - port: 9898
`
	serviceAccountFile = "podinfo/templates/serviceaccount.yaml"
	serviceAccount     = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: podinfo
  labels:
    app.kubernetes.io/instance: podinfo
`
	hookServiceAccountFile = "podinfo/templates/hooks/serviceaccount.yaml"
	hookServiceAccount     = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: podinfo
  labels:
    app.kubernetes.io/instance: podinfo
  annotations:
    helm.sh/hook: pre-install,pre-upgrade
    helm.sh/hook-weight: "-10"
`
	hookJobFile = "podinfo/templates/hooks/migrate-job.yaml"
	hookJob     = `apiVersion: batch/v1
kind: Job
metadata:
  name: podinfo-migrate
  labels:
    app.kubernetes.io/instance: podinfo
  annotations:
    helm.sh/hook: pre-install
    helm.sh/hook-delete-policy: before-hook-creation
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: migrate
          image: ghcr.io/stefanprodan/podinfo:6.5.0
`
	testPodFile = "podinfo/templates/tests/test-connection.yaml"
	testPod     = `apiVersion: v1
kind: Pod
metadata:
  generateName: podinfo-test-
  labels:
    app.kubernetes.io/instance: podinfo
  annotations:
    helm.sh/hook: test
spec:
  restartPolicy: Never
  containers:
    - name: curl
      image: curlimages/curl:8.5.0
`
	redisFile = "podinfo/charts/redis/templates/statefulset.yaml"
	redis     = `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: podinfo-redis
  labels:
    app.kubernetes.io/name: redis
spec:
  serviceName: podinfo-redis
  selector:
    matchLabels:
      app.kubernetes.io/name: redis
  template:
    metadata:
      labels:
        app.kubernetes.io/name: redis
    spec:
      containers:
        - name: redis
          image: docker.io/library/redis:7.2
`
	// instanceLabelPatch marks the resources it is applied to.
	instanceLabelPatch = `- op: add
  path: /metadata/labels/patched
  value: "true"
`
)

var instanceTarget = &api.KustomizeSelector{LabelSelector: "app.kubernetes.io/instance=podinfo"}

func TestNew_noRenderers(t *testing.T) {
	t.Parallel()
	for _, renderers := range [][]api.PostRenderer{nil, {}} {
		// A strategy without post-renderers is a no-op, even an unknown one.
		r, err := New(context.Background(), api.Chart{Name: "podinfo", PostRenderStrategy: "sometimes", PostRenderers: renderers}, nil)
		require.NoError(t, err)
		require.Nil(t, r)
	}
	// A nil Renderer returns its input untouched, without parsing it.
	in := bytes.NewBufferString("---\n# Source: x\n{{ not yaml\n---\n")
	out, err := (*Renderer)(nil).Run(in)
	require.NoError(t, err)
	require.Same(t, in, out)
}

func TestNeedsTemplating(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		renderers []api.PostRenderer
		want      bool
	}{
		{name: "no post-renderers"},
		{name: "templating disabled", renderers: kustomizeRenderers(api.KustomizePostRenderer{})},
		{name: "no renderer kind", renderers: []api.PostRenderer{{}}},
		{
			name:      "one renderer enables templating",
			renderers: []api.PostRenderer{{Kustomize: &api.KustomizePostRenderer{}}, {Kustomize: &api.KustomizePostRenderer{EnableTemplating: true}}},
			want:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, NeedsTemplating(api.Chart{PostRenderers: tt.renderers}))
		})
	}
}

func TestRun_emptyInput(t *testing.T) {
	t.Parallel()
	renderers := kustomizeRenderers(api.KustomizePostRenderer{Images: []api.KustomizeImage{{Name: "nginx", NewTag: "1"}}})
	for _, in := range []string{"", "---\n", "\n---\n# only a comment\n---\n", "apiVersion: v1\nkind: List\nitems: []\n"} {
		for _, strategy := range []api.PostRenderStrategy{"", api.PostRenderStrategyNoHooks, api.PostRenderStrategyCombined, api.PostRenderStrategySeparate} {
			out, err := render(t, []byte(in), strategy, renderers, nil)
			require.NoError(t, err)
			require.Empty(t, out)
		}
	}
}

func TestRun_kustomize(t *testing.T) {
	t.Parallel()
	input := helmStream(t, doc{deploymentFile, deployment}, doc{serviceFile, service}, doc{redisFile, redis})
	tests := []struct {
		name      string
		renderers []api.PostRenderer
		check     func(t *testing.T, objs []unstructured.Unstructured)
	}{
		{
			name: "strategic merge patch with target",
			renderers: kustomizeRenderers(api.KustomizePostRenderer{Patches: []api.KustomizePatch{{
				Patch:  "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: ignored-by-target\nspec:\n  replicas: 3\n",
				Target: &api.KustomizeSelector{Group: "apps", Version: "v1", Kind: "Deployment", Name: "podinfo"},
			}}}),
			check: func(t *testing.T, objs []unstructured.Unstructured) {
				require.Equal(t, "3", nested(t, find(t, objs, "Deployment", "podinfo"), "spec", "replicas"))
			},
		},
		{
			name: "strategic merge patch matched by name merges containers",
			renderers: kustomizeRenderers(api.KustomizePostRenderer{Patches: []api.KustomizePatch{{Patch: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: podinfo
spec:
  template:
    spec:
      containers:
        - name: podinfo
          resources:
            limits:
              memory: 128Mi
`}}}),
			check: func(t *testing.T, objs []unstructured.Unstructured) {
				c := container(t, find(t, objs, "Deployment", "podinfo"))
				require.Equal(t, "ghcr.io/stefanprodan/podinfo:6.5.0", c["image"])
				memory, _, err := unstructured.NestedString(c, "resources", "limits", "memory")
				require.NoError(t, err)
				require.Equal(t, "128Mi", memory)
			},
		},
		{
			name: "json6902 patch with target",
			renderers: kustomizeRenderers(api.KustomizePostRenderer{Patches: []api.KustomizePatch{{
				Patch:  "- op: replace\n  path: /spec/type\n  value: NodePort\n",
				Target: &api.KustomizeSelector{Version: "v1", Kind: "Service", Name: "podinfo"},
			}}}),
			check: func(t *testing.T, objs []unstructured.Unstructured) {
				require.Equal(t, "NodePort", nested(t, find(t, objs, "Service", "podinfo"), "spec", "type"))
				require.Equal(t, "1", nested(t, find(t, objs, "Deployment", "podinfo"), "spec", "replicas"))
			},
		},
		{
			name:      "label selector targets every matching kind",
			renderers: kustomizeRenderers(api.KustomizePostRenderer{Patches: []api.KustomizePatch{{Patch: instanceLabelPatch, Target: instanceTarget}}}),
			check: func(t *testing.T, objs []unstructured.Unstructured) {
				require.Equal(t, "true", find(t, objs, "Deployment", "podinfo").GetLabels()["patched"])
				require.Equal(t, "true", find(t, objs, "Service", "podinfo").GetLabels()["patched"])
				require.NotContains(t, find(t, objs, "StatefulSet", "podinfo-redis").GetLabels(), "patched")
			},
		},
		{
			name: "annotation selector",
			renderers: kustomizeRenderers(api.KustomizePostRenderer{Patches: []api.KustomizePatch{{
				Patch:  instanceLabelPatch,
				Target: &api.KustomizeSelector{AnnotationSelector: "team=web"},
			}}}),
			check: func(t *testing.T, objs []unstructured.Unstructured) {
				require.Equal(t, "true", find(t, objs, "Deployment", "podinfo").GetLabels()["patched"])
				require.NotContains(t, find(t, objs, "Service", "podinfo").GetLabels(), "patched")
			},
		},
		{
			name: "subchart resource patched and image renamed",
			renderers: kustomizeRenderers(api.KustomizePostRenderer{
				Patches: []api.KustomizePatch{{Patch: "apiVersion: apps/v1\nkind: StatefulSet\nmetadata:\n  name: podinfo-redis\nspec:\n  replicas: 2\n"}},
				Images:  []api.KustomizeImage{{Name: "docker.io/library/redis", NewName: "registry.example.com/redis", NewTag: "7.4"}},
			}),
			check: func(t *testing.T, objs []unstructured.Unstructured) {
				sts := find(t, objs, "StatefulSet", "podinfo-redis")
				require.Equal(t, "2", nested(t, sts, "spec", "replicas"))
				require.Equal(t, "registry.example.com/redis:7.4", containerImage(t, sts))
				require.Equal(t, "ghcr.io/stefanprodan/podinfo:6.5.0", containerImage(t, find(t, objs, "Deployment", "podinfo")))
			},
		},
		{
			name: "image digest",
			renderers: kustomizeRenderers(api.KustomizePostRenderer{Images: []api.KustomizeImage{{
				Name:   "ghcr.io/stefanprodan/podinfo",
				Digest: "sha256:0000000000000000000000000000000000000000000000000000000000000001",
			}}}),
			check: func(t *testing.T, objs []unstructured.Unstructured) {
				require.Equal(t, "ghcr.io/stefanprodan/podinfo@sha256:0000000000000000000000000000000000000000000000000000000000000001",
					containerImage(t, find(t, objs, "Deployment", "podinfo")))
			},
		},
		{
			name: "renderers run in order",
			renderers: []api.PostRenderer{
				{Kustomize: &api.KustomizePostRenderer{
					Patches: []api.KustomizePatch{{
						Patch:  "- op: add\n  path: /metadata/labels/tier\n  value: frontend\n",
						Target: &api.KustomizeSelector{Kind: "Deployment"},
					}},
					Images: []api.KustomizeImage{{Name: "ghcr.io/stefanprodan/podinfo", NewTag: "first"}},
				}},
				{Kustomize: &api.KustomizePostRenderer{
					// Only matches if the first renderer's label is already present.
					Patches: []api.KustomizePatch{{
						Patch:  "- op: replace\n  path: /spec/replicas\n  value: 5\n",
						Target: &api.KustomizeSelector{LabelSelector: "tier=frontend"},
					}},
					Images: []api.KustomizeImage{{Name: "ghcr.io/stefanprodan/podinfo", NewTag: "second"}},
				}},
			},
			check: func(t *testing.T, objs []unstructured.Unstructured) {
				d := find(t, objs, "Deployment", "podinfo")
				require.Equal(t, "5", nested(t, d, "spec", "replicas"))
				require.Equal(t, "ghcr.io/stefanprodan/podinfo:second", containerImage(t, d))
			},
		},
		{
			name:      "renderer without changes keeps resources",
			renderers: kustomizeRenderers(api.KustomizePostRenderer{Images: []api.KustomizeImage{{Name: "docker.io/library/nginx", NewTag: "unused"}}}),
			check: func(t *testing.T, objs []unstructured.Unstructured) {
				require.Equal(t, "ghcr.io/stefanprodan/podinfo:6.5.0", containerImage(t, find(t, objs, "Deployment", "podinfo")))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, err := render(t, input, api.PostRenderStrategyNoHooks, tt.renderers, nil)
			require.NoError(t, err)
			objs := parseOutput(t, out)
			require.Len(t, objs, 3)
			for _, o := range objs {
				require.NotEmpty(t, o.GetAnnotations()[filenameAnnotation], o.GetKind())
			}
			tt.check(t, objs)
		})
	}
}

// TestRun_strategies checks which documents each strategy post-renders, the output order, and that every document
// keeps its Helm filename annotation and hook annotation.
func TestRun_strategies(t *testing.T) {
	t.Parallel()
	renderers := kustomizeRenderers(api.KustomizePostRenderer{
		Patches: []api.KustomizePatch{{Patch: instanceLabelPatch, Target: instanceTarget}},
		Images:  []api.KustomizeImage{{Name: "ghcr.io/stefanprodan/podinfo", NewTag: "6.6.0"}},
	})
	tests := []struct {
		name         string
		strategy     api.PostRenderStrategy
		docs         []doc
		wantOrder    []string
		hooksPatched bool
	}{
		{
			name:      "default is nohooks",
			docs:      []doc{{hookJobFile, hookJob}, {deploymentFile, deployment}, {serviceFile, service}, {redisFile, redis}},
			wantOrder: []string{"Deployment/podinfo", "Service/podinfo", "StatefulSet/podinfo-redis", "Job/podinfo-migrate"},
		},
		{
			// Hooks that would break Kustomize (duplicate ids, generateName only) pass through untouched.
			name:     "nohooks with hostile hooks",
			strategy: api.PostRenderStrategyNoHooks,
			docs: []doc{
				{hookJobFile, hookJob}, {hookServiceAccountFile, hookServiceAccount}, {testPodFile, testPod},
				{deploymentFile, deployment}, {serviceAccountFile, serviceAccount},
			},
			wantOrder: []string{"Deployment/podinfo", "ServiceAccount/podinfo", "Job/podinfo-migrate", "ServiceAccount/podinfo", "Pod/"},
		},
		{
			name:         "combined keeps input order",
			strategy:     api.PostRenderStrategyCombined,
			docs:         []doc{{hookJobFile, hookJob}, {deploymentFile, deployment}, {serviceFile, service}, {redisFile, redis}},
			wantOrder:    []string{"Job/podinfo-migrate", "Deployment/podinfo", "Service/podinfo", "StatefulSet/podinfo-redis"},
			hooksPatched: true,
		},
		{
			name:     "separate tolerates a hook duplicating a resource",
			strategy: api.PostRenderStrategySeparate,
			docs: []doc{
				{hookJobFile, hookJob}, {hookServiceAccountFile, hookServiceAccount},
				{deploymentFile, deployment}, {serviceAccountFile, serviceAccount},
			},
			wantOrder:    []string{"Deployment/podinfo", "ServiceAccount/podinfo", "Job/podinfo-migrate", "ServiceAccount/podinfo"},
			hooksPatched: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, err := render(t, helmStream(t, tt.docs...), tt.strategy, renderers, nil)
			require.NoError(t, err)
			objs := parseOutput(t, out)

			wantFiles := map[string]string{}
			for _, d := range tt.docs {
				o := parseOutput(t, []byte(d.content))[0]
				_, hook := o.GetAnnotations()[releasev1.HookAnnotation]
				wantFiles[fmt.Sprintf("%s/%s/%t", o.GetKind(), o.GetName(), hook)] = d.file
			}
			var order []string
			for _, o := range objs {
				order = append(order, o.GetKind()+"/"+o.GetName())
				_, hook := o.GetAnnotations()[releasev1.HookAnnotation]
				id := fmt.Sprintf("%s/%s/%t", o.GetKind(), o.GetName(), hook)
				require.Equal(t, wantFiles[id], o.GetAnnotations()[filenameAnnotation], id)

				rendered := !hook || tt.hooksPatched
				_, instance := o.GetLabels()["app.kubernetes.io/instance"]
				require.Equal(t, rendered && instance, o.GetLabels()["patched"] == "true", id)
				if o.GetKind() == "Job" || o.GetKind() == "Deployment" {
					wantImage := "ghcr.io/stefanprodan/podinfo:6.5.0"
					if rendered {
						wantImage = "ghcr.io/stefanprodan/podinfo:6.6.0"
					}
					require.Equal(t, wantImage, containerImage(t, &o), id)
				}
			}
			require.Equal(t, tt.wantOrder, order)
		})
	}
}

// Helm output usually omits metadata.namespace. Kustomize treats an empty namespace as "default", so a namespace
// target only matches resources that set metadata.namespace explicitly (or "default" matches namespace-less ones).
func TestRun_namespaceTarget(t *testing.T) {
	t.Parallel()
	input := helmStream(t,
		doc{"podinfo/templates/explicit.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: explicit\n  namespace: podinfo\ndata: {}\n"},
		doc{"podinfo/templates/implicit.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: implicit\ndata: {}\n"},
	)
	tests := []struct {
		namespace string
		want      map[string]bool
	}{
		{namespace: "podinfo", want: map[string]bool{"explicit": true, "implicit": false}},
		{namespace: "default", want: map[string]bool{"explicit": false, "implicit": true}},
		{namespace: "", want: map[string]bool{"explicit": true, "implicit": true}},
	}
	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			t.Parallel()
			renderers := kustomizeRenderers(api.KustomizePostRenderer{Patches: []api.KustomizePatch{{
				Patch:  "- op: add\n  path: /data/patched\n  value: \"true\"\n",
				Target: &api.KustomizeSelector{Kind: "ConfigMap", Namespace: tt.namespace},
			}}})
			out, err := render(t, input, "", renderers, nil)
			require.NoError(t, err)
			objs := parseOutput(t, out)
			for name, patched := range tt.want {
				require.Equal(t, patched, nested(t, find(t, objs, "ConfigMap", name), "data", "patched") == "true", name)
			}
		})
	}
}

// Targeted patches that match nothing are silently skipped by Kustomize; untargeted strategic merge patches only
// need a match in each pass that runs (failures are covered in TestRun_error).
func TestRun_patchMatching(t *testing.T) {
	t.Parallel()
	input := helmStream(t, doc{deploymentFile, deployment}, doc{hookJobFile, hookJob})
	untargeted := api.KustomizePatch{Patch: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: podinfo\nspec:\n  replicas: 2\n"}
	targeted := untargeted
	targeted.Target = &api.KustomizeSelector{Kind: "Deployment", Name: "podinfo"}
	noMatch := []api.KustomizePatch{
		{Patch: "- op: replace\n  path: /spec/replicas\n  value: 9\n", Target: &api.KustomizeSelector{Kind: "DaemonSet"}},
		{Patch: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\ndata:\n  a: b\n", Target: &api.KustomizeSelector{Name: "missing"}},
	}
	tests := []struct {
		name         string
		strategy     api.PostRenderStrategy
		patches      []api.KustomizePatch
		wantReplicas string
	}{
		{name: "targeted no match is a no-op", strategy: api.PostRenderStrategyCombined, patches: noMatch, wantReplicas: "1"},
		{name: "untargeted under nohooks", strategy: api.PostRenderStrategyNoHooks, patches: []api.KustomizePatch{untargeted}, wantReplicas: "2"},
		{name: "untargeted under combined", strategy: api.PostRenderStrategyCombined, patches: []api.KustomizePatch{untargeted}, wantReplicas: "2"},
		{name: "targeted under separate", strategy: api.PostRenderStrategySeparate, patches: []api.KustomizePatch{targeted}, wantReplicas: "2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, err := render(t, input, tt.strategy, kustomizeRenderers(api.KustomizePostRenderer{Patches: tt.patches}), nil)
			require.NoError(t, err)
			objs := parseOutput(t, out)
			require.Len(t, objs, 2)
			require.Equal(t, tt.wantReplicas, nested(t, find(t, objs, "Deployment", "podinfo"), "spec", "replicas"))
		})
	}
}

func TestRun_templating(t *testing.T) {
	t.Parallel()
	input := helmStream(t, doc{deploymentFile, deployment})
	objs := template.NewObjects(value.Values{
		"registry": "registry.example.com",
		"tag":      "6.6.0",
		"replicas": 4,
		"digest":   "sha256:0000000000000000000000000000000000000000000000000000000000000002",
	})
	const templatedPatch = "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: podinfo\nspec:\n  replicas: {{ .Values.replicas }}\n"

	t.Run("disabled keeps literal braces", func(t *testing.T) {
		t.Parallel()
		renderers := kustomizeRenderers(api.KustomizePostRenderer{Patches: []api.KustomizePatch{{
			Patch: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: podinfo\n  annotations:\n    message: \"{{ .Values.registry }}\"\n",
		}}})
		// No templating data is required when templating is disabled.
		out, err := render(t, input, "", renderers, nil)
		require.NoError(t, err)
		require.Equal(t, "{{ .Values.registry }}", find(t, parseOutput(t, out), "Deployment", "podinfo").GetAnnotations()["message"])
	})

	t.Run("enabled substitutes values", func(t *testing.T) {
		t.Parallel()
		k := &api.KustomizePostRenderer{
			EnableTemplating: true,
			Patches:          []api.KustomizePatch{{Patch: templatedPatch}},
			Images:           []api.KustomizeImage{{Name: "ghcr.io/stefanprodan/podinfo", NewName: "{{ .Values.registry }}/podinfo", NewTag: "{{ .Values.tag }}"}},
		}
		out, err := render(t, input, "", []api.PostRenderer{{Kustomize: k}}, objs)
		require.NoError(t, err)
		d := find(t, parseOutput(t, out), "Deployment", "podinfo")
		require.Equal(t, "4", nested(t, d, "spec", "replicas"))
		require.Equal(t, "registry.example.com/podinfo:6.6.0", containerImage(t, d))
		// The caller's structs are not mutated.
		require.Equal(t, templatedPatch, k.Patches[0].Patch)
		require.Equal(t, "{{ .Values.registry }}/podinfo", k.Images[0].NewName)
	})

	t.Run("enabled digest", func(t *testing.T) {
		t.Parallel()
		renderers := kustomizeRenderers(api.KustomizePostRenderer{
			EnableTemplating: true,
			Images:           []api.KustomizeImage{{Name: "ghcr.io/stefanprodan/podinfo", Digest: "{{ .Values.digest }}"}},
		})
		out, err := render(t, input, "", renderers, objs)
		require.NoError(t, err)
		require.Equal(t, "ghcr.io/stefanprodan/podinfo@sha256:0000000000000000000000000000000000000000000000000000000000000002",
			containerImage(t, find(t, parseOutput(t, out), "Deployment", "podinfo")))
	})
}

func TestRun_lists(t *testing.T) {
	t.Parallel()
	list := `apiVersion: v1
kind: ConfigMapList
items:
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: first
    data: {}
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: second
      annotations:
        postrenderer.helm.sh/postrender-filename: podinfo/templates/other.yaml
    data: {}
`
	listFile := "podinfo/templates/configmaps.yaml"
	input := helmStream(t,
		doc{deploymentFile, deployment},
		doc{listFile, list},
		doc{"podinfo/templates/empty.yaml", "apiVersion: v1\nkind: List\nitems: []\n"},
		doc{serviceFile, service},
	)
	renderers := kustomizeRenderers(api.KustomizePostRenderer{Patches: []api.KustomizePatch{{
		Patch:  "- op: add\n  path: /data/patched\n  value: \"true\"\n",
		Target: &api.KustomizeSelector{Kind: "ConfigMap"},
	}}})
	out, err := render(t, input, api.PostRenderStrategyCombined, renderers, nil)
	require.NoError(t, err)
	objs := parseOutput(t, out)

	var order []string
	for _, o := range objs {
		order = append(order, o.GetKind()+"/"+o.GetName())
	}
	require.Equal(t, []string{"Deployment/podinfo", "ConfigMap/first", "ConfigMap/second", "Service/podinfo"}, order)
	first := find(t, objs, "ConfigMap", "first")
	require.Equal(t, "true", nested(t, first, "data", "patched"))
	require.Equal(t, listFile, first.GetAnnotations()[filenameAnnotation])
	require.Equal(t, "podinfo/templates/other.yaml", find(t, objs, "ConfigMap", "second").GetAnnotations()[filenameAnnotation])
}

func TestRun_inputFormatting(t *testing.T) {
	t.Parallel()
	// Hand-written stream with a leading separator, comment-only and empty documents, and Helm-style source comments.
	input := `---
# Source: podinfo/templates/NOTES-like-comment.yaml
---

---
# Source: podinfo/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: podinfo
  annotations:
    postrenderer.helm.sh/postrender-filename: podinfo/templates/service.yaml
spec:
  type: ClusterIP
---
---
# Source: podinfo/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: podinfo
  annotations:
    postrenderer.helm.sh/postrender-filename: podinfo/templates/configmap.yaml
data:
  literal: "{{ not a template }}"
`
	renderers := kustomizeRenderers(api.KustomizePostRenderer{Patches: []api.KustomizePatch{{
		Patch:  "- op: replace\n  path: /spec/type\n  value: LoadBalancer\n",
		Target: &api.KustomizeSelector{Kind: "Service"},
	}}})
	out, err := render(t, []byte(input), api.PostRenderStrategyCombined, renderers, nil)
	require.NoError(t, err)
	objs := parseOutput(t, out)
	require.Len(t, objs, 2)
	require.Equal(t, "LoadBalancer", nested(t, find(t, objs, "Service", "podinfo"), "spec", "type"))
	require.Equal(t, "{{ not a template }}", nested(t, find(t, objs, "ConfigMap", "podinfo"), "data", "literal"))
}

// TestRun_concurrent reuses one Renderer across goroutines, as Helm may when several releases render at once.
func TestRun_concurrent(t *testing.T) {
	t.Parallel()
	input := helmStream(t, doc{deploymentFile, deployment}, doc{hookJobFile, hookJob})
	r, err := New(context.Background(), api.Chart{
		Name:               "podinfo",
		PostRenderStrategy: api.PostRenderStrategySeparate,
		PostRenderers: kustomizeRenderers(api.KustomizePostRenderer{
			Patches: []api.KustomizePatch{{Patch: instanceLabelPatch, Target: instanceTarget}},
			Images:  []api.KustomizeImage{{Name: "ghcr.io/stefanprodan/podinfo", NewTag: "6.6.0"}},
		}),
	}, nil)
	require.NoError(t, err)
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = r.Run(bytes.NewBuffer(input))
		}()
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
}

func TestRun_error(t *testing.T) {
	t.Parallel()
	const secret = "s3cr3t-registry-password"
	objs := template.NewObjects(value.Values{"secret": secret, "empty": "", "tag": "6.6.0"})
	valid := api.PostRenderer{Kustomize: &api.KustomizePostRenderer{Images: []api.KustomizeImage{{Name: "a", NewTag: "b"}}}}
	labelPatch := api.KustomizePatch{Patch: instanceLabelPatch, Target: instanceTarget}
	labelRenderers := kustomizeRenderers(api.KustomizePostRenderer{Patches: []api.KustomizePatch{labelPatch}})
	templated := func(patches ...api.KustomizePatch) api.PostRenderer {
		return api.PostRenderer{Kustomize: &api.KustomizePostRenderer{EnableTemplating: true, Patches: patches}}
	}
	generateNamePod := strings.Replace(strings.Replace(testPod, "  annotations:\n    helm.sh/hook: test\n", "", 1), "podinfo-test-", "podinfo-debug-", 1)
	const (
		strategyHint = "post-render strategy"
		redacted     = "kustomize build failed (details redacted because templating is enabled)"
	)

	tests := []struct {
		name      string
		input     []byte // defaults to a single Deployment
		strategy  api.PostRenderStrategy
		renderers []api.PostRenderer
		want      []string
		notWant   []string
	}{
		{
			name:      "unknown strategy",
			strategy:  "sometimes",
			renderers: []api.PostRenderer{valid},
			want:      []string{`chart "podinfo": unknown post-render strategy "sometimes"`},
		},
		{
			name:      "no renderer kind",
			renderers: []api.PostRenderer{valid, {}},
			want:      []string{`chart "podinfo": post-renderer at index 1: kustomize is not set`},
		},
		{
			name:      "malformed input",
			input:     []byte("apiVersion: v1\nkind: ConfigMap\nmetadata: [\n"),
			renderers: []api.PostRenderer{valid},
			want:      []string{`chart "podinfo": parsing rendered manifests for post-rendering`},
		},
		{
			name:      "invalid patch",
			renderers: []api.PostRenderer{valid, {Kustomize: &api.KustomizePostRenderer{Patches: []api.KustomizePatch{{Patch: "spec: [unterminated"}}}}},
			want:      []string{`chart "podinfo": post-renderer at index 1: kustomize build failed: `},
		},
		{
			// Validation rejects blank patches; Kustomize still reports them clearly.
			name:      "blank patch",
			renderers: kustomizeRenderers(api.KustomizePostRenderer{Patches: []api.KustomizePatch{{Patch: "\n"}}}),
			want:      []string{"post-renderer at index 0: kustomize build failed: ", "must specify one of patch and path"},
		},
		{
			name:      "json6902 patch without target",
			renderers: kustomizeRenderers(api.KustomizePostRenderer{Patches: []api.KustomizePatch{{Patch: "- op: replace\n  path: /spec/replicas\n  value: 2\n"}}}),
			want:      []string{"post-renderer at index 0: kustomize build failed: "},
		},
		{
			name: "json6902 patch failing to apply",
			renderers: kustomizeRenderers(api.KustomizePostRenderer{Patches: []api.KustomizePatch{{
				Patch:  "- op: remove\n  path: /spec/doesNotExist\n",
				Target: &api.KustomizeSelector{Kind: "Deployment"},
			}}}),
			want: []string{"post-renderer at index 0: kustomize build failed: "},
		},
		{
			name:      "combined hook duplicates a resource",
			input:     helmStream(t, doc{hookServiceAccountFile, hookServiceAccount}, doc{serviceAccountFile, serviceAccount}),
			strategy:  api.PostRenderStrategyCombined,
			renderers: labelRenderers,
			want:      []string{"post-renderer at index 0: kustomize build failed", "already registered id", `use the "nohooks" or "separate" post-render strategy`},
		},
		{
			name:      "duplicate regular resources",
			input:     helmStream(t, doc{serviceAccountFile, serviceAccount}, doc{"podinfo/templates/serviceaccount-copy.yaml", serviceAccount}),
			strategy:  api.PostRenderStrategyNoHooks,
			renderers: labelRenderers,
			want:      []string{"already registered id", "Kustomize requires each resource's kind, name and namespace to be unique"},
			notWant:   []string{strategyHint},
		},
		{
			name:      "combined generateName hook",
			input:     helmStream(t, doc{deploymentFile, deployment}, doc{testPodFile, testPod}),
			strategy:  api.PostRenderStrategyCombined,
			renderers: labelRenderers,
			want:      []string{"post-renderer at index 0: kustomize build failed", "missing metadata.name", `use the "nohooks" post-render strategy`},
		},
		{
			// Hooks still go through Kustomize under separate, so generateName-only hooks fail there too.
			name:      "separate generateName hook",
			input:     helmStream(t, doc{deploymentFile, deployment}, doc{testPodFile, testPod}),
			strategy:  api.PostRenderStrategySeparate,
			renderers: labelRenderers,
			want:      []string{"post-renderer at index 0 (hooks): kustomize build failed", "missing metadata.name", `use the "nohooks" post-render strategy`},
		},
		{
			name:      "nohooks generateName resource",
			input:     helmStream(t, doc{deploymentFile, deployment}, doc{"podinfo/templates/debug.yaml", generateNamePod}),
			strategy:  api.PostRenderStrategyNoHooks,
			renderers: labelRenderers,
			want:      []string{"post-renderer at index 0: kustomize build failed", "a regular (non-hook) resource that only sets metadata.generateName cannot be post-rendered"},
			notWant:   []string{strategyHint},
		},
		{
			name:      "separate generateName resource",
			input:     helmStream(t, doc{deploymentFile, deployment}, doc{"podinfo/templates/debug.yaml", generateNamePod}),
			strategy:  api.PostRenderStrategySeparate,
			renderers: labelRenderers,
			want:      []string{"post-renderer at index 0 (resources): kustomize build failed", "a regular (non-hook) resource"},
			notWant:   []string{strategyHint},
		},
		{
			// The hooks pass has no Deployment to match.
			name:     "separate untargeted patch",
			input:    helmStream(t, doc{deploymentFile, deployment}, doc{hookJobFile, hookJob}),
			strategy: api.PostRenderStrategySeparate,
			renderers: kustomizeRenderers(api.KustomizePostRenderer{Patches: []api.KustomizePatch{{
				Patch: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: podinfo\nspec:\n  replicas: 2\n",
			}}}),
			want: []string{"post-renderer at index 0 (hooks)", "no resource matches strategic merge patch", "add a target to the patch"},
		},
		{
			name: "templating missing key",
			renderers: []api.PostRenderer{valid, {Kustomize: &api.KustomizePostRenderer{
				EnableTemplating: true,
				Images:           []api.KustomizeImage{{Name: "ghcr.io/stefanprodan/podinfo", NewTag: "{{ .Values.missing }}"}},
			}}},
			want: []string{`chart "podinfo": post-renderer at index 1: image override at index 0: templating failed: `, "missing"},
		},
		{
			name:      "blank patch after templating",
			renderers: []api.PostRenderer{templated(labelPatch, api.KustomizePatch{Patch: "{{ .Values.empty }}"})},
			want:      []string{`chart "podinfo": post-renderer at index 0: kustomize patch at index 1 is empty after templating`},
		},
		{
			name:      "whitespace-only patch after templating",
			renderers: []api.PostRenderer{valid, templated(api.KustomizePatch{Patch: "  {{ .Values.empty }}\n\t\n"})},
			want:      []string{"post-renderer at index 1: kustomize patch at index 0 is empty after templating"},
		},
		{
			name: "blank image name after templating",
			renderers: kustomizeRenderers(api.KustomizePostRenderer{EnableTemplating: true, Images: []api.KustomizeImage{
				{Name: "ghcr.io/stefanprodan/podinfo", NewTag: "{{ .Values.tag }}"},
				{Name: " {{ .Values.empty }} ", NewTag: "{{ .Values.tag }}"},
			}}),
			want: []string{"post-renderer at index 0: image override at index 1: name is empty after templating"},
		},
		{
			name:      "redacted unparseable patch",
			renderers: []api.PostRenderer{templated(labelPatch, api.KustomizePatch{Patch: "spec: [{{ .Values.secret }}\n"})},
			want:      []string{`chart "podinfo": post-renderer at index 0: ` + redacted},
			notWant:   []string{secret},
		},
		{
			name:      "redacted json6902 patch without target",
			renderers: []api.PostRenderer{templated(api.KustomizePatch{Patch: "- op: add\n  path: /metadata/annotations/token\n  value: {{ .Values.secret }}\n"})},
			want:      []string{"post-renderer at index 0: " + redacted},
			notWant:   []string{secret},
		},
		{
			// krusty panics (with the duplicate, templated ID in the message) when a patch renames resources into a
			// collision; the recovered panic must be redacted too, while the hint still applies.
			name: "redacted patch renaming resources into duplicate ids",
			input: helmStream(t, doc{deploymentFile, deployment},
				doc{"podinfo/templates/canary.yaml", strings.Replace(deployment, "name: podinfo\n", "name: podinfo-canary\n", 1)}),
			renderers: []api.PostRenderer{templated(api.KustomizePatch{
				Patch:  "- op: replace\n  path: /metadata/name\n  value: {{ .Values.secret }}\n",
				Target: &api.KustomizeSelector{Kind: "Deployment"},
			})},
			want:    []string{"post-renderer at index 0: " + redacted, "duplicate resource id"},
			notWant: []string{secret},
		},
		{
			// Kustomize errors can echo input objects, which now carry the templated annotation.
			name: "redacted renderer after a templated renderer",
			renderers: []api.PostRenderer{
				templated(api.KustomizePatch{
					Patch:  "- op: add\n  path: /metadata/annotations/token\n  value: {{ .Values.secret }}\n",
					Target: &api.KustomizeSelector{Kind: "Deployment"},
				}),
				{Kustomize: &api.KustomizePostRenderer{Patches: []api.KustomizePatch{labelPatch, {Patch: "spec: [unterminated\n"}}}},
			},
			want:    []string{"post-renderer at index 1: kustomize build failed (details redacted because an earlier post-renderer enables templating)"},
			notWant: []string{secret},
		},
		{
			name:      "unredacted without templating",
			renderers: kustomizeRenderers(api.KustomizePostRenderer{Patches: []api.KustomizePatch{{Patch: "spec: [" + secret + "\n"}}}),
			want:      []string{"post-renderer at index 0: kustomize build failed: ", secret},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			input := tt.input
			if input == nil {
				input = helmStream(t, doc{deploymentFile, deployment})
			}
			_, err := render(t, input, tt.strategy, tt.renderers, objs)
			require.Error(t, err)
			for _, want := range tt.want {
				require.ErrorContains(t, err, want)
			}
			for _, notWant := range tt.notWant {
				require.NotContains(t, err.Error(), notWant)
			}
			require.Equal(t, 1, strings.Count(err.Error(), `chart "podinfo"`), "the chart is named exactly once")
		})
	}
}
