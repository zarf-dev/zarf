// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package bigbang

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	fluxv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRequiredBigBangVersions(t *testing.T) {
	// Support 1.54.0 and beyond
	vv, err := isValidVersion("1.54.0")
	require.NoError(t, err)
	require.True(t, vv)

	// Do not support earlier than 1.54.0
	vv, err = isValidVersion("1.53.0")
	require.NoError(t, err)
	require.False(t, vv)

	// Support for Big Bang release candidates
	vv, err = isValidVersion("1.57.0-rc.0")
	require.NoError(t, err)
	require.True(t, vv)

	// Support for Big Bang 2.0.0
	vv, err = isValidVersion("2.0.0")
	require.NoError(t, err)
	require.True(t, vv)

	// Fail on non-semantic versions
	vv, err = isValidVersion("1.57b")
	require.EqualError(t, err, "Invalid Semantic Version")
	require.False(t, vv)
}

func TestFindBBResources(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "find-bb-template-resources.yaml"))
	require.NoError(t, err)
	template := string(b)
	tests := []struct {
		name                      string
		input                     string
		expectedGitRepos          map[string]string
		expectedHelmReleaseDeps   map[string]HelmReleaseDependency
		expectedHelmReleaseValues map[string]map[string]interface{}
		expectedErr               bool
	}{
		{
			name:  "Valid input with HelmRelease, GitRepository, Secret, and ConfigMap",
			input: template,
			expectedGitRepos: map[string]string{
				"default.my-git-repo": "https://github.com/example/repo.git@main",
			},
			expectedHelmReleaseDeps: map[string]HelmReleaseDependency{
				"default.my-helm-release": {
					Metadata: metav1.ObjectMeta{
						Name:      "my-helm-release",
						Namespace: "default",
					},
					NamespacedDependencies: []string{"istio.another-helm-release"},
					NamespacedSource:       "default.my-git-repo",
					ValuesFrom: []fluxv2.ValuesReference{
						{
							Kind: "ConfigMap",
							Name: "my-configmap",
						},
						{
							Kind: "Secret",
							Name: "my-secret",
						},
					},
				},
			},
			expectedHelmReleaseValues: map[string]map[string]interface{}{
				"default.my-helm-release": {
					"key1": "value1",
					"key2": "value2",
				},
			},
			expectedErr: false,
		},
		{
			name: "Invalid YAML input",
			input: `
		invalid-yaml
		`,
			expectedGitRepos:          nil,
			expectedHelmReleaseDeps:   nil,
			expectedHelmReleaseValues: nil,
			expectedErr:               true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitRepos, helmReleaseDeps, helmReleaseValues, err := findBBResources(tt.input)
			if tt.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedGitRepos, gitRepos)
				require.Equal(t, tt.expectedHelmReleaseDeps, helmReleaseDeps)
				require.Equal(t, tt.expectedHelmReleaseValues, helmReleaseValues)
			}
		})
	}
}

func TestGetValuesFromManifest(t *testing.T) {
	tests := []struct {
		name           string
		fileName       string
		expectedOutput string
		expectError    bool
	}{
		{
			name:           "Valid Secret",
			fileName:       "valid_secret.yaml",
			expectedOutput: "key: value\n",
			expectError:    false,
		},
		{
			name:           "Valid ConfigMap",
			fileName:       "valid_configmap.yaml",
			expectedOutput: "key: value\n",
			expectError:    false,
		},
		{
			name:           "Invalid Kind",
			fileName:       "invalid_kind.yaml",
			expectedOutput: "",
			expectError:    true,
		},
		{
			name:           "Missing values.yaml",
			fileName:       "missing_values.yaml",
			expectedOutput: "",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join("testdata", "getValuesFromManifest", tt.fileName)
			output, err := getValuesFromManifest(filePath)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedOutput, output)
			}
		})
	}
}

func TestAddBigBangManifests(t *testing.T) {
	// Set up a temporary directory for the manifests

	tempDir := t.TempDir()

	// ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 	// Extract the filename from the URL
	// 	urlPath := r.URL.Path
	// 	filename := path.Base(urlPath)
	// 	filepath := path.Join("testdata", "downloaded", filename)
	// 	data, err := os.ReadFile(filepath)
	// 	require.NoError(t, err)
	// 	w.Write(data)
	// }))
	// defer ts.Close()

	// Define test cases
	tests := []struct {
		name          string
		airgap        bool
		valuesFiles   []string
		version       string
		repo          string
		expectedFiles []string
		expectError   bool
	}{
		{
			name:        "Airgap true",
			airgap:      true,
			valuesFiles: []string{},
			version:     "2.35.0",
			repo:        "https://repo1.dso.mil/big-bang/bigbang",
			expectedFiles: []string{
				filepath.Join("testdata", "addBBManifests", "airgap-true", "gitrepository.yaml"),
				filepath.Join("testdata", "addBBManifests", "airgap-true", "bb-zarf-credentials.yaml"),
				filepath.Join("testdata", "addBBManifests", "airgap-true", "helmrelease.yaml"),
			},
			expectError: false,
		},
		{
			name:   "Airgap false with values files and v2beta1 version",
			airgap: false,
			valuesFiles: []string{
				filepath.Join("testdata", "addBBManifests", "airgap-false", "neuvector.yaml"),
			},
			version: "2.0.0",
			repo:    "https://repo1.dso.mil/big-bang/bigbang",
			expectedFiles: []string{
				filepath.Join("testdata", "addBBManifests", "airgap-false", "gitrepository.yaml"),
				filepath.Join("testdata", "addBBManifests", "airgap-false", "helmrelease.yaml"),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var expectedManifests []string
			for _, f := range tt.expectedFiles {
				expectedManifests = append(expectedManifests, filepath.Join(tempDir, filepath.Base(f)))
			}
			expectedManifests = append(expectedManifests, tt.valuesFiles...)
			manifest, err := addBigBangManifests(context.Background(), tt.airgap, tempDir, tt.valuesFiles, tt.version, tt.repo)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.ElementsMatch(t, expectedManifests, manifest.Files)

			for _, expectedFile := range tt.expectedFiles {
				_, filename := filepath.Split(expectedFile)
				generatedFile := filepath.Join(tempDir, filename)
				expectedContent, err := os.ReadFile(expectedFile)
				require.NoError(t, err)
				generatedContent, err := os.ReadFile(generatedFile)
				require.NoError(t, err)
				require.Equal(t, string(expectedContent), string(generatedContent))
			}
		})
	}
}
