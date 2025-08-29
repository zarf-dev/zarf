package value

import (
	"io/fs"
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

			result, err := ParseFiles(ctx, tt.files)
			require.NoError(t, err)
			require.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestParseFiles_Errors(t *testing.T) {
	tests := []struct {
		name          string
		files         []string
		expectedError error
	}{
		{
			name:          "non-existent file",
			files:         []string{"testdata/nonexistent.yaml"},
			expectedError: &fs.PathError{},
		},
		{
			name: "both existing and non-existing files",
			files: []string{
				"testdata/valid/simple.yaml",
				"testdata/nonexistent.yaml",
			},
			expectedError: &fs.PathError{},
		},
		{
			name:          "invalid YAML syntax",
			files:         []string{"testdata/invalid/malformed.yaml"},
			expectedError: &YAMLDecodeError{},
		},
		{
			name:          "malformed YAML with tabs",
			files:         []string{"testdata/invalid/tabs.yaml"},
			expectedError: &YAMLDecodeError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)

			result, err := ParseFiles(ctx, tt.files)
			require.Error(t, err)
			require.IsType(t, tt.expectedError, err)
			require.Nil(t, result)
		})
	}
}
