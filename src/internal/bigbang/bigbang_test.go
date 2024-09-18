// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package bigbang

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fluxv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
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
	b, err := os.ReadFile(filepath.Join("testdata", "findBBResources", "resources.yaml"))
	require.NoError(t, err)
	template := string(b)
	tests := []struct {
		name                      string
		input                     string
		expectedGitRepos          map[string]string
		expectedHelmReleaseDeps   []HelmReleaseDependency
		expectedHelmReleaseValues map[string]map[string]interface{}
	}{
		{
			name:  "Valid input with HelmRelease, GitRepository, Secret, and ConfigMap",
			input: template,
			expectedGitRepos: map[string]string{
				"default.my-git-repo": "https://github.com/example/repo.git@main",
			},
			expectedHelmReleaseDeps: []HelmReleaseDependency{
				{
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitRepos, helmReleaseDeps, helmReleaseValues, err := findBBResources(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expectedGitRepos, gitRepos)
			require.Equal(t, tt.expectedHelmReleaseDeps, helmReleaseDeps)
			require.Equal(t, tt.expectedHelmReleaseValues, helmReleaseValues)
		})
	}
}

func TestGetValuesFromManifest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		fileName       string
		expectedOutput string
		expectedErr    error
	}{
		{
			name:           "Valid Secret string data",
			fileName:       "valid_secret_string_data.yaml",
			expectedOutput: "key: value\n",
			expectedErr:    nil,
		},
		{
			name:           "Valid Secret regular data",
			fileName:       "valid_secret_data.yaml",
			expectedOutput: "key: value",
			expectedErr:    nil,
		},
		{
			name:           "Valid ConfigMap",
			fileName:       "valid_configmap.yaml",
			expectedOutput: "key: value\n",
			expectedErr:    nil,
		},
		{
			name:           "Invalid Kind",
			fileName:       "invalid_kind.yaml",
			expectedOutput: "",
			expectedErr:    errors.New("values manifests must be a Secret or ConfigMap"),
		},
		{
			name:           "Missing values.yaml",
			fileName:       "missing_values.yaml",
			expectedOutput: "",
			expectedErr:    errors.New("values.yaml key must exist in data"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			filePath := filepath.Join("testdata", "getValuesFromManifest", tt.fileName)
			output, err := getValuesFromManifest(filePath)
			if tt.expectedErr != nil {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedOutput, output)
		})
	}
}

func TestAddBigBangManifests(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		airgap        bool
		valuesFiles   []string
		version       string
		repo          string
		expectedFiles []string
	}{
		{
			name:        "Airgap true",
			airgap:      true,
			valuesFiles: []string{},
			version:     "2.35.0",
			repo:        "https://repo1.dso.mil/big-bang/bigbang",
			expectedFiles: []string{
				filepath.Join("testdata", "addBBManifests", "airgap-true", "bb-gitrepository.yaml"),
				filepath.Join("testdata", "addBBManifests", "airgap-true", "bb-zarf-credentials.yaml"),
				filepath.Join("testdata", "addBBManifests", "airgap-true", "bb-helmrelease.yaml"),
			},
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
				filepath.Join("testdata", "addBBManifests", "airgap-false", "bb-gitrepository.yaml"),
				filepath.Join("testdata", "addBBManifests", "airgap-false", "bb-helmrelease.yaml"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Expected URL format:
				// /-/raw/{version}/base/{file}?ref_type=tags
				// Example: /-/raw/2.35.0/base/gitrepository.yaml?ref_type=tags

				// Split the URL path to extract version and file name
				pathParts := strings.Split(r.URL.Path, "/")
				if len(pathParts) < 5 {
					http.Error(w, "Invalid URL path", http.StatusBadRequest)
					return
				}
				version := pathParts[5]
				gitPath := pathParts[7]
				localFilePath := filepath.Join("testdata", "addBBManifests", "mock-downloads", version, gitPath)
				data, err := os.ReadFile(localFilePath)
				if err != nil {
					http.Error(w, "File Not Found", http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusOK)
				//nolint: errcheck // ignore
				w.Write(data)
			}))
			defer testServer.Close()

			testRepoURL := testServer.URL + "/big-bang/bigbang"
			tt.repo = testRepoURL

			tempDir := t.TempDir()
			var expectedManifests []string
			for _, f := range tt.expectedFiles {
				expectedManifests = append(expectedManifests, filepath.Join(tempDir, filepath.Base(f)))
			}
			expectedManifests = append(expectedManifests, tt.valuesFiles...)
			manifest, err := createBBManifests(context.Background(), tt.airgap, tempDir, tt.valuesFiles, tt.version, tt.repo)
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

func TestCreate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		airgap          bool
		version         string
		repo            string
		skipFlux        bool
		expectedPackage string
	}{
		{
			name:            "default BB install",
			airgap:          true,
			version:         "2.35.0",
			repo:            "https://repo1.dso.mil/big-bang/bigbang",
			skipFlux:        false,
			expectedPackage: filepath.Join("testdata", "create", "default.yaml"),
		},
		{
			name:            "skip flux",
			airgap:          true,
			version:         "2.35.0",
			repo:            "https://repo1.dso.mil/big-bang/bigbang",
			skipFlux:        true,
			expectedPackage: filepath.Join("testdata", "create", "skip_flux.yaml"),
		},
		{
			name:            "Not air gapped",
			airgap:          false,
			version:         "2.35.0",
			repo:            "https://repo1.dso.mil/big-bang/bigbang",
			skipFlux:        false,
			expectedPackage: filepath.Join("testdata", "create", "not_airgap.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tempDir := t.TempDir()
			bbOpts := Opts{
				Airgap:              tt.airgap,
				ValuesFileManifests: nil,
				Version:             tt.version,
				Repo:                tt.repo,
				SkipFlux:            tt.skipFlux,
				BaseDir:             tempDir,
				KubeVersion:         "v1.30.0",
			}
			err := Create(context.Background(), bbOpts)
			require.NoError(t, err)

			expectedContent, err := os.ReadFile(tt.expectedPackage)
			require.NoError(t, err)
			var expectedPkg v1alpha1.ZarfPackage
			err = yaml.Unmarshal(expectedContent, &expectedPkg)
			require.NoError(t, err)
			actualContent, err := os.ReadFile(filepath.Join(tempDir, "zarf.yaml"))
			require.NoError(t, err)
			var actualPkg v1alpha1.ZarfPackage
			err = yaml.Unmarshal(actualContent, &actualPkg)
			require.NoError(t, err)
			for i, c := range actualPkg.Components {
				for j, m := range c.Manifests {
					for k, f := range m.Files {
						actualPkg.Components[i].Manifests[j].Files[k] = strings.TrimPrefix(f, fmt.Sprintf("%s/", tempDir))
					}
				}
			}
			require.Equal(t, expectedPkg, actualPkg)
		})
	}
}
