// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager/images"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPackageList(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		outputFormat outputFormat
		file         string
	}{
		{
			name:         "json package list",
			outputFormat: outputJSON,
			file:         "expected.json",
		},
		{
			name:         "yaml package list",
			outputFormat: outputYAML,
			file:         "expected.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			c := &cluster.Cluster{
				Clientset: fake.NewClientset(),
			}

			packages := []state.DeployedPackage{
				{
					Name: "package1",
					Data: v1alpha1.ZarfPackage{
						Metadata: v1alpha1.ZarfMetadata{
							Version: "0.42.0",
						},
					},
					DeployedComponents: []state.DeployedComponent{
						{
							Name: "component1",
						},
						{
							Name: "component2",
						},
					},
				},
				{
					Name:              "package2",
					NamespaceOverride: "test2",
					Data: v1alpha1.ZarfPackage{
						Metadata: v1alpha1.ZarfMetadata{
							Version: "1.0.0",
						},
					},
					DeployedComponents: []state.DeployedComponent{
						{
							Name: "component3",
						},
						{
							Name: "component4",
						},
					},
				},
			}

			for _, p := range packages {
				b, err := json.Marshal(p)
				require.NoError(t, err)
				secret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      strings.Join([]string{config.ZarfPackagePrefix, p.Name}, ""),
						Namespace: "zarf",
						Labels: map[string]string{
							state.ZarfPackageInfoLabel: p.Name,
						},
					},
					Data: map[string][]byte{
						"data": b,
					},
				}
				_, err = c.Clientset.CoreV1().Secrets("zarf").Create(ctx, &secret, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			buf := new(bytes.Buffer)
			listOpts := packageListOptions{
				outputFormat: tt.outputFormat,
				outputWriter: buf,
				cluster:      c,
			}
			err := listOpts.run(ctx)
			require.NoError(t, err)
			b, err := os.ReadFile(filepath.Join("testdata", "package-list", tt.file))
			require.NoError(t, err)
			if tt.outputFormat == outputJSON {
				require.JSONEq(t, string(b), buf.String())
			}
			if tt.outputFormat == outputYAML {
				require.YAMLEq(t, string(b), buf.String())
			}
		})
	}
}

func TestPackageInspectManifests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		definitionDir  string
		components     string
		expectedOutput string
		packageName    string
		setVariables   map[string]string
		kubeVersion    string
		expectedErr    string
	}{
		{
			name:           "manifest inspect",
			packageName:    "manifests",
			definitionDir:  filepath.Join("testdata", "inspect-manifests", "manifest"),
			expectedOutput: filepath.Join("testdata", "inspect-manifests", "manifest", "expected.yaml"),
			setVariables: map[string]string{
				"REPLICAS": "2",
			},
		},
		{
			name:           "manifest inspect, select component",
			components:     "httpd-local",
			packageName:    "manifests",
			definitionDir:  filepath.Join("testdata", "inspect-manifests", "manifest"),
			expectedOutput: filepath.Join("testdata", "inspect-manifests", "manifest", "expected-httpd-component.yaml"),
			setVariables: map[string]string{
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
			setVariables: map[string]string{
				"REPLICAS": "2",
				"PORT":     "8080",
				"LABEL":    "httpd",
			},
		},
		{
			name:          "empty inspect",
			packageName:   "empty",
			definitionDir: filepath.Join("testdata", "inspect-manifests", "empty"),
			expectedErr:   "0 manifests found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpdir := t.TempDir()

			// Create package
			createOpts := packageCreateOptions{
				confirm: true,
				output:  tmpdir,
			}
			err := createOpts.run(context.Background(), []string{tc.definitionDir})
			require.NoError(t, err)

			// Inspect manifests
			buf := new(bytes.Buffer)
			opts := packageInspectManifestsOptions{
				outputWriter: buf,
				kubeVersion:  tc.kubeVersion,
				setVariables: tc.setVariables,
				components:   tc.components,
			}
			packagePath := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-%s-%s.tar.zst", tc.packageName, config.GetArch()))
			err = opts.run(context.Background(), []string{packagePath})
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
func newYAMLFileServer(t *testing.T, path string) *httptest.Server {
	t.Helper()
	abs, err := filepath.Abs(path)
	require.NoError(t, err)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		http.ServeFile(w, r, abs)
	}))
}

type ValuesFilesTestData struct {
	name           string
	components     string
	definitionDir  string
	expectedOutput string
	packageName    string
	setVariables   map[string]string
	kubeVersion    string
	expectedErr    string
}

func TestPackageInspectValuesFiles(t *testing.T) {
	t.Parallel()

	tests := []ValuesFilesTestData{
		{
			name:           "chart inspect",
			packageName:    "chart",
			definitionDir:  filepath.Join("testdata", "inspect-values-files", "chart"),
			expectedOutput: filepath.Join("testdata", "inspect-values-files", "chart", "expected.yaml"),
			kubeVersion:    "1.25",
			setVariables: map[string]string{
				"REPLICAS":    "2",
				"DESCRIPTION": ".chart.variables takes priority",
				"PORT":        "8080",
			},
		},
		{
			name:           "chart inspect with one component",
			components:     "httpd-local",
			packageName:    "chart",
			definitionDir:  filepath.Join("testdata", "inspect-values-files", "chart"),
			expectedOutput: filepath.Join("testdata", "inspect-values-files", "chart", "expected-httpd-component.yaml"),
			kubeVersion:    "1.25",
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			checkPackageValuesInspectFiles(t, tc)
		})
	}
}

func TestPackageInspectRemoteValuesFiles(t *testing.T) {
	// set up a test http server that serves test values file:
	remoteValuesFile := filepath.Join("testdata", "inspect-values-files", "chart-remote", "remote-values", "values.yaml")
	fileServer := newYAMLFileServer(t, remoteValuesFile)
	url := fileServer.URL + "/values.yaml"
	defer fileServer.Close()

	// Prepare zarf.yaml in-place in chart-remote by templating zarf-template.yaml
	srcDir := filepath.Join("testdata", "inspect-values-files", "chart-remote")
	tmplPath := filepath.Join(srcDir, "zarf-template.yaml")
	b, err := os.ReadFile(tmplPath)
	require.NoError(t, err)
	zarfContent := strings.ReplaceAll(string(b), "VALUES_YAML_URL", url)
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "zarf.yaml"), []byte(zarfContent), 0o644))
	test := ValuesFilesTestData{
		name:           "chart inspect with remote values URL",
		packageName:    "chart",
		definitionDir:  srcDir,
		expectedOutput: filepath.Join("testdata", "inspect-values-files", "chart-remote", "expected.yaml"),
		kubeVersion:    "1.25",
		setVariables:   map[string]string{},
	}

	checkPackageValuesInspectFiles(t, test)
}

func checkPackageValuesInspectFiles(t *testing.T, tc ValuesFilesTestData) {
	tmpdir := t.TempDir()
	// Create package
	createOpts := packageCreateOptions{
		confirm: true,
		output:  tmpdir,
	}
	err := createOpts.run(context.Background(), []string{tc.definitionDir})
	require.NoError(t, err)

	// Inspect values files
	buf := new(bytes.Buffer)
	opts := packageInspectValuesFilesOptions{
		outputWriter: buf,
		kubeVersion:  tc.kubeVersion,
		setVariables: tc.setVariables,
		components:   tc.components,
	}
	packagePath := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-%s-%s.tar.zst", tc.packageName, config.GetArch()))
	err = opts.run(context.Background(), []string{packagePath})
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
}

// TestParseRegistryOverrides ensures that ordering is maintained for registry overrides.
func TestParseRegistryOverrides(t *testing.T) {
	t.Parallel()
	const intranetRegistry = "docker.example.com/repo"
	tests := []struct {
		name     string
		provided []string
		expected []images.RegistryOverride
	}{
		{
			name:     "single override",
			provided: []string{"docker.io=" + intranetRegistry},
			expected: []images.RegistryOverride{
				{Source: "docker.io", Override: intranetRegistry},
			},
		},
		{
			name: "multiple override",
			provided: []string{
				"docker.io=" + intranetRegistry,
				"registry1.dso.mil=" + intranetRegistry,
				"ghcr.io=" + intranetRegistry,
				"quay.io=" + intranetRegistry,
			},
			expected: []images.RegistryOverride{
				{Source: "registry1.dso.mil", Override: intranetRegistry},
				{Source: "quay.io", Override: intranetRegistry},
				{Source: "ghcr.io", Override: intranetRegistry},
				{Source: "docker.io", Override: intranetRegistry},
			},
		},
		{
			name: "prefix override",
			provided: []string{
				"docker.io/library=" + intranetRegistry,
				"docker.io=" + intranetRegistry,
			},
			expected: []images.RegistryOverride{
				{Source: "docker.io/library", Override: intranetRegistry},
				{Source: "docker.io", Override: intranetRegistry},
			},
		},
		{
			name: "multiple prefix override",
			provided: []string{
				"docker.io/library=" + intranetRegistry,
				"docker.io=" + intranetRegistry,
				"registry1.dso.mil/libary=" + intranetRegistry,
				"registry1.dso.mil=" + intranetRegistry,
			},
			expected: []images.RegistryOverride{
				{Source: "registry1.dso.mil/libary", Override: intranetRegistry},
				{Source: "registry1.dso.mil", Override: intranetRegistry},
				{Source: "docker.io/library", Override: intranetRegistry},
				{Source: "docker.io", Override: intranetRegistry},
			},
		},
		{
			name: "prefix override with multiple standard",
			provided: []string{
				"docker.io/library=" + intranetRegistry,
				"docker.io=" + intranetRegistry,
				"registry1.dso.mil=" + intranetRegistry,
				"ghcr.io=" + intranetRegistry,
				"quay.io=" + intranetRegistry,
			},
			expected: []images.RegistryOverride{
				{Source: "registry1.dso.mil", Override: intranetRegistry},
				{Source: "quay.io", Override: intranetRegistry},
				{Source: "ghcr.io", Override: intranetRegistry},
				{Source: "docker.io/library", Override: intranetRegistry},
				{Source: "docker.io", Override: intranetRegistry},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := parseRegistryOverrides(tc.provided)
			require.NoError(t, err)
			for index, element := range result {
				require.Equal(t, tc.expected[index], element, "Element at index failed to match: %d", index)
			}
		})
	}

	errorTests := []struct {
		name          string
		provided      []string
		errorContents string
	}{
		{
			name:          "error: invalid mapping",
			provided:      []string{"docker.io:" + intranetRegistry},
			errorContents: "registry override missing '='",
		},
		{
			name:          "error: invalid source",
			provided:      []string{"=" + intranetRegistry},
			errorContents: "registry override missing source",
		},
		{
			name:          "error: invalid override",
			provided:      []string{"docker.io="},
			errorContents: "registry override missing value",
		},
		{
			name:          "error: duplicate source",
			provided:      []string{"docker.io=" + intranetRegistry, "docker.io=" + intranetRegistry},
			errorContents: "registry override has duplicate source: existing index 0, new index 1, source docker.io",
		},
	}

	for _, tc := range errorTests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseRegistryOverrides(tc.provided)
			require.ErrorContains(t, err, tc.errorContents)
		})
	}
}
