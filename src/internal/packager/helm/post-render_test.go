// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
			name: "Pod gets label but has no template",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name": "test-pod",
				},
			}},
			expectLabel: true,
		},
		{
			name: "Secret gets label",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]interface{}{
					"name": "test-secret",
				},
			}},
			expectLabel: true,
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
			name: "CronJob without jobTemplate gets label on resource only",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "batch/v1",
				"kind":       "CronJob",
				"metadata": map[string]interface{}{
					"name": "test-cj-no-template",
				},
			}},
			expectLabel: true,
		},
		{
			name: "CronJob with jobTemplate gets label on resource and pod template",
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
			name: "GitRepository (Flux) gets label",
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

			r := &renderer{connectedDeploy: true}

			// Capture pre-existing labels for preservation check
			preLabels := tt.obj.GetLabels()

			err := r.addAgentIgnoreLabels(tt.obj)
			require.NoError(t, err)

			labels := tt.obj.GetLabels()
			if tt.expectLabel {
				require.Equal(t, "ignore", labels["zarf.dev/agent"])
			} else {
				if labels != nil {
					require.Empty(t, labels["zarf.dev/agent"])
				}
			}

			// Verify existing labels are preserved
			for k, v := range preLabels {
				require.Equal(t, v, labels[k], "pre-existing label %q should be preserved", k)
			}

			// If the resource was labeled and has a pod template, verify it was also labeled.
			// Check both standard and CronJob template paths.
			templatePaths := [][]string{
				{"spec", "template", "metadata", "labels"},
				{"spec", "jobTemplate", "spec", "template", "metadata", "labels"},
			}
			for _, path := range templatePaths {
				templateLabels, hasTemplate, err := unstructured.NestedStringMap(tt.obj.Object, path...)
				require.NoError(t, err)
				if tt.expectLabel && hasTemplate {
					require.Equal(t, "ignore", templateLabels["zarf.dev/agent"], "expected agent ignore label at path %v", path)
				} else if hasTemplate {
					require.Empty(t, templateLabels["zarf.dev/agent"], "unexpected agent ignore label at path %v", path)
				}
			}
		})
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
