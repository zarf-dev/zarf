// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package template provides functions for applying go-templates within Zarf.
package template

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/value"
	"github.com/zarf-dev/zarf/src/pkg/pki"
	"github.com/zarf-dev/zarf/src/pkg/state"
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

func TestObjects_WithState(t *testing.T) {
	tests := []struct {
		name     string
		state    *state.State
		expected map[string]any
	}{
		{
			name: "complete state",
			state: &state.State{
				StorageClass: "standard",
				RegistryInfo: state.RegistryInfo{
					Address:      "registry.example.com",
					NodePort:     30000,
					PushPassword: "push-secret",
					PullPassword: "pull-secret",
				},
				GitServer: state.GitServerInfo{
					PushUsername: "git-push-user",
					PushPassword: "git-push-secret",
					PullUsername: "git-pull-user",
					PullPassword: "git-pull-secret",
				},
			},
			expected: map[string]any{
				"storage": map[string]any{
					"class": "standard",
				},
				"registry": map[string]any{
					"address":  "registry.example.com",
					"nodePort": 30000,
					"push": map[string]any{
						"password": "push-secret",
					},
					"pull": map[string]any{
						"password": "pull-secret",
					},
				},
				"git": map[string]any{
					"push": map[string]any{
						"username": "git-push-user",
						"password": "git-push-secret",
					},
					"pull": map[string]any{
						"username": "git-pull-user",
						"password": "git-pull-secret",
					},
				},
			},
		},
		{
			name: "minimal state",
			state: &state.State{
				StorageClass: "fast-storage",
			},
			expected: map[string]any{
				"storage": map[string]any{
					"class": "fast-storage",
				},
				"registry": map[string]any{
					"address":  "",
					"nodePort": 0,
					"push": map[string]any{
						"password": "",
					},
					"pull": map[string]any{
						"password": "",
					},
				},
				"git": map[string]any{
					"push": map[string]any{
						"username": "",
						"password": "",
					},
					"pull": map[string]any{
						"username": "",
						"password": "",
					},
				},
			},
		},
		{
			name:     "nil state returns unchanged",
			state:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make(Objects)
			result := objects.WithState(tt.state)

			if tt.expected == nil {
				require.NotContains(t, result, objectKeyState)
				return
			}

			require.Contains(t, result, objectKeyState)
			require.Equal(t, tt.expected, result[objectKeyState])
		})
	}
}

func TestObjects_WithAgentState(t *testing.T) {
	caCert := []byte("test-ca-cert")
	cert := []byte("test-cert")
	key := []byte("test-key")

	tests := []struct {
		name     string
		state    *state.State
		expected map[string]any
	}{
		{
			name: "with agent TLS",
			state: &state.State{
				AgentTLS: pki.GeneratedPKI{
					CA:   caCert,
					Cert: cert,
					Key:  key,
				},
			},
			expected: map[string]any{
				"agent": map[string]any{
					"tls": map[string]any{
						"ca":   base64.StdEncoding.EncodeToString(caCert),
						"cert": base64.StdEncoding.EncodeToString(cert),
						"key":  base64.StdEncoding.EncodeToString(key),
					},
				},
			},
		},
		{
			name:     "nil state returns unchanged",
			state:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make(Objects)
			result := objects.WithAgentState(tt.state)

			if tt.expected == nil {
				require.NotContains(t, result, objectKeyState)
				return
			}

			require.Contains(t, result, objectKeyState)
			require.Equal(t, tt.expected, result[objectKeyState])
		})
	}
}

func TestObjects_WithSeedRegistryState(t *testing.T) {
	tests := []struct {
		name  string
		state *state.State
	}{
		{
			name: "with internal registry",
			state: &state.State{
				RegistryInfo: state.RegistryInfo{
					Address:      "127.0.0.1:31999",
					NodePort:     31999,
					PushUsername: "zarf-push",
					PushPassword: "push-pass",
					PullUsername: "zarf-pull",
					PullPassword: "pull-pass",
					Secret:       "registry-secret-value",
				},
			},
		},
		{
			name:  "nil state returns unchanged",
			state: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make(Objects)
			result := objects.WithSeedRegistryState(tt.state)

			if tt.state == nil {
				require.NotContains(t, result, objectKeyState)
				return
			}

			require.Contains(t, result, objectKeyState)
			stateMap := result[objectKeyState].(map[string]any)
			registryMap := stateMap["registry"].(map[string]any)

			// Verify expected fields exist
			require.Contains(t, registryMap, "htpasswd")
			require.Contains(t, registryMap, "seed")
			require.Contains(t, registryMap, "secret")

			// Verify htpasswd has bcrypt format
			require.NotEmpty(t, registryMap["htpasswd"])
			require.Contains(t, registryMap["htpasswd"].(string), "$2")
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
