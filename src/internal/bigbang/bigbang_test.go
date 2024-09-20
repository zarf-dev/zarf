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

	"github.com/defenseunicorns/pkg/helpers/v2"
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
					typeMeta: metav1.TypeMeta{
						Kind:       "HelmRelease",
						APIVersion: "helm.toolkit.fluxcd.io/v2beta1",
					},
					metadata: metav1.ObjectMeta{
						Name:      "my-helm-release",
						Namespace: "default",
					},
					namespacedDependencies: []string{"istio.another-helm-release"},
					namespacedSource:       "default.my-git-repo",
					valuesFrom: []fluxv2.ValuesReference{
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
			name:        "Airgap false",
			airgap:      false,
			valuesFiles: []string{},
			version:     "2.35.0",
			repo:        "https://repo1.dso.mil/big-bang/bigbang",
			expectedFiles: []string{
				filepath.Join("testdata", "addBBManifests", "airgap-false", "bb-gitrepository.yaml"),
				filepath.Join("testdata", "addBBManifests", "airgap-false", "bb-helmrelease.yaml"),
			},
		},
		{
			name:   "Airgap true with values files and v2beta1 version",
			airgap: true,
			valuesFiles: []string{
				filepath.Join("testdata", "addBBManifests", "airgap-true", "neuvector.yaml"),
			},
			version: "2.0.0",
			repo:    "https://repo1.dso.mil/big-bang/bigbang",
			expectedFiles: []string{
				filepath.Join("testdata", "addBBManifests", "airgap-true", "bb-gitrepository.yaml"),
				filepath.Join("testdata", "addBBManifests", "airgap-true", "bb-helmrelease.yaml"),
				filepath.Join("testdata", "addBBManifests", "airgap-true", "bb-zarf-credentials.yaml"),
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
	bbHealthChecks := []v1alpha1.NamespacedObjectKindReference{
		{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Namespace: "bigbang", Name: "grafana"},
		{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Namespace: "bigbang", Name: "istio"},
		{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Namespace: "bigbang", Name: "istio-operator"},
		{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Namespace: "bigbang", Name: "kiali"},
		{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Namespace: "bigbang", Name: "kyverno"},
		{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Namespace: "bigbang", Name: "kyverno-policies"},
		{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Namespace: "bigbang", Name: "kyverno-reporter"},
		{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Namespace: "bigbang", Name: "loki"},
		{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Namespace: "bigbang", Name: "monitoring"},
		{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Namespace: "bigbang", Name: "neuvector"},
		{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Namespace: "bigbang", Name: "promtail"},
		{APIVersion: "helm.toolkit.fluxcd.io/v2", Kind: "HelmRelease", Namespace: "bigbang", Name: "tempo"},
	}
	bbImages := []string{
		"registry1.dso.mil/ironbank/big-bang/grafana/grafana-plugins:11.1.4",
		"registry1.dso.mil/ironbank/kiwigrid/k8s-sidecar:1.27.5",
		"registry1.dso.mil/ironbank/big-bang/base:2.1.0",
		"registry1.dso.mil/ironbank/opensource/istio/pilot:1.22.4",
		"registry1.dso.mil/ironbank/opensource/istio/proxyv2:1.22.4",
		"registry1.dso.mil/ironbank/opensource/istio/operator:1.22.4",
		"registry1.dso.mil/ironbank/opensource/kiali/kiali:v1.89.0",
		"registry1.dso.mil/ironbank/opensource/kiali/kiali-operator:v1.89.1",
		"registry1.dso.mil/ironbank/opensource/kyverno:v1.12.5",
		"registry1.dso.mil/ironbank/opensource/kyverno/kyvernopre:v1.12.5",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.29.7",
		"registry1.dso.mil/ironbank/redhat/ubi/ubi9-minimal:9.4",
		"registry1.dso.mil/ironbank/opensource/kyverno/kyverno/reports-controller:v1.12.5",
		"registry1.dso.mil/ironbank/opensource/kyverno/kyverno/background-controller:v1.12.5",
		"registry1.dso.mil/ironbank/opensource/kyverno/kyverno/cleanup-controller:v1.12.5",
		"registry1.dso.mil/ironbank/opensource/kyverno/kyvernocli:v1.12.5",
		"registry1.dso.mil/ironbank/opensource/kyverno/policy-reporter:2.20.1",
		"registry1.dso.mil/ironbank/opensource/grafana/loki:3.1.1",
		"registry1.dso.mil/ironbank/opensource/kubernetes-sigs/metrics-server:v0.7.1",
		"registry1.dso.mil/ironbank/opensource/prometheus/alertmanager:v0.27.0",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.29.6",
		"registry1.dso.mil/ironbank/opensource/kubernetes/kube-state-metrics:v2.12.0",
		"registry1.dso.mil/ironbank/opensource/ingress-nginx/kube-webhook-certgen:v1.3.0",
		"registry1.dso.mil/ironbank/opensource/prometheus/prometheus:v2.53.0",
		"registry1.dso.mil/ironbank/opensource/prometheus-operator/prometheus-config-reloader:v0.75.0",
		"registry1.dso.mil/ironbank/opensource/prometheus-operator/prometheus-operator:v0.75.0",
		"registry1.dso.mil/ironbank/opensource/prometheus/node-exporter:v1.8.1",
		"registry1.dso.mil/ironbank/opensource/thanos/thanos:v0.35.1",
		"registry1.dso.mil/ironbank/neuvector/neuvector/controller:5.3.4",
		"registry1.dso.mil/ironbank/neuvector/neuvector/enforcer:5.3.4",
		"registry1.dso.mil/ironbank/neuvector/neuvector/manager:5.3.4",
		"registry1.dso.mil/ironbank/neuvector/neuvector/scanner:5",
		"registry1.dso.mil/ironbank/neuvector/neuvector/prometheus-exporter:5.3.2",
		"registry1.dso.mil/ironbank/opensource/grafana/promtail:v3.0.0",
		"registry1.dso.mil/ironbank/opensource/grafana/tempo:2.5.0",
		"registry1.dso.mil/ironbank/opensource/grafana/tempo-query:2.5.0",
	}
	bbRepos := []string{
		"https://repo1.dso.mil/big-bang/bigbang@2.35.0",
		"https://repo1.dso.mil/big-bang/product/packages/grafana.git@8.4.6-bb.1",
		"https://repo1.dso.mil/big-bang/product/packages/istio-controlplane.git@1.22.4-bb.1",
		"https://repo1.dso.mil/big-bang/product/packages/istio-operator.git@1.22.4-bb.0",
		"https://repo1.dso.mil/big-bang/product/packages/kiali.git@1.89.0-bb.0",
		"https://repo1.dso.mil/big-bang/product/packages/kyverno-policies.git@3.2.5-bb.3",
		"https://repo1.dso.mil/big-bang/product/packages/kyverno-reporter.git@2.24.1-bb.0",
		"https://repo1.dso.mil/big-bang/product/packages/kyverno.git@3.2.6-bb.0",
		"https://repo1.dso.mil/big-bang/product/packages/loki.git@6.10.0-bb.0",
		"https://repo1.dso.mil/big-bang/product/packages/metrics-server.git@3.12.1-bb.4",
		"https://repo1.dso.mil/big-bang/product/packages/monitoring.git@62.1.0-bb.0",
		"https://repo1.dso.mil/big-bang/product/packages/neuvector.git@2.7.8-bb.1",
		"https://repo1.dso.mil/big-bang/product/packages/promtail.git@6.16.2-bb.3",
		"https://repo1.dso.mil/big-bang/product/packages/tempo.git@1.10.3-bb.0",
	}
	tests := []struct {
		name     string
		airgap   bool
		version  string
		repo     string
		skipFlux bool
		pkg      v1alpha1.ZarfPackage
	}{
		{
			name:     "default BB install",
			airgap:   true,
			version:  "2.35.0",
			repo:     "https://repo1.dso.mil/big-bang/bigbang",
			skipFlux: false,
			pkg: v1alpha1.ZarfPackage{
				APIVersion: "zarf.dev/v1alpha1",
				Kind:       v1alpha1.ZarfPackageConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Name: "bigbang",
				},
				Components: []v1alpha1.ZarfComponent{
					{
						Name:     "flux",
						Required: helpers.BoolPtr(true),
						Manifests: []v1alpha1.ZarfManifest{
							{
								Name:      "flux-system",
								Namespace: "flux-system",
								Files:     []string{"flux/bb-flux.yaml"},
							},
						},
						Images: []string{
							"registry1.dso.mil/ironbank/fluxcd/source-controller:v1.3.0",
							"registry1.dso.mil/ironbank/fluxcd/kustomize-controller:v1.3.0",
							"registry1.dso.mil/ironbank/fluxcd/helm-controller:v1.0.1",
							"registry1.dso.mil/ironbank/fluxcd/notification-controller:v1.3.0",
						},
					},
					{
						Name:     "bigbang",
						Required: helpers.BoolPtr(true),
						Manifests: []v1alpha1.ZarfManifest{
							{
								Name:      "bigbang",
								Namespace: "bigbang",
								Files: []string{
									"manifests/bb-gitrepository.yaml",
									"manifests/bb-zarf-credentials.yaml",
									"manifests/bb-helmrelease.yaml",
								},
							},
						},
						Images: bbImages,
						Repos:  bbRepos,
						Actions: v1alpha1.ZarfComponentActions{
							OnRemove: v1alpha1.ZarfComponentActionSet{
								Before: []v1alpha1.ZarfComponentAction{
									{
										Cmd:         "./zarf tools kubectl patch helmrelease -n bigbang bigbang --type=merge -p '{\"spec\":{\"suspend\":true}}'",
										Description: "Suspend Big Bang HelmReleases to prevent reconciliation during removal.",
									},
								},
							},
						},
						HealthChecks: bbHealthChecks,
					},
				},
			},
		},
		{
			name:     "skip flux",
			airgap:   true,
			version:  "2.35.0",
			repo:     "https://repo1.dso.mil/big-bang/bigbang",
			skipFlux: true,
			pkg: v1alpha1.ZarfPackage{
				APIVersion: "zarf.dev/v1alpha1",
				Kind:       v1alpha1.ZarfPackageConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Name: "bigbang",
				},
				Components: []v1alpha1.ZarfComponent{
					{
						Name:     "bigbang",
						Required: helpers.BoolPtr(true),
						Manifests: []v1alpha1.ZarfManifest{
							{
								Name:      "bigbang",
								Namespace: "bigbang",
								Files: []string{
									"manifests/bb-gitrepository.yaml",
									"manifests/bb-zarf-credentials.yaml",
									"manifests/bb-helmrelease.yaml",
								},
							},
						},
						Images: bbImages,
						Repos:  bbRepos,
						Actions: v1alpha1.ZarfComponentActions{
							OnRemove: v1alpha1.ZarfComponentActionSet{
								Before: []v1alpha1.ZarfComponentAction{
									{
										Cmd:         "./zarf tools kubectl patch helmrelease -n bigbang bigbang --type=merge -p '{\"spec\":{\"suspend\":true}}'",
										Description: "Suspend Big Bang HelmReleases to prevent reconciliation during removal.",
									},
								},
							},
						},
						HealthChecks: bbHealthChecks,
					},
				},
			},
		},
		{
			name:     "Not air gapped",
			airgap:   false,
			version:  "2.35.0",
			repo:     "https://repo1.dso.mil/big-bang/bigbang",
			skipFlux: false,
			pkg: v1alpha1.ZarfPackage{
				APIVersion: "zarf.dev/v1alpha1",
				Kind:       v1alpha1.ZarfPackageConfig,
				Metadata: v1alpha1.ZarfMetadata{
					YOLO: true,
					Name: "bigbang",
				},
				Components: []v1alpha1.ZarfComponent{
					{
						Name:     "flux",
						Required: helpers.BoolPtr(true),
						Manifests: []v1alpha1.ZarfManifest{
							{
								Name:      "flux-system",
								Namespace: "flux-system",
								Files:     []string{"flux/bb-flux.yaml"},
							},
						},
					},
					{
						Name:     "bigbang",
						Required: helpers.BoolPtr(true),
						Manifests: []v1alpha1.ZarfManifest{
							{
								Name:      "bigbang",
								Namespace: "bigbang",
								Files: []string{
									"manifests/bb-gitrepository.yaml",
									"manifests/bb-helmrelease.yaml",
								},
							},
						},
						Actions: v1alpha1.ZarfComponentActions{
							OnRemove: v1alpha1.ZarfComponentActionSet{
								Before: []v1alpha1.ZarfComponentAction{
									{
										Cmd:         "./zarf tools kubectl patch helmrelease -n bigbang bigbang --type=merge -p '{\"spec\":{\"suspend\":true}}'",
										Description: "Suspend Big Bang HelmReleases to prevent reconciliation during removal.",
									},
								},
							},
						},
						HealthChecks: bbHealthChecks,
					},
				},
			},
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

			actualContent, err := os.ReadFile(filepath.Join(tempDir, "zarf.yaml"))
			require.NoError(t, err)
			var actualPkg v1alpha1.ZarfPackage
			err = yaml.Unmarshal(actualContent, &actualPkg)
			require.NoError(t, err)
			for i, c := range tt.pkg.Components {
				for j, m := range c.Manifests {
					for k, f := range m.Files {
						tt.pkg.Components[i].Manifests[j].Files[k] = fmt.Sprintf("%s/%s", tempDir, f)
					}
				}
			}
			require.Equal(t, tt.pkg, actualPkg)
		})
	}
}
