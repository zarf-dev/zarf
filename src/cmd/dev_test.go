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

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/feature"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

func TestParseValuesCoercesSetValues(t *testing.T) {
	t.Parallel()
	vals, err := parseValues(context.Background(), nil, map[string]string{
		"myBooleanVar":   "true",
		"app.replicas":   "3",
		"site.name":      "my-site",
		"app.version":    "1.0.0",
		"nested.enabled": "false",
	})
	require.NoError(t, err)
	app, ok := vals["app"].(map[string]any)
	require.True(t, ok)
	site, ok := vals["site"].(map[string]any)
	require.True(t, ok)
	nested, ok := vals["nested"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, vals["myBooleanVar"])
	require.Equal(t, int64(3), app["replicas"])
	require.Equal(t, "my-site", site["name"])
	require.Equal(t, "1.0.0", app["version"])
	require.Equal(t, false, nested["enabled"])
}

func TestParseValuesMergesConfigBaseWithCLI(t *testing.T) {
	t.Parallel()
	v := viper.New()
	v.Set(VPkgRemoveSetValues, map[string]string{"app.name": "fromConfig", "app.replicas": "1"})

	// Mirror the merge every set-values command performs: config set_values are the base,
	// CLI --set-values are applied after so they win per-key.
	cliSetValues := map[string]string{"app.replicas": "5", "app.tag": "latest"}
	setValues := mergeMap(v.GetStringMapString(VPkgRemoveSetValues), cliSetValues)

	vals, err := parseValues(context.Background(), nil, setValues)
	require.NoError(t, err)
	app, ok := vals["app"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "fromConfig", app["name"], "config-only key must survive when CLI values are provided")
	require.Equal(t, int64(5), app["replicas"], "CLI value must override config base per key")
	require.Equal(t, "latest", app["tag"], "CLI-only key must be applied")
}

func TestDevInspectManifests(t *testing.T) {
	t.Parallel()

	// Enable values feature for tests (ignore error if already set by a sibling test)
	_ = feature.Set([]feature.Feature{{Name: feature.Values, Enabled: true}}) //nolint:errcheck

	tests := []struct {
		name               string
		definitionDir      string
		expectedOutput     string
		packageName        string
		deploySetVariables map[string]string
		createSetPkgTmpl   map[string]string
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
			createSetPkgTmpl: map[string]string{
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
				createSetPkgTmpl:   tc.createSetPkgTmpl,
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

func TestDevSha256Sum(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		sourceURL   string
		extractPath string
		sha256sum   string
		expectedErr string
	}{
		{
			// this is the version before we vendored the go depends, and is a lot quicker to download
			name:      "dev sha tarball source v0.80.0",
			sourceURL: "https://github.com/zarf-dev/zarf/archive/refs/tags/v0.80.0.tar.gz",
			sha256sum: "67b5d5a2801db7dcce5f7d8ef46e36515a56a9b5f952f39cbe36166018167410",
		},
		{
			// this is the version before we vendored the go depends, and is a lot quicker to download
			name:        "dev sha tarball source v0.80.0 with extract path",
			sourceURL:   "https://github.com/zarf-dev/zarf/archive/refs/tags/v0.80.0.tar.gz",
			extractPath: "zarf-0.80.0/cosign.pub",
			sha256sum:   "8361bdbf3fb0c5d2980c5cb2192536b81ba439466d85b8328f7bf5de6fce58eb",
		},
		{
			name:      "dev sha tarball source v0.30.0",
			sourceURL: "https://github.com/zarf-dev/zarf/archive/refs/tags/v0.30.0.tar.gz",
			sha256sum: "a042c9ffec7907101b58dd5b1aee6e54d08d17d91e7e2572da22e4180e02decd",
		},
		{
			name:        "dev sha tarball source v0.30.0 with extract path",
			sourceURL:   "https://github.com/zarf-dev/zarf/archive/refs/tags/v0.30.0.tar.gz",
			extractPath: "zarf-0.30.0/cosign.pub",
			sha256sum:   "2aac6a3c85d8513545fa85c11ee0d58ea5125095052f77e8d33f22402dbe6e4e",
		},
		{
			name:        "dev sha no url",
			sourceURL:   "",
			expectedErr: "accepts 1 arg(s), received 0",
		},
		{
			name:      "dev sha url does not exist",
			sourceURL: "https://github.com/zarf-dev/zarf/archive/refs/tags/v99.99.99.tar.gz",
			expectedErr: `unable to compute the SHA256SUM hash
bad HTTP status: 404 Not Found`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := devSha256SumOptions{
				extractPath: tc.extractPath,
			}
			out, err := opts.compute(context.Background(), []string{tc.sourceURL})
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.sha256sum, out)
		})
	}
}
