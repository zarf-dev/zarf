// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package template provides functions for applying go-templates within Zarf.
package template

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/value"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

func TestNewObjects(t *testing.T) {
	values := value.Values{
		"app": map[string]any{
			"name": "test-app",
			"port": 8080,
		},
	}

	objects := NewObjects(values)

	require.Contains(t, objects, objectKeyValues)
	require.Equal(t, values, objects[objectKeyValues])
}

func TestObjects_WithValues(t *testing.T) {
	tests := []struct {
		name     string
		values   value.Values
		expected value.Values
	}{
		{
			name: "basic values",
			values: value.Values{
				"database": map[string]any{
					"host": "localhost",
					"port": 5432,
				},
			},
			expected: value.Values{
				"database": map[string]any{
					"host": "localhost",
					"port": 5432,
				},
			},
		},
		{
			name: "complex nested values",
			values: value.Values{
				"app": map[string]any{
					"name": "test-app",
					"config": map[string]any{
						"database": map[string]any{
							"host": "db.example.com",
							"port": 3306,
						},
						"cache": map[string]any{
							"enabled": true,
							"ttl":     300,
						},
					},
				},
			},
			expected: value.Values{
				"app": map[string]any{
					"name": "test-app",
					"config": map[string]any{
						"database": map[string]any{
							"host": "db.example.com",
							"port": 3306,
						},
						"cache": map[string]any{
							"enabled": true,
							"ttl":     300,
						},
					},
				},
			},
		},
		{
			name:     "empty values",
			values:   value.Values{},
			expected: value.Values{},
		},
		{
			name: "single string value",
			values: value.Values{
				"message": "hello world",
			},
			expected: value.Values{
				"message": "hello world",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make(Objects)
			result := objects.WithValues(tt.values)
			require.Equal(t, tt.expected, result[objectKeyValues])
		})
	}
}

func TestObjects_WithMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata v1alpha1.ZarfMetadata
		expected v1alpha1.ZarfMetadata
	}{
		{
			name: "complete metadata",
			metadata: v1alpha1.ZarfMetadata{
				Name:        "test-package",
				Description: "A test package",
				Version:     "1.0.0",
			},
			expected: v1alpha1.ZarfMetadata{
				Name:        "test-package",
				Description: "A test package",
				Version:     "1.0.0",
			},
		},
		{
			name: "minimal metadata",
			metadata: v1alpha1.ZarfMetadata{
				Name: "minimal-package",
			},
			expected: v1alpha1.ZarfMetadata{
				Name: "minimal-package",
			},
		},
		{
			name: "metadata with URL and author",
			metadata: v1alpha1.ZarfMetadata{
				Name:        "example-package",
				Description: "An example package for testing",
				Version:     "2.1.0",
				URL:         "https://example.com",
				Authors:     "Test Author <test@example.com>",
			},
			expected: v1alpha1.ZarfMetadata{
				Name:        "example-package",
				Description: "An example package for testing",
				Version:     "2.1.0",
				URL:         "https://example.com",
				Authors:     "Test Author <test@example.com>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make(Objects)
			result := objects.WithMetadata(tt.metadata)
			require.Equal(t, tt.expected, result[objectKeyMetadata])
		})
	}
}

func TestObjects_WithBuild(t *testing.T) {
	tests := []struct {
		name     string
		build    v1alpha1.ZarfBuildData
		expected v1alpha1.ZarfBuildData
	}{
		{
			name: "complete build data",
			build: v1alpha1.ZarfBuildData{
				User:         "test-user",
				Architecture: "amd64",
				Timestamp:    "2023-01-01T00:00:00Z",
			},
			expected: v1alpha1.ZarfBuildData{
				User:         "test-user",
				Architecture: "amd64",
				Timestamp:    "2023-01-01T00:00:00Z",
			},
		},
		{
			name: "minimal build data",
			build: v1alpha1.ZarfBuildData{
				User: "minimal-user",
			},
			expected: v1alpha1.ZarfBuildData{
				User: "minimal-user",
			},
		},
		{
			name: "arm64 architecture",
			build: v1alpha1.ZarfBuildData{
				User:         "arm-user",
				Architecture: "arm64",
				Timestamp:    "2023-12-01T10:30:00Z",
			},
			expected: v1alpha1.ZarfBuildData{
				User:         "arm-user",
				Architecture: "arm64",
				Timestamp:    "2023-12-01T10:30:00Z",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make(Objects)
			result := objects.WithBuild(tt.build)
			require.Equal(t, tt.expected, result[objectKeyBuild])
		})
	}
}

func TestObjects_WithConstants(t *testing.T) {
	tests := []struct {
		name      string
		constants []v1alpha1.Constant
		expected  map[string]string
	}{
		{
			name: "multiple constants",
			constants: []v1alpha1.Constant{
				{Name: "APP_NAME", Value: "my-app"},
				{Name: "VERSION", Value: "1.2.3"},
				{Name: "NAMESPACE", Value: "default"},
			},
			expected: map[string]string{
				"APP_NAME":  "my-app",
				"VERSION":   "1.2.3",
				"NAMESPACE": "default",
			},
		},
		{
			name:      "empty constants",
			constants: nil,
			expected:  map[string]string{},
		},
		{
			name: "single constant",
			constants: []v1alpha1.Constant{
				{Name: "ENVIRONMENT", Value: "production"},
			},
			expected: map[string]string{
				"ENVIRONMENT": "production",
			},
		},
		{
			name: "constants with special characters",
			constants: []v1alpha1.Constant{
				{Name: "DB_URL", Value: "postgresql://user:pass@localhost:5432/db"},
				{Name: "API_KEY", Value: "abc123-def456-ghi789"},
			},
			expected: map[string]string{
				"DB_URL":  "postgresql://user:pass@localhost:5432/db",
				"API_KEY": "abc123-def456-ghi789",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make(Objects)
			result := objects.WithConstants(tt.constants)
			require.Equal(t, tt.expected, result[objectKeyConstants])
		})
	}
}

func TestObjects_WithVariables(t *testing.T) {
	tests := []struct {
		name      string
		variables variables.SetVariableMap
		expected  map[string]string
	}{
		{
			name: "multiple variables",
			variables: variables.SetVariableMap{
				"DB_HOST": {Value: "localhost"},
				"DB_PORT": {Value: "5432"},
				"DB_NAME": {Value: "testdb"},
			},
			expected: map[string]string{
				"DB_HOST": "localhost",
				"DB_PORT": "5432",
				"DB_NAME": "testdb",
			},
		},
		{
			name:      "empty variables",
			variables: variables.SetVariableMap{},
			expected:  map[string]string{},
		},
		{
			name: "single variable",
			variables: variables.SetVariableMap{
				"SERVICE_NAME": {Value: "web-service"},
			},
			expected: map[string]string{
				"SERVICE_NAME": "web-service",
			},
		},
		{
			name: "variables with complex values",
			variables: variables.SetVariableMap{
				"CONFIG_JSON": {Value: `{"enabled": true, "timeout": 30}`},
				"SECRET_KEY":  {Value: "very-secret-key-123"},
			},
			expected: map[string]string{
				"CONFIG_JSON": `{"enabled": true, "timeout": 30}`,
				"SECRET_KEY":  "very-secret-key-123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make(Objects)
			result := objects.WithVariables(tt.variables)
			require.Equal(t, tt.expected, result[objectKeyVariables])
		})
	}
}

func TestApply(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		objects  Objects
		expected string
		wantErr  bool
	}{
		{
			name: "simple variable substitution",
			s:    "echo {{ .Values.app.name }}",
			objects: Objects{
				objectKeyValues: value.Values{
					"app": map[string]any{"name": "test-app"},
				},
			},
			expected: "echo test-app",
		},
		{
			name: "multiple variables",
			s:    "kubectl create deployment {{ .Values.app.name }} --image={{ .Values.app.image }}:{{ .Values.app.tag }}",
			objects: Objects{
				objectKeyValues: value.Values{
					"app": map[string]any{
						"name":  "my-app",
						"image": "nginx",
						"tag":   "latest",
					},
				},
			},
			expected: "kubectl create deployment my-app --image=nginx:latest",
		},
		{
			name: "using constants",
			s:    "echo {{ .Constants.NAMESPACE }}",
			objects: Objects{
				objectKeyConstants: map[string]string{
					"NAMESPACE": "production",
				},
			},
			expected: "echo production",
		},
		{
			name: "using variables",
			s:    "connect to {{ .Variables.DB_HOST }}:{{ .Variables.DB_PORT }}",
			objects: Objects{
				objectKeyVariables: map[string]string{
					"DB_HOST": "localhost",
					"DB_PORT": "5432",
				},
			},
			expected: "connect to localhost:5432",
		},
		{
			name: "sprig functions",
			s:    "echo {{ .Values.name | upper }}",
			objects: Objects{
				objectKeyValues: value.Values{
					"name": "hello world",
				},
			},
			expected: "echo HELLO WORLD",
		},
		{
			name:    "invalid template syntax",
			s:       "echo {{ .Values.missing",
			objects: Objects{},
			wantErr: true,
		},
		{
			name: "missing field returns error",
			s:    "echo {{ .Values.missing.field }}",
			objects: Objects{
				objectKeyValues: value.Values{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := Apply(ctx, tt.s, tt.objects)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestApplyToFile(t *testing.T) {
	tests := []struct {
		name     string
		testCase string
		objects  Objects
	}{
		{
			name:     "simple file template",
			testCase: "configmap",
			objects: Objects{
				objectKeyValues: value.Values{
					"app": map[string]any{
						"name": "my-config",
						"port": 8080,
					},
				},
			},
		},
		{
			name:     "using constants in file",
			testCase: "constants",
			objects: Objects{
				objectKeyConstants: map[string]string{
					"NAMESPACE": "production",
					"VERSION":   "1.0.0",
				},
			},
		},
		{
			name:     "complex nested values in file",
			testCase: "deployment",
			objects: Objects{
				objectKeyValues: value.Values{
					"app": map[string]any{
						"name":      "my-deployment",
						"namespace": "production",
						"replicas":  3,
					},
				},
			},
		},
		{
			name:     "basic sprig functions",
			testCase: "sprig-basic",
			objects: Objects{
				objectKeyValues: value.Values{
					"name":    "hello",
					"message": "hello world",
					"numbers": []int{1, 2, 3},
					"text":    "hello",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tempDir := t.TempDir()

			// Build file paths
			inputFile := filepath.Join("testdata", tt.testCase, "input.yaml")
			expectedFile := filepath.Join("testdata", tt.testCase, "expected.yaml")
			dstFile := filepath.Join(tempDir, "output.yaml")

			// Apply template using testdata file
			err := ApplyToFile(ctx, inputFile, dstFile, tt.objects)
			require.NoError(t, err)

			// Read expected output
			expectedContent, err := os.ReadFile(expectedFile)
			require.NoError(t, err)

			// Read actual output and compare
			result, err := os.ReadFile(dstFile)
			require.NoError(t, err)
			require.Equal(t, string(expectedContent), string(result))
		})
	}
}

func TestApplyToFile_Errors(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	tests := []struct {
		name    string
		srcFile string
		dstFile string
		objects Objects
	}{
		{
			name:    "source file does not exist",
			srcFile: filepath.Join(tempDir, "nonexistent.yaml"),
			dstFile: filepath.Join(tempDir, "output.yaml"),
			objects: Objects{},
		},
		{
			name:    "destination directory does not exist",
			srcFile: filepath.Join(tempDir, "template.yaml"),
			dstFile: "/nonexistent/dir/output.yaml",
			objects: Objects{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create source file if the test isn't about missing source
			if tt.name != "source file does not exist" {
				err := os.WriteFile(tt.srcFile, []byte("test content"), 0644)
				require.NoError(t, err)
			}

			err := ApplyToFile(ctx, tt.srcFile, tt.dstFile, tt.objects)
			require.Error(t, err)
		})
	}
}

func TestApplyToFile_InvalidTemplate(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// Use testdata file with invalid template syntax
	srcFile := filepath.Join("testdata", "invalid", "input.yaml")
	dstFile := filepath.Join(tempDir, "output.yaml")

	err := ApplyToFile(ctx, srcFile, dstFile, Objects{})
	require.Error(t, err)
}

func TestApplyToFile_SprigFunctions(t *testing.T) {
	tests := []struct {
		name     string
		testCase string
		objects  Objects
	}{
		{
			name:     "basic sprig functions - string manipulation",
			testCase: "sprig-basic",
			objects: Objects{
				objectKeyValues: value.Values{
					"name":    "hello",
					"message": "hello world",
					"numbers": []int{1, 2, 3},
					"text":    "hello",
				},
			},
		},
		{
			name:     "advanced sprig functions",
			testCase: "sprig-functions",
			objects: Objects{
				objectKeyValues: value.Values{
					"app": map[string]any{
						"name": "MyTestApp",
					},
					"env":     "production",
					"version": "1.0.0",
					"message": "hello world",
					"data":    "some data",
					"items":   []string{"apple", "banana", "cherry"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tempDir := t.TempDir()

			// Build file paths
			inputFile := filepath.Join("testdata", tt.testCase, "input.yaml")
			expectedFile := filepath.Join("testdata", tt.testCase, "expected.yaml")
			dstFile := filepath.Join(tempDir, "output.yaml")

			// Apply template using testdata file
			err := ApplyToFile(ctx, inputFile, dstFile, tt.objects)
			require.NoError(t, err)

			// Read expected output
			expectedContent, err := os.ReadFile(expectedFile)
			require.NoError(t, err)

			// Read actual output and compare
			result, err := os.ReadFile(dstFile)
			require.NoError(t, err)

			// For the advanced sprig functions test, we'll need to handle non-deterministic functions
			if tt.testCase == "sprig-functions" {
				// Just verify that the file was processed without error and contains expected structure
				// We can't do exact comparison due to random/time functions
				require.Contains(t, string(result), "my-test-app")         // kebabcase of MyTestApp
				require.Contains(t, string(result), "PRODUCTION")          // upper of production
				require.Contains(t, string(result), "Hello World")         // title of hello world
				require.Contains(t, string(result), "apple,banana,cherry") // join
				require.Contains(t, string(result), "1.0.0")               // default uses actual value
			} else {
				require.Equal(t, string(expectedContent), string(result))
			}
		})
	}
}

func TestToYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "simple map",
			input:    map[string]any{"name": "test", "value": 123},
			expected: "name: test\nvalue: 123",
		},
		{
			name:     "nested map",
			input:    map[string]any{"app": map[string]any{"name": "test", "port": 8080}},
			expected: "app:\n  name: test\n  port: 8080",
		},
		{
			name:     "array",
			input:    []string{"one", "two", "three"},
			expected: "- one\n- two\n- three",
		},
		{
			name:     "string",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "number",
			input:    42,
			expected: "42",
		},
		{
			name:     "boolean",
			input:    true,
			expected: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toYAML(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestMustToYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "simple map",
			input:    map[string]any{"name": "test", "value": 123},
			expected: "name: test\nvalue: 123",
		},
		{
			name:     "nested map",
			input:    map[string]any{"app": map[string]any{"name": "test"}},
			expected: "app:\n  name: test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mustToYAML(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestToYAMLPretty(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "simple map",
			input:    map[string]any{"name": "test", "value": 123},
			expected: "name: test\nvalue: 123",
		},
		{
			name: "nested map",
			input: map[string]any{
				"app": map[string]any{
					"name": "test",
					"config": map[string]any{
						"port":    8080,
						"enabled": true,
					},
				},
			},
			expected: "app:\n  name: test\n  config:\n    port: 8080\n    enabled: true",
		},
		{
			name:     "array",
			input:    []string{"one", "two", "three"},
			expected: "- one\n- two\n- three",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toYAMLPretty(tt.input)
			// Use YAMLEq to compare YAML semantically (ignores key order)
			require.YAMLEq(t, tt.expected, result)
		})
	}
}

func TestFromYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]interface{}
	}{
		{
			name:     "simple map",
			input:    "name: test\nvalue: 123",
			expected: map[string]interface{}{"name": "test", "value": uint64(123)},
		},
		{
			name:  "nested map",
			input: "app:\n  name: test\n  port: 8080",
			expected: map[string]interface{}{
				"app": map[string]interface{}{"name": "test", "port": uint64(8080)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromYAML(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFromYAML_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "invalid yaml syntax",
			input: "invalid: yaml: with: bad: indentation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromYAML(tt.input)
			errMsg, ok := result["Error"]
			require.True(t, ok, "expected Error key in result map")
			require.Contains(t, errMsg, "yaml:")
		})
	}
}

func TestFromYAMLArray(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []interface{}
	}{
		{
			name:     "simple array",
			input:    "- one\n- two\n- three",
			expected: []interface{}{"one", "two", "three"},
		},
		{
			name:     "number array",
			input:    "- 1\n- 2\n- 3",
			expected: []interface{}{uint64(1), uint64(2), uint64(3)},
		},
		{
			name:     "mixed array",
			input:    "- 1\n- two\n- true",
			expected: []interface{}{uint64(1), "two", true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromYAMLArray(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFromYAMLArray_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "not an array",
			input: "invalid: not an array",
		},
		{
			name:  "object instead of array",
			input: "key: value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromYAMLArray(tt.input)
			require.Len(t, result, 1, "expected single error message in array")
			errMsg, ok := result[0].(string)
			require.True(t, ok, "expected error message to be a string")
			require.NotEmpty(t, errMsg, "expected non-empty error message")
		})
	}
}

func TestToJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "simple map",
			input:    map[string]any{"name": "test", "value": 123},
			expected: `{"name":"test","value":123}`,
		},
		{
			name:     "nested map",
			input:    map[string]any{"app": map[string]any{"name": "test"}},
			expected: `{"app":{"name":"test"}}`,
		},
		{
			name:     "array",
			input:    []string{"one", "two", "three"},
			expected: `["one","two","three"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toJSON(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestMustToJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "simple map",
			input:    map[string]any{"name": "test"},
			expected: `{"name":"test"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mustToJSON(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFromJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]interface{}
	}{
		{
			name:     "simple object",
			input:    `{"name":"test","value":123}`,
			expected: map[string]interface{}{"name": "test", "value": float64(123)},
		},
		{
			name:  "nested object",
			input: `{"app":{"name":"test"}}`,
			expected: map[string]interface{}{
				"app": map[string]interface{}{"name": "test"},
			},
		},
		{
			name:     "object with array",
			input:    `{"items":["a","b","c"]}`,
			expected: map[string]interface{}{"items": []interface{}{"a", "b", "c"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromJSON(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFromJSON_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "invalid json syntax",
			input: `{invalid json}`,
		},
		{
			name:  "unclosed brace",
			input: `{"key":"value"`,
		},
		{
			name:  "trailing comma",
			input: `{"key":"value",}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromJSON(tt.input)
			errMsg, ok := result["Error"]
			require.True(t, ok, "expected Error key in result map")
			require.NotEmpty(t, errMsg)
		})
	}
}

func TestFromJSONArray(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []interface{}
	}{
		{
			name:     "simple array",
			input:    `["one","two","three"]`,
			expected: []interface{}{"one", "two", "three"},
		},
		{
			name:     "number array",
			input:    `[1,2,3]`,
			expected: []interface{}{float64(1), float64(2), float64(3)},
		},
		{
			name:     "mixed type array",
			input:    `[1,"two",true,null]`,
			expected: []interface{}{float64(1), "two", true, nil},
		},
		{
			name:     "nested arrays",
			input:    `[[1,2],[3,4]]`,
			expected: []interface{}{[]interface{}{float64(1), float64(2)}, []interface{}{float64(3), float64(4)}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromJSONArray(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFromJSONArray_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "object instead of array",
			input: `{not an array}`,
		},
		{
			name:  "unclosed bracket",
			input: `["one","two"`,
		},
		{
			name:  "invalid json",
			input: `[1,2,]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromJSONArray(tt.input)
			require.Len(t, result, 1, "expected single error message in array")
			errMsg, ok := result[0].(string)
			require.True(t, ok, "expected error message to be a string")
			require.NotEmpty(t, errMsg)
		})
	}
}

func TestToTOML(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "simple map",
			input:    map[string]any{"name": "test", "value": 123},
			expected: "name = \"test\"\nvalue = 123\n",
		},
		{
			name: "nested map",
			input: map[string]any{
				"database": map[string]any{
					"host": "localhost",
					"port": 5432,
				},
			},
			expected: "[database]\n  host = \"localhost\"\n  port = 5432\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toTOML(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFromTOML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]interface{}
	}{
		{
			name:     "simple map",
			input:    "name = \"test\"\nvalue = 123",
			expected: map[string]interface{}{"name": "test", "value": int64(123)},
		},
		{
			name:  "nested map",
			input: "[database]\nhost = \"localhost\"\nport = 5432",
			expected: map[string]interface{}{
				"database": map[string]interface{}{
					"host": "localhost",
					"port": int64(5432),
				},
			},
		},
		{
			name:  "array in toml",
			input: "items = [\"a\", \"b\", \"c\"]",
			expected: map[string]interface{}{
				"items": []interface{}{"a", "b", "c"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromTOML(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFromTOML_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "invalid toml syntax",
			input: "invalid toml content",
		},
		{
			name:  "missing equals",
			input: "key value",
		},
		{
			name:  "unclosed quote",
			input: "name = \"test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromTOML(tt.input)
			errMsg, ok := result["Error"]
			require.True(t, ok, "expected Error key in result map")
			require.Contains(t, errMsg, "toml:")
		})
	}
}
