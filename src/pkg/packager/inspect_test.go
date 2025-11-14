// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
package packager

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/internal/feature"
	"github.com/zarf-dev/zarf/src/internal/value"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

var testDataRoot = "testdata/inspect"

func setupInspectTests(t *testing.T) {
	t.Helper()
	lint.ZarfSchema = testutil.LoadSchema(t, "../../../zarf.schema.json")
	// Enable values feature for tests (ignore error if already set)
	_ = feature.Set([]feature.Feature{{Name: feature.Values, Enabled: true}}) //nolint:errcheck
}

func inspectTestDataPath(parts ...string) string {
	return filepath.Join(append([]string{testDataRoot}, parts...)...)
}

func assertContainsAll(t *testing.T, resources []Resource, expectedContent []string) {
	t.Helper()
	// Combine all resource content for validation
	var allContent string
	for _, r := range resources {
		allContent += r.Content
	}
	// Verify all expected content is present
	for _, expected := range expectedContent {
		require.Contains(t, allContent, expected)
	}
}

func TestInspectDefinitionResources(t *testing.T) {
	t.Parallel()
	setupInspectTests(t)

	tests := []struct {
		name              string
		packageDir        string
		opts              InspectDefinitionResourcesOptions
		expectedResources int
		expectedContent   []string
	}{
		{
			name:       "chart with helm values from Values parameter",
			packageDir: inspectTestDataPath("chart-with-helm-values"),
			opts: InspectDefinitionResourcesOptions{
				DeploySetVariables: map[string]string{
					"REPLICAS": "3",
				},
				Values: value.Values{
					"customField": "fromAPI",
					"image": map[string]any{
						"pullPolicy": "Always",
					},
					"port": 9090,
				},
			},
			expectedResources: 2, // 1 chart resource + 1 values file resource
			expectedContent: []string{
				"customField: fromAPI",
				"pullPolicy: Always",
				"port: 9090",
				"replicaCount: \"3\"",
			},
		},
		{
			name:       "manifest with values using Go templates",
			packageDir: inspectTestDataPath("manifest-with-values"),
			opts: InspectDefinitionResourcesOptions{
				Values: value.Values{
					"replicas": 5,
					"imageTag": "latest",
					"port":     8080,
				},
			},
			expectedResources: 1, // 1 manifest resource
			expectedContent: []string{
				"replicas: 5",
				"image: httpd:latest",
				"containerPort: 8080",
			},
		},
		{
			name:       "values from variables only (no CLI values)",
			packageDir: inspectTestDataPath("chart"),
			opts: InspectDefinitionResourcesOptions{
				DeploySetVariables: map[string]string{
					"REPLICAS": "2",
				},
				Values: value.Values{}, // Empty values
			},
			expectedResources: 2, // 1 chart resource + 1 values file
			expectedContent: []string{
				"replicaCount: \"2\"",
			},
		},
		{
			name:              "manifest with package-level default values",
			packageDir:        inspectTestDataPath("manifest-with-package-values"),
			opts:              InspectDefinitionResourcesOptions{},
			expectedResources: 1, // 1 manifest resource
			expectedContent: []string{
				"name: my-httpd-app",
				"replicas: 3",
				"image: httpd:2.4",
				"containerPort: 80",
			},
		},
		{
			name:       "manifest with package values overridden by CLI",
			packageDir: inspectTestDataPath("manifest-with-package-values"),
			opts: InspectDefinitionResourcesOptions{
				Values: value.Values{
					"app": map[string]any{
						"name":     "overridden-app",
						"replicas": 5,
						"image": map[string]any{
							"repository": "nginx",
							"tag":        "latest",
						},
						"port": 8080,
					},
				},
			},
			expectedResources: 1,
			expectedContent: []string{
				"name: overridden-app",
				"replicas: 5",
				"image: nginx:latest",
				"containerPort: 8080",
			},
		},
		{
			name:       "chart with values mapping",
			packageDir: inspectTestDataPath("chart-with-helm-values"),
			opts: InspectDefinitionResourcesOptions{
				Values: value.Values{
					"customField": "fromValues",
					"image": map[string]any{
						"pullPolicy": "IfNotPresent",
					},
					"port": 8080,
				},
			},
			expectedResources: 2, // 1 chart + 1 values file
			expectedContent: []string{
				"containerPort: 8080",
				"customField: fromValues",
				"pullPolicy: IfNotPresent",
				"port: 8080",
			},
		},
		{
			name:       "chart with values and variables",
			packageDir: inspectTestDataPath("chart-with-helm-values"),
			opts: InspectDefinitionResourcesOptions{
				DeploySetVariables: map[string]string{
					"REPLICAS": "5",
				},
				Values: value.Values{
					"customField": "testValue",
					"image": map[string]any{
						"pullPolicy": "Never",
					},
					"port": 9090,
				},
			},
			expectedResources: 2, // 1 chart + 1 values file
			expectedContent: []string{
				"replicas: 5",
				"containerPort: 9090",
				"customField: testValue",
				"pullPolicy: Never",
				"port: 9090",
				"replicaCount: \"5\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)

			absDir, err := filepath.Abs(tt.packageDir)
			require.NoError(t, err)

			resources, err := InspectDefinitionResources(ctx, absDir, tt.opts)
			require.NoError(t, err)
			require.Len(t, resources, tt.expectedResources)

			assertContainsAll(t, resources, tt.expectedContent)
		})
	}
}

func TestInspectDefinitionResources_Errors(t *testing.T) {
	t.Parallel()
	setupInspectTests(t)

	tests := []struct {
		name       string
		packageDir string
		opts       InspectDefinitionResourcesOptions
	}{
		{
			name:       "nonexistent package directory",
			packageDir: "/nonexistent/path/to/package",
			opts:       InspectDefinitionResourcesOptions{},
		},
		{
			name:       "invalid zarf.yaml path",
			packageDir: testDataRoot,
			opts:       InspectDefinitionResourcesOptions{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)

			resources, err := InspectDefinitionResources(ctx, tt.packageDir, tt.opts)
			require.Error(t, err)
			require.Nil(t, resources)
		})
	}
}
