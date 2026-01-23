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
