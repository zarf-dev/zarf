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

func TestSet(t *testing.T) {
	tests := []struct {
		name   string
		values Values
		path   Path
		value  any
		expect Values
	}{
		{
			name:   "set at root path merges map contents",
			values: Values{"existing": "value"},
			path:   ".",
			value:  map[string]any{"new": "data", "another": 123},
			expect: Values{"existing": "value", "new": "data", "another": 123},
		},
		{
			name:   "set at root overwrites existing keys",
			values: Values{"key": "old"},
			path:   ".",
			value:  map[string]any{"key": "new"},
			expect: Values{"key": "new"},
		},
		{
			name:   "set simple key",
			values: Values{},
			path:   ".key1",
			value:  "value1",
			expect: Values{"key1": "value1"},
		},
		{
			name:   "set overwrites existing key",
			values: Values{"key1": "old"},
			path:   ".key1",
			value:  "new",
			expect: Values{"key1": "new"},
		},
		{
			name:   "set nested key creates intermediate maps",
			values: Values{},
			path:   ".app.name",
			value:  "myapp",
			expect: Values{"app": map[string]any{"name": "myapp"}},
		},
		{
			name:   "set nested key in existing map",
			values: Values{"app": map[string]any{"version": "1.0"}},
			path:   ".app.name",
			value:  "myapp",
			expect: Values{"app": map[string]any{"version": "1.0", "name": "myapp"}},
		},
		{
			name:   "set deeply nested key",
			values: Values{},
			path:   ".deployment.resources.limits.cpu",
			value:  "100m",
			expect: Values{
				"deployment": map[string]any{
					"resources": map[string]any{
						"limits": map[string]any{
							"cpu": "100m",
						},
					},
				},
			},
		},
		{
			name:   "set with various value types",
			values: Values{},
			path:   ".config",
			value:  map[string]any{"enabled": true, "count": 42, "rate": 3.14},
			expect: Values{
				"config": map[string]any{"enabled": true, "count": 42, "rate": 3.14},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.values.Set(tt.path, tt.value)
			require.NoError(t, err)
			require.Equal(t, tt.expect, tt.values)
		})
	}
}

func TestSet_Errors(t *testing.T) {
	tests := []struct {
		name      string
		values    Values
		path      Path
		value     any
		errSubstr string
	}{
		{
			name:      "error on non-map value at root",
			values:    Values{},
			path:      ".",
			value:     "string value",
			errSubstr: "cannot merge non-map value at root path",
		},
		{
			name:      "error on conflict with non-map",
			values:    Values{"app": "string-value"},
			path:      ".app.name",
			value:     "myapp",
			errSubstr: "conflict",
		},
		{
			name:      "error on invalid path format (no leading dot)",
			values:    Values{},
			path:      "key",
			value:     "value",
			errSubstr: "invalid path format",
		},
		{
			name:      "error on empty path",
			values:    Values{},
			path:      "",
			value:     "value",
			errSubstr: "invalid path format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.values.Set(tt.path, tt.value)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errSubstr)
		})
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name    string
		dst     Values
		sources []Values
		expect  Values
	}{
		{
			name:    "merge single map",
			dst:     Values{"key1": "value1"},
			sources: []Values{{"key2": "value2"}},
			expect:  Values{"key1": "value1", "key2": "value2"},
		},
		{
			name: "merge multiple maps",
			dst:  Values{"key1": "value1"},
			sources: []Values{
				{"key2": "value2"},
				{"key3": "value3"},
			},
			expect: Values{"key1": "value1", "key2": "value2", "key3": "value3"},
		},
		{
			name: "later sources override earlier ones",
			dst:  Values{"key": "original"},
			sources: []Values{
				{"key": "first"},
				{"key": "second"},
			},
			expect: Values{"key": "second"},
		},
		{
			name: "nested maps merge recursively",
			dst: Values{
				"app": map[string]any{
					"name": "myapp",
				},
			},
			sources: []Values{
				{
					"app": map[string]any{
						"version": "1.0",
					},
				},
			},
			expect: Values{
				"app": map[string]any{
					"name":    "myapp",
					"version": "1.0",
				},
			},
		},
		{
			name: "deeply nested maps merge",
			dst: Values{
				"deployment": map[string]any{
					"resources": map[string]any{
						"limits": map[string]any{
							"cpu": "100m",
						},
					},
				},
			},
			sources: []Values{
				{
					"deployment": map[string]any{
						"resources": map[string]any{
							"limits": map[string]any{
								"memory": "128Mi",
							},
						},
					},
				},
			},
			expect: Values{
				"deployment": map[string]any{
					"resources": map[string]any{
						"limits": map[string]any{
							"cpu":    "100m",
							"memory": "128Mi",
						},
					},
				},
			},
		},
		{
			name: "non-map values overwrite",
			dst: Values{
				"key": map[string]any{
					"nested": "value",
				},
			},
			sources: []Values{
				{"key": "string-value"},
			},
			expect: Values{"key": "string-value"},
		},
		{
			name:    "nil source is skipped",
			dst:     Values{"key1": "value1"},
			sources: []Values{nil, {"key2": "value2"}},
			expect:  Values{"key1": "value1", "key2": "value2"},
		},
		{
			name:    "empty sources list",
			dst:     Values{"key": "value"},
			sources: []Values{},
			expect:  Values{"key": "value"},
		},
		{
			name: "complex merge with precedence",
			dst:  Values{"replicas": 1},
			sources: []Values{
				{
					"replicas": 2,
					"image": map[string]any{
						"tag": "v1.0",
					},
				},
				{
					"replicas": 3,
					"service": map[string]any{
						"type": "LoadBalancer",
					},
				},
			},
			expect: Values{
				"replicas": 3,
				"image": map[string]any{
					"tag": "v1.0",
				},
				"service": map[string]any{
					"type": "LoadBalancer",
				},
			},
		},
		{
			name: "slice values are unioned (no duplicates)",
			dst: Values{
				"items": []any{"item1", "item2"},
			},
			sources: []Values{
				{"items": []any{"item3", "item4"}},
			},
			expect: Values{
				"items": []any{"item1", "item2", "item3", "item4"},
			},
		},
		{
			name: "slice union removes duplicates",
			dst: Values{
				"items": []any{"item1", "item2"},
			},
			sources: []Values{
				{"items": []any{"item2", "item3"}},
			},
			expect: Values{
				"items": []any{"item1", "item2", "item3"},
			},
		},
		{
			name: "slice union with complex types",
			dst: Values{
				"configs": []any{
					map[string]any{"name": "config1", "value": "val1"},
				},
			},
			sources: []Values{
				{
					"configs": []any{
						map[string]any{"name": "config2", "value": "val2"},
						map[string]any{"name": "config1", "value": "val1"}, // duplicate
					},
				},
			},
			expect: Values{
				"configs": []any{
					map[string]any{"name": "config1", "value": "val1"},
					map[string]any{"name": "config2", "value": "val2"},
				},
			},
		},
		{
			name: "slice can be added when key doesn't exist",
			dst: Values{
				"existing": "value",
			},
			sources: []Values{
				{"items": []any{"item1", "item2"}},
			},
			expect: Values{
				"existing": "value",
				"items":    []any{"item1", "item2"},
			},
		},
		{
			name: "map overwrites existing slice",
			dst: Values{
				"data": []any{"value1", "value2"},
			},
			sources: []Values{
				{
					"data": map[string]any{
						"key": "value",
					},
				},
			},
			expect: Values{
				"data": map[string]any{
					"key": "value",
				},
			},
		},
		{
			name: "union with empty source slice",
			dst: Values{
				"items": []any{"item1", "item2"},
			},
			sources: []Values{
				{"items": []any{}},
			},
			expect: Values{
				"items": []any{"item1", "item2"},
			},
		},
		{
			name: "union with empty destination slice",
			dst: Values{
				"items": []any{},
			},
			sources: []Values{
				{"items": []any{"item1", "item2"}},
			},
			expect: Values{
				"items": []any{"item1", "item2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.dst.Merge(tt.sources...)
			require.Equal(t, tt.expect, tt.dst)
		})
	}
}

func TestMerge_NilReceiver(t *testing.T) {
	t.Parallel()

	var nilValues Values
	// Should not panic when calling Merge on nil receiver
	nilValues.Merge(Values{"key": "value"})
	require.Nil(t, nilValues)
}

func TestFill(t *testing.T) {
	tests := []struct {
		name    string
		dst     Values
		sources []Values
		expect  Values
	}{
		{
			name:    "fill single map with new keys",
			dst:     Values{"key1": "value1"},
			sources: []Values{{"key2": "value2"}},
			expect:  Values{"key1": "value1", "key2": "value2"},
		},
		{
			name: "fill multiple maps with new keys",
			dst:  Values{"key1": "value1"},
			sources: []Values{
				{"key2": "value2"},
				{"key3": "value3"},
			},
			expect: Values{"key1": "value1", "key2": "value2", "key3": "value3"},
		},
		{
			name: "existing values are NOT overwritten",
			dst:  Values{"key": "original"},
			sources: []Values{
				{"key": "first"},
				{"key": "second"},
			},
			expect: Values{"key": "original"},
		},
		{
			name: "nested maps fill recursively without overwriting",
			dst: Values{
				"app": map[string]any{
					"name": "myapp",
				},
			},
			sources: []Values{
				{
					"app": map[string]any{
						"version": "1.0",
					},
				},
			},
			expect: Values{
				"app": map[string]any{
					"name":    "myapp",
					"version": "1.0",
				},
			},
		},
		{
			name: "deeply nested maps fill without overwriting",
			dst: Values{
				"deployment": map[string]any{
					"resources": map[string]any{
						"limits": map[string]any{
							"cpu": "100m",
						},
					},
				},
			},
			sources: []Values{
				{
					"deployment": map[string]any{
						"resources": map[string]any{
							"limits": map[string]any{
								"memory": "128Mi",
							},
						},
					},
				},
			},
			expect: Values{
				"deployment": map[string]any{
					"resources": map[string]any{
						"limits": map[string]any{
							"cpu":    "100m",
							"memory": "128Mi",
						},
					},
				},
			},
		},
		{
			name: "nested values in dst are preserved over source",
			dst: Values{
				"deployment": map[string]any{
					"resources": map[string]any{
						"limits": map[string]any{
							"cpu":    "200m",
							"memory": "256Mi",
						},
					},
				},
			},
			sources: []Values{
				{
					"deployment": map[string]any{
						"resources": map[string]any{
							"limits": map[string]any{
								"cpu":    "100m",
								"memory": "128Mi",
							},
							"requests": map[string]any{
								"cpu": "50m",
							},
						},
					},
				},
			},
			expect: Values{
				"deployment": map[string]any{
					"resources": map[string]any{
						"limits": map[string]any{
							"cpu":    "200m",
							"memory": "256Mi",
						},
						"requests": map[string]any{
							"cpu": "50m",
						},
					},
				},
			},
		},
		{
			name: "non-map values do NOT overwrite",
			dst: Values{
				"key": map[string]any{
					"nested": "value",
				},
			},
			sources: []Values{
				{"key": "string-value"},
			},
			expect: Values{
				"key": map[string]any{
					"nested": "value",
				},
			},
		},
		{
			name:    "nil source is skipped",
			dst:     Values{"key1": "value1"},
			sources: []Values{nil, {"key2": "value2"}},
			expect:  Values{"key1": "value1", "key2": "value2"},
		},
		{
			name:    "empty sources list",
			dst:     Values{"key": "value"},
			sources: []Values{},
			expect:  Values{"key": "value"},
		},
		{
			name: "later sources fill gaps earlier sources couldn't",
			dst:  Values{"key1": "value1"},
			sources: []Values{
				{
					"key2": "from-first",
					"key3": "from-first",
				},
				{
					"key3": "from-second",
					"key4": "from-second",
				},
			},
			expect: Values{
				"key1": "value1",
				"key2": "from-first",
				"key3": "from-first",
				"key4": "from-second",
			},
		},
		{
			name: "partial overlap with nested structures",
			dst: Values{
				"app": map[string]any{
					"name": "myapp",
				},
				"replicas": 5,
			},
			sources: []Values{
				{
					"app": map[string]any{
						"name":    "other-app",
						"version": "1.0",
					},
					"replicas": 3,
					"service": map[string]any{
						"type": "LoadBalancer",
					},
				},
			},
			expect: Values{
				"app": map[string]any{
					"name":    "myapp",
					"version": "1.0",
				},
				"replicas": 5,
				"service": map[string]any{
					"type": "LoadBalancer",
				},
			},
		},
		{
			name: "slice values are unioned in Fill",
			dst: Values{
				"items": []any{"item1", "item2"},
			},
			sources: []Values{
				{"items": []any{"item3", "item4"}},
			},
			expect: Values{
				"items": []any{"item1", "item2", "item3", "item4"},
			},
		},
		{
			name: "slice union in Fill removes duplicates",
			dst: Values{
				"items": []any{"item1", "item2"},
			},
			sources: []Values{
				{"items": []any{"item2", "item3"}},
			},
			expect: Values{
				"items": []any{"item1", "item2", "item3"},
			},
		},
		{
			name: "slice can be added to dst when key doesn't exist",
			dst: Values{
				"existing": "value",
			},
			sources: []Values{
				{"items": []any{"item1", "item2"}},
			},
			expect: Values{
				"existing": "value",
				"items":    []any{"item1", "item2"},
			},
		},
		{
			name: "map does not overwrite existing slice in Fill",
			dst: Values{
				"data": []any{"value1", "value2"},
			},
			sources: []Values{
				{
					"data": map[string]any{
						"key": "value",
					},
				},
			},
			expect: Values{
				"data": []any{"value1", "value2"},
			},
		},
		{
			name: "union with empty source slice",
			dst: Values{
				"items": []any{"item1", "item2"},
			},
			sources: []Values{
				{"items": []any{}},
			},
			expect: Values{
				"items": []any{"item1", "item2"},
			},
		},
		{
			name: "union with empty destination slice",
			dst: Values{
				"items": []any{},
			},
			sources: []Values{
				{"items": []any{"item1", "item2"}},
			},
			expect: Values{
				"items": []any{"item1", "item2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.dst.Fill(tt.sources...)
			require.Equal(t, tt.expect, tt.dst)
		})
	}
}

func TestFill_NilReceiver(t *testing.T) {
	t.Parallel()

	var nilValues Values
	// Should not panic when calling Fill on nil receiver
	nilValues.Fill(Values{"key": "value"})
	require.Nil(t, nilValues)
}
