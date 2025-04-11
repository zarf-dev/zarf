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
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestDevInspectManifests(t *testing.T) {
	t.Parallel()
	lint.ZarfSchema = testutil.LoadSchema(t, "../../zarf.schema.json")

	tests := []struct {
		name               string
		definitionDir      string
		expectedOutput     string
		packageName        string
		deploySetVariables map[string]string
		createSetVariables map[string]string
		kubeVersion        string
		flavor             string
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
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Inspect manifests
			buf := new(bytes.Buffer)
			opts := devInspectManifestsOptions{
				outputWriter:       buf,
				kubeVersion:        tc.kubeVersion,
				deploySetVariables: tc.deploySetVariables,
				createSetVariables: tc.createSetVariables,
				flavor:             tc.flavor,
			}
			err := opts.run(context.Background(), []string{tc.definitionDir})
			require.NoError(t, err)

			// validate
			expected, err := os.ReadFile(tc.expectedOutput)
			require.NoError(t, err)
			require.YAMLEq(t, string(expected), buf.String())
		})
	}
}
