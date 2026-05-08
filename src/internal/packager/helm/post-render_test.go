// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helm

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/pki"
	"github.com/zarf-dev/zarf/src/pkg/state"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

func TestProcessManifestContentPreservesBlockScalarNewlines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		manifest      string
		dataKey       string
		expectedValue string
	}{
		{
			name: "block scalar with trailing newline (|)",
			manifest: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: default
data:
  file.txt: |
    line1
    line2
`,
			dataKey:       "file.txt",
			expectedValue: "line1\nline2\n",
		},
		{
			name: "block scalar strip trailing newline (|-)",
			manifest: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: default
data:
  file.txt: |-
    line1
    line2
`,
			dataKey:       "file.txt",
			expectedValue: "line1\nline2",
		},
		{
			name: "block scalar keep trailing newlines (|+)",
			manifest: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: default
data:
  file.txt: |+
    line1
    line2

`,
			dataKey:       "file.txt",
			expectedValue: "line1\nline2\n\n",
		},
		{
			name: "manifest without trailing newline - block scalar with clip (|)",
			manifest: strings.TrimSuffix(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: default
data:
  file.txt: |
    line1
    line2
`, "\n"),
			dataKey:       "file.txt",
			expectedValue: "line1\nline2\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Use the actual processManifestContent function from post-render.go
			outputContent, rawData, err := processManifestContent(tt.manifest, nil)
			require.NoError(t, err)
			require.NotNil(t, rawData)

			// Verify the output content can be parsed
			require.NotEmpty(t, outputContent)

			data, found, err := unstructured.NestedStringMap(rawData.Object, "data")
			require.NoError(t, err)
			require.True(t, found, "data field should exist in ConfigMap")

			actualValue, exists := data[tt.dataKey]
			require.True(t, exists, "data key %q should exist", tt.dataKey)

			require.Equal(t, tt.expectedValue, actualValue,
				"block scalar value should be preserved through processManifestContent")
		})
	}
}

func TestProcessManifestContentWithModifyFn(t *testing.T) {
	t.Parallel()

	manifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: default
data:
  config.yaml: |
    key: value
`

	// Test that modifyFn is called and changes are reflected
	outputContent, rawData, err := processManifestContent(manifest, func(obj *unstructured.Unstructured) error {
		labels := obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels["test-label"] = "test-value"
		obj.SetLabels(labels)
		return nil
	})
	require.NoError(t, err)
	require.NotNil(t, rawData)
	require.NotEmpty(t, outputContent)

	// Verify the label was added (check returned rawData directly)
	labels := rawData.GetLabels()
	require.Equal(t, "test-value", labels["test-label"])

	// Verify the data was still preserved
	data, found, err := unstructured.NestedStringMap(rawData.Object, "data")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "key: value\n", data["config.yaml"])
}

func TestRendererShouldAddAgentIgnoreLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		renderer renderer
		expected bool
	}{
		{
			name: "connected deploy with configured agent",
			renderer: renderer{
				connectedDeploy: true,
				state: &state.State{
					AgentTLS: pki.GeneratedPKI{Cert: []byte("cert")},
				},
			},
			expected: true,
		},
		{
			name: "connected deploy without configured agent",
			renderer: renderer{
				connectedDeploy: true,
				state:           &state.State{},
			},
			expected: false,
		},
		{
			name: "airgap deploy with configured agent",
			renderer: renderer{
				state: &state.State{
					AgentTLS: pki.GeneratedPKI{Cert: []byte("cert")},
				},
			},
			expected: false,
		},
		{
			name: "connected deploy without state",
			renderer: renderer{
				connectedDeploy: true,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, tt.renderer.shouldAddAgentIgnoreLabels())
		})
	}
}

func TestAddAgentIgnoreLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		obj         *unstructured.Unstructured
		expectLabel bool
	}{
		{
			name: "Deployment gets label on resource and pod template",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name": "test-deploy",
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"app": "test",
							},
						},
					},
				},
			}},
			expectLabel: true,
		},
		{
			name: "ArgoCD repository secret gets label",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]interface{}{
					"name": "test-argocd-repo",
					"labels": map[string]interface{}{
						"argocd.argoproj.io/secret-type": "repository",
					},
				},
			}},
			expectLabel: true,
		},
		{
			name: "Non-ArgoCD secret is not labeled",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]interface{}{
					"name": "test-tls-secret",
				},
			}},
			expectLabel: false,
		},
		{
			name: "ConfigMap is not an agent-mutated kind, no label",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name": "test-cm",
				},
			}},
			expectLabel: false,
		},
		{
			name: "StatefulSet with pod template",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "StatefulSet",
				"metadata": map[string]interface{}{
					"name": "test-sts",
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{},
						},
					},
				},
			}},
			expectLabel: true,
		},
		{
			name: "CronJob with jobTemplate gets label on pod template",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "batch/v1",
				"kind":       "CronJob",
				"metadata": map[string]interface{}{
					"name": "test-cj",
				},
				"spec": map[string]interface{}{
					"jobTemplate": map[string]interface{}{
						"spec": map[string]interface{}{
							"template": map[string]interface{}{
								"metadata": map[string]interface{}{
									"labels": map[string]interface{}{
										"app": "cron",
									},
								},
							},
						},
					},
				},
			}},
			expectLabel: true,
		},
		{
			name: "GitRepository (Flux) gets label even with no existing labels",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "source.toolkit.fluxcd.io/v1",
				"kind":       "GitRepository",
				"metadata": map[string]interface{}{
					"name": "test-git-repo",
				},
			}},
			expectLabel: true,
		},
		{
			name: "Deployment with no existing labels gets label added",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name": "test-deploy-no-labels",
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{},
						},
					},
				},
			}},
			expectLabel: true,
		},
		{
			name: "Deployment preserves existing labels",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name": "test-deploy-existing",
					"labels": map[string]interface{}{
						"existing": "label",
					},
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"app": "myapp",
							},
						},
					},
				},
			}},
			expectLabel: true,
		},
		{
			name: "Deployment with spec.template.metadata but no labels key creates labels",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name": "test-deploy-no-labels-key",
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{},
					},
				},
			}},
			expectLabel: true,
		},
		{
			name: "Deployment with spec.template but no metadata key creates labels",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name": "test-deploy-no-metadata",
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{},
				},
			}},
			expectLabel: true,
		},
		{
			name: "Service is not agent-mutated",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Service",
				"metadata": map[string]interface{}{
					"name": "test-svc",
				},
			}},
			expectLabel: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Capture pre-existing top-level labels for preservation check
			preLabels := tt.obj.GetLabels()

			err := addAgentIgnoreLabels(tt.obj)
			require.NoError(t, err)

			// Verify existing top-level labels are preserved
			labels := tt.obj.GetLabels()
			for k, v := range preLabels {
				require.Equal(t, v, labels[k], "pre-existing label %q should be preserved", k)
			}

			// Verify the label was set at the paths defined in agentMutatedKinds
			for _, path := range agentMutatedKinds[tt.obj.GroupVersionKind().GroupKind()] {
				pathLabels, found, err := unstructured.NestedStringMap(tt.obj.Object, path...)
				require.NoError(t, err)
				if tt.expectLabel {
					require.True(t, found, "expected labels to exist at path %v", path)
					require.Equal(t, "ignore", pathLabels["zarf.dev/agent"], "expected agent ignore label at path %v", path)
				} else {
					if found {
						require.Empty(t, pathLabels["zarf.dev/agent"], "unexpected agent ignore label at path %v", path)
					}
				}
			}
		})
	}
}

func TestAgentMutatedKindsMatchesWebhook(t *testing.T) {
	t.Parallel()

	webhookPath := filepath.Join("..", "..", "..", "..", "packages", "zarf-agent", "chart", "templates", "webhook.yaml")
	data, err := os.ReadFile(webhookPath)
	require.NoError(t, err)

	// Strip Helm template directives so the manifest can be parsed as plain YAML.
	cleaned := regexp.MustCompile(`{{[^}]*}}`).ReplaceAllString(string(data), "placeholder")

	// Only parse the rules — decoding the full MutatingWebhookConfiguration would
	// fail on the templated caBundle placeholder which is not valid base64.
	var cfg struct {
		Webhooks []struct {
			Rules []admissionregistrationv1.RuleWithOperations `json:"rules"`
		} `json:"webhooks"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(cleaned), &cfg))
	require.NotEmpty(t, cfg.Webhooks, "expected webhook configuration to contain webhooks")

	// Maps plural resource names from the webhook rules to their Kind.
	// Update this when adding a new resource to webhook.yaml.
	resourceToKind := map[string]string{
		"pods":             "Pod",
		"secrets":          "Secret",
		"gitrepositories":  "GitRepository",
		"ocirepositories":  "OCIRepository",
		"helmrepositories": "HelmRepository",
		"applications":     "Application",
		"applicationsets":  "ApplicationSet",
		"appprojects":      "AppProject",
	}

	webhookGroupKinds := map[schema.GroupKind]struct{}{}
	for _, w := range cfg.Webhooks {
		for _, rule := range w.Rules {
			for _, group := range rule.APIGroups {
				for _, resource := range rule.Resources {
					// Skip subresources (e.g. pods/ephemeralcontainers).
					if strings.Contains(resource, "/") {
						continue
					}
					kind, ok := resourceToKind[resource]
					require.Truef(t, ok, "no Kind mapping for webhook resource %q — update resourceToKind in this test", resource)
					webhookGroupKinds[schema.GroupKind{Group: group, Kind: kind}] = struct{}{}
				}
			}
		}
	}

	// Every webhook-targeted GroupKind must be present in agentMutatedKinds so
	// that addAgentIgnoreLabels can annotate it with the ignore label.
	for gk := range webhookGroupKinds {
		_, ok := agentMutatedKinds[gk]
		require.Truef(t, ok, "webhook targets %v but it is missing from agentMutatedKinds", gk)
	}

	// Workload controllers are included in agentMutatedKinds even though the webhook does not mutate
	// them directly, because they create pods that the webhook will mutate.
	// Every other entry in agentMutatedKinds must correspond to a webhook-targeted GroupKind.
	podControllers := map[schema.GroupKind]struct{}{
		{Group: "apps", Kind: "Deployment"}:  {},
		{Group: "apps", Kind: "StatefulSet"}: {},
		{Group: "apps", Kind: "DaemonSet"}:   {},
		{Group: "apps", Kind: "ReplicaSet"}:  {},
		{Group: "batch", Kind: "Job"}:        {},
		{Group: "batch", Kind: "CronJob"}:    {},
	}
	for gk := range agentMutatedKinds {
		if _, ok := podControllers[gk]; ok {
			continue
		}
		_, ok := webhookGroupKinds[gk]
		require.Truef(t, ok, "agentMutatedKinds has %v but no matching webhook rule exists", gk)
	}
}

func TestProcessManifestContentEmptyObject(t *testing.T) {
	t.Parallel()

	// Empty or blank YAML should return original content
	emptyManifest := ""
	outputContent, rawData, err := processManifestContent(emptyManifest, nil)
	require.NoError(t, err)
	require.Equal(t, emptyManifest, outputContent)
	require.NotNil(t, rawData)
	require.Empty(t, rawData.Object)
}
