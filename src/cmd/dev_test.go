// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/feature"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

func TestDevInspectManifests(t *testing.T) {
	t.Parallel()

	// Enable values feature for tests
	err := feature.Set([]feature.Feature{{Name: feature.Values, Enabled: true}})
	require.NoError(t, err)

	tests := []struct {
		name               string
		definitionDir      string
		expectedOutput     string
		packageName        string
		deploySetVariables map[string]string
		createSetVariables map[string]string
		valuesFiles        []string
		setValues          map[string]string
		kubeVersion        string
		flavor             string
		expectedErr        string
	}{
		{
			name:           "manifest inspect",
			packageName:    "manifests",
			definitionDir:  filepath.Join("testdata", "inspect-manifests", "manifest"),
			expectedOutput: filepath.Join("testdata", "inspect-manifests", "manifest", "expected.yaml"),
			deploySetVariables: map[string]string{
				"REPLICAS": "2",
			},
		},
		{
			name:           "kustomize inspect",
			packageName:    "kustomize",
			definitionDir:  filepath.Join("testdata", "inspect-manifests", "kustomize"),
			expectedOutput: filepath.Join("testdata", "inspect-manifests", "kustomize", "expected.yaml"),
		},
		{
			name:           "chart inspect",
			packageName:    "chart",
			definitionDir:  filepath.Join("testdata", "inspect-manifests", "chart"),
			expectedOutput: filepath.Join("testdata", "inspect-manifests", "chart", "expected.yaml"),
			kubeVersion:    "1.25",
			deploySetVariables: map[string]string{
				"REPLICAS": "2",
				"PORT":     "8080",
				"LABEL":    "httpd",
			},
		},
		{
			name:           "variable templates inspect",
			packageName:    "variable-templates",
			definitionDir:  filepath.Join("testdata", "inspect-manifests", "variable-templates"),
			expectedOutput: filepath.Join("testdata", "inspect-manifests", "variable-templates", "expected.yaml"),
			createSetVariables: map[string]string{
				"HTTPD_VERSION": "1.0.0",
				"LABEL":         "httpd",
			},
			deploySetVariables: map[string]string{
				"REPLICAS": "2",
			},
			flavor: "cool",
		},
		{
			name:          "empty inspect",
			packageName:   "empty",
			definitionDir: filepath.Join("testdata", "inspect-manifests", "empty"),
			expectedErr:   "0 manifests found",
		},
		{
			name:           "manifest with CLI values only",
			packageName:    "manifest-with-values",
			definitionDir:  filepath.Join("testdata", "inspect-manifests", "manifest-with-values"),
			expectedOutput: filepath.Join("testdata", "inspect-manifests", "manifest-with-values", "expected.yaml"),
			valuesFiles: []string{
				filepath.Join("testdata", "inspect-manifests", "manifest-with-values", "user-values.yaml"),
			},
			setValues: map[string]string{
				"replicas": "5",
				"imageTag": "latest",
			},
		},
		{
			name:           "manifest with package default values",
			packageName:    "manifest-with-package-values",
			definitionDir:  filepath.Join("testdata", "inspect-manifests", "manifest-with-package-values"),
			expectedOutput: filepath.Join("testdata", "inspect-manifests", "manifest-with-package-values", "expected-default.yaml"),
		},
		{
			name:           "manifest with package values overridden by CLI",
			packageName:    "manifest-with-package-values",
			definitionDir:  filepath.Join("testdata", "inspect-manifests", "manifest-with-package-values"),
			expectedOutput: filepath.Join("testdata", "inspect-manifests", "manifest-with-package-values", "expected-override.yaml"),
			setValues: map[string]string{
				"app.name":             "overridden-app",
				"app.replicas":         "5",
				"app.image.repository": "nginx",
				"app.image.tag":        "latest",
				"app.port":             "8080",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Inspect manifests
			buf := new(bytes.Buffer)
			opts := devInspectManifestsOptions{
				outputWriter:       buf,
				kubeVersion:        tc.kubeVersion,
				deploySetVariables: tc.deploySetVariables,
				createSetVariables: tc.createSetVariables,
				valuesFiles:        tc.valuesFiles,
				setValues:          tc.setValues,
				flavor:             tc.flavor,
			}
			err := opts.run(context.Background(), []string{tc.definitionDir})
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)

			// validate
			expected, err := os.ReadFile(tc.expectedOutput)
			require.NoError(t, err)
			// Since we have multiple yamls split by the --- syntax we have to split them to accurately test
			expectedYAMLs, err := utils.SplitYAMLToString(expected)
			require.NoError(t, err)
			actualYAMLs, err := utils.SplitYAMLToString(buf.Bytes())
			require.NoError(t, err)
			require.Equal(t, expectedYAMLs, actualYAMLs)
		})
	}
}

func TestDevInspectValuesFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		definitionDir  string
		expectedOutput string
		packageName    string
		setVariables   map[string]string
		valuesFiles    []string
		setValues      map[string]string
		expectedErr    string
		components     string
	}{
		{
			name:           "chart inspect",
			packageName:    "chart",
			definitionDir:  filepath.Join("testdata", "inspect-values-files", "chart"),
			expectedOutput: filepath.Join("testdata", "inspect-values-files", "chart", "expected.yaml"),
			components:     "demo-helm-charts,different-values-set",
			setVariables: map[string]string{
				"REPLICAS":    "2",
				"DESCRIPTION": ".chart.variables takes priority",
				"PORT":        "8080",
			},
		},
		{
			name:          "manifest inspect -> fail with no values-files",
			packageName:   "manifests",
			definitionDir: filepath.Join("testdata", "inspect-manifests", "manifest"),
			expectedErr:   "0 values files found",
		},
		{
			name:           "chart with values from file and CLI",
			packageName:    "chart-with-values",
			definitionDir:  filepath.Join("testdata", "inspect-values-files", "chart-with-values"),
			expectedOutput: filepath.Join("testdata", "inspect-values-files", "chart-with-values", "expected.yaml"),
			setVariables: map[string]string{
				"REPLICAS": "3",
			},
			valuesFiles: []string{
				filepath.Join("testdata", "inspect-values-files", "chart-with-values", "user-values.yaml"),
			},
			setValues: map[string]string{
				"customField": "fromCLI",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Inspect manifests
			buf := new(bytes.Buffer)
			opts := devInspectValuesFilesOptions{
				outputWriter:       buf,
				deploySetVariables: tc.setVariables,
				valuesFiles:        tc.valuesFiles,
				setValues:          tc.setValues,
			}
			err := opts.run(context.Background(), []string{tc.definitionDir})
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)

			// validate
			expected, err := os.ReadFile(tc.expectedOutput)
			require.NoError(t, err)
			// Since we have multiple yamls split by the --- syntax we have to split them to accurately test
			expectedYAMLs, err := utils.SplitYAMLToString(expected)
			require.NoError(t, err)
			actualYAMLs, err := utils.SplitYAMLToString(buf.Bytes())
			require.NoError(t, err)
			require.Equal(t, expectedYAMLs, actualYAMLs)
		})
	}
}
