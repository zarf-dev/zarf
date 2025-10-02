// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package value supports values files and validation
package value

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestParseFiles(t *testing.T) {
	tests := []struct {
		name           string
		files          []string
		expectedResult Values
	}{
		{
			name:           "empty file list",
			files:          []string{},
			expectedResult: Values{},
		},
		{
			name:  "single valid YAML file",
			files: []string{"testdata/valid/simple.yaml"},
			expectedResult: Values{
				"my-component": map[string]any{
					"key1": "value1",
					"key2": "value2",
				},
			},
		},
		{
			name: "multiple YAML files merge",
			files: []string{
				"testdata/valid/merge1.yaml",
				"testdata/valid/merge2.yaml",
			},
			expectedResult: Values{
				"app": map[string]any{
					"name":     "myapp",
					"version":  "1.0",
					"replicas": uint64(3),
				},
				"config": map[string]any{
					"debug": true,
				},
			},
		},
		{
			name: "multiple YAML files can merge with later files overwriting previous ones",
			files: []string{
				"testdata/valid/merge1.yaml",
				"testdata/valid/merge2.yaml",
				"testdata/valid/merge-overwrite.yaml",
			},
			expectedResult: Values{
				"app": map[string]any{
					"name":     "myapp",
					"version":  "1.0",
					"replicas": uint64(4),
				},
				"config": map[string]any{
					"debug": true,
				},
			},
		},
		{
			name: "multiple YAML files can merge with later files overwriting previous ones (flipped order)",
			files: []string{
				"testdata/valid/merge-overwrite.yaml",
				"testdata/valid/merge2.yaml",
			},
			expectedResult: Values{
				"app": map[string]any{
					"replicas": uint64(3),
				},
				"config": map[string]any{
					"debug": true,
				},
			},
		},
		{
			name:  "complex nested YAML",
			files: []string{"testdata/valid/complex.yaml"},
			expectedResult: Values{
				"deployment": map[string]any{
					"replicas": uint64(3),
					"image": map[string]any{
						"repository": "nginx",
						"tag":        "1.21",
					},
					"resources": map[string]any{
						"limits": map[string]any{
							"cpu":    "100m",
							"memory": "128Mi",
						},
						"requests": map[string]any{
							"cpu":    "50m",
							"memory": "64Mi",
						},
					},
				},
				"service": map[string]any{
					"type":       "ClusterIP",
					"port":       uint64(80),
					"targetPort": uint64(8080),
				},
				"ingress": map[string]any{
					"enabled": true,
					"annotations": map[string]any{
						"kubernetes.io/ingress.class": "nginx",
					},
					"hosts": []any{
						map[string]any{
							"host": "example.com",
							"paths": []any{
								map[string]any{
									"path":     "/",
									"pathType": "Prefix",
								},
							},
						},
					},
				},
			},
		},
		{
			name:           "empty YAML file",
			files:          []string{"testdata/valid/empty.yaml"},
			expectedResult: Values{},
		},
		{
			name:  "YAML with null values",
			files: []string{"testdata/valid/nulls.yaml"},
			expectedResult: Values{
				"key1": nil,
				"key2": nil,
				"key3": "",
			},
		},
		{
			name:  "YAML with arrays and mixed types",
			files: []string{"testdata/valid/arrays.yaml"},
			expectedResult: Values{
				"items": []any{
					map[string]any{"name": "item1", "count": uint64(5), "enabled": true},
					map[string]any{"name": "item2", "count": uint64(10), "enabled": false},
				},
				"numbers": []any{
					uint64(1),
					uint64(2),
					uint64(3),
					uint64(4),
					uint64(5),
				},
				"mixed": map[string]any{
					"string":  "hello",
					"number":  uint64(42),
					"boolean": true,
					"float":   3.14,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)

			result, err := ParseFiles(ctx, tt.files, ParseFilesOptions{})
			require.NoError(t, err)
			require.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestParseFiles_Errors(t *testing.T) {
	tests := []struct {
		name  string
		files []string
	}{
		{
			name:  "non-existent file",
			files: []string{"testdata/nonexistent.yaml"},
		},
		{
			name: "both existing and non-existing files",
			files: []string{
				"testdata/valid/simple.yaml",
				"testdata/nonexistent.yaml",
			},
		},
		{
			name:  "invalid YAML syntax",
			files: []string{"testdata/invalid/malformed.yaml"},
		},
		{
			name:  "malformed YAML with tabs",
			files: []string{"testdata/invalid/tabs.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)

			result, err := ParseFiles(ctx, tt.files, ParseFilesOptions{})
			require.Error(t, err)
			require.Nil(t, result)
		})
	}
}

func TestExtract(t *testing.T) {
	tests := []struct {
		name   string
		values Values
		path   Path
		expect any
	}{
		{
			name: "extract root path returns entire map",
			values: Values{
				"key1": "value1",
				"key2": map[string]any{
					"nested": "value2",
				},
			},
			path: ".",
			expect: Values{
				"key1": "value1",
				"key2": map[string]any{
					"nested": "value2",
				},
			},
		},
		{
			name: "extract simple key",
			values: Values{
				"key1": "value1",
				"key2": "value2",
			},
			path:   ".key1",
			expect: "value1",
		},
		{
			name: "extract nested key",
			values: Values{
				"app": map[string]any{
					"name":    "myapp",
					"version": "1.0",
				},
			},
			path:   ".app.name",
			expect: "myapp",
		},
		{
			name: "extract deeply nested key",
			values: Values{
				"deployment": map[string]any{
					"resources": map[string]any{
						"limits": map[string]any{
							"cpu": "100m",
						},
					},
				},
			},
			path:   ".deployment.resources.limits.cpu",
			expect: "100m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := tt.values.Extract(tt.path)
			require.NoError(t, err)
			require.Equal(t, tt.expect, result)
		})
	}
}

func TestExtract_Errors(t *testing.T) {
	tests := []struct {
		name      string
		values    Values
		path      Path
		errSubstr string
	}{
		{
			name: "error on non-existent key",
			values: Values{
				"key1": "value1",
			},
			path:      ".key2",
			errSubstr: "not found",
		},
		{
			name: "error on non-existent nested key",
			values: Values{
				"app": map[string]any{
					"name": "myapp",
				},
			},
			path:      ".app.version",
			errSubstr: "not found",
		},
		{
			name: "error on traversing non-map",
			values: Values{
				"app": "string-value",
			},
			path:      ".app.name",
			errSubstr: "expected map",
		},
		{
			name:      "error on invalid path format (no leading dot)",
			values:    Values{},
			path:      "key",
			errSubstr: "invalid path format",
		},
		{
			name:      "error on empty path",
			values:    Values{},
			path:      "",
			errSubstr: "invalid path format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := tt.values.Extract(tt.path)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errSubstr)
			require.Nil(t, result)
		})
	}
}
