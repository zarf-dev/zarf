// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

func TestGetDeployedPackageWarnsOnUnrecognizedAPIVersion(t *testing.T) {
	t.Parallel()

	const futureVersion = "zarf.dev/v9"
	knownDefinition, err := json.Marshal(v1alpha1.ZarfPackage{
		APIVersion: v1alpha1.APIVersion,
		Metadata:   v1alpha1.ZarfMetadata{Name: "future-package"},
	})
	require.NoError(t, err)
	deployed := state.DeployedPackage{
		Name: "future-package",
		PackageData: map[string]json.RawMessage{
			v1alpha1.APIVersion: knownDefinition,
			futureVersion:       json.RawMessage(`{"apiVersion":"zarf.dev/v9"}`),
		},
	}
	secretData, err := json.Marshal(deployed)
	require.NoError(t, err)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployed.GetSecretName(),
			Namespace: state.ZarfNamespaceName,
			Labels:    map[string]string{state.ZarfPackageInfoLabel: deployed.Name},
		},
		Data: map[string][]byte{"data": secretData},
	}
	c := &Cluster{Clientset: fake.NewClientset(secret)}
	var logs bytes.Buffer
	ctx := logger.WithContext(t.Context(), slog.New(slog.NewJSONHandler(&logs, nil)))
	known := deployed
	known.Name = "known-package"
	known.PackageData = map[string]json.RawMessage{v1alpha1.APIVersion: knownDefinition}
	knownData, err := json.Marshal(known)
	require.NoError(t, err)
	knownSecret := secret.DeepCopy()
	knownSecret.Name = known.GetSecretName()
	knownSecret.Data = map[string][]byte{"data": knownData}
	_, err = c.Clientset.CoreV1().Secrets(state.ZarfNamespaceName).Create(ctx, knownSecret, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = c.GetDeployedPackage(ctx, known.Name)
	require.NoError(t, err)
	require.Empty(t, logs.String())

	loaded, err := c.GetDeployedPackage(ctx, deployed.Name)
	require.NoError(t, err)
	definition, err := loaded.Definition()
	require.NoError(t, err)
	require.Equal(t, v1alpha1.APIVersion, definition.GetAPIVersion())
	require.Contains(t, logs.String(), `"level":"WARN"`)
	require.Contains(t, logs.String(), futureVersion)
	require.Contains(t, logs.String(), "this version of Zarf does not recognize")

	logs.Reset()
	listed, err := c.GetDeployedZarfPackages(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 2)
	require.Contains(t, logs.String(), `"level":"WARN"`)
	require.Contains(t, logs.String(), futureVersion)
}

func TestRecordPackageDefinitionDeployment(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	c := &Cluster{Clientset: fake.NewClientset()}
	definition := convert.PackageFromV1beta1(v1beta1.Package{
		APIVersion: v1beta1.APIVersion,
		Metadata:   v1beta1.PackageMetadata{Name: "beta-package", Version: "1.2.3"},
	})

	recorded, err := c.RecordPackageDeployment(ctx, definition, "sha256:abcdeadbeef", nil, 1)
	require.NoError(t, err)
	recordedDefinition, err := recorded.Definition()
	require.NoError(t, err)
	require.Equal(t, convert.PackageToV1alpha1(definition), convert.PackageToV1alpha1(recordedDefinition))
	require.Contains(t, recorded.PackageData, v1alpha1.APIVersion)
	require.Contains(t, recorded.PackageData, v1beta1.APIVersion)

	loaded, err := c.GetDeployedPackage(ctx, "beta-package")
	require.NoError(t, err)
	loadedDefinition, err := loaded.Definition()
	require.NoError(t, err)
	require.Equal(t, convert.PackageToV1beta1(definition), convert.PackageToV1beta1(loadedDefinition))
}

func TestRequireServiceCapability(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name           string
		annotation     string
		packageService api.Service
		deployed       bool
		legacyData     bool
		wantError      string
	}{
		{name: "no deployed service", wantError: `no deployed package providing service "git-server"`},
		{name: "older init package", packageService: api.ServiceGitServer, deployed: true, legacyData: true, wantError: "git-server-tls/v1=enabled"},
		{name: "disabled capability", packageService: api.ServiceGitServer, annotation: "disabled", deployed: true, wantError: "git-server-tls/v1=enabled"},
		{name: "different service", packageService: api.ServiceRegistry, annotation: api.CapabilityEnabled, deployed: true, wantError: `no deployed package providing service "git-server"`},
		{name: "enabled capability", packageService: api.ServiceGitServer, annotation: api.CapabilityEnabled, deployed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			c := &Cluster{Clientset: fake.NewClientset()}
			if tc.deployed {
				metadata := api.PackageMetadata{Name: "init"}
				if tc.annotation != "" {
					metadata.Annotations = map[string]string{string(api.CapabilityGitServerTLSV1): tc.annotation}
				}
				componentName := "git-server"
				if tc.packageService == api.ServiceRegistry {
					componentName = "zarf-registry"
				}
				deployed, err := c.RecordPackageDeployment(ctx, api.Package{
					Kind:       api.ZarfInitConfig,
					Metadata:   metadata,
					Components: []api.Component{{Name: componentName, Service: tc.packageService}},
				}, "sha256:abc", nil, 1)
				require.NoError(t, err)
				if tc.legacyData {
					deployed.PackageData = nil
					require.NoError(t, c.UpdateDeployedPackage(ctx, *deployed))
				}
			}
			err := c.RequireServiceCapability(ctx, api.ServiceGitServer, api.CapabilityGitServerTLSV1)
			if tc.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantError)
			}
		})
	}
}

func TestGetInstalledChartsForComponentNamespaceOverride(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := &Cluster{Clientset: fake.NewClientset()}

	componentName := "games"
	packageName := "dos-games"

	packages := []state.DeployedPackage{
		{
			Name: packageName,
			DeployedComponents: []state.DeployedComponent{{
				Name: componentName,
				InstalledCharts: []state.InstalledChart{
					{Namespace: "dos-games", ChartName: "zarf-original"},
				},
			}},
		},
		{
			Name:              packageName,
			NamespaceOverride: "arcade-alt",
			DeployedComponents: []state.DeployedComponent{{
				Name: componentName,
				InstalledCharts: []state.InstalledChart{
					{Namespace: "arcade-alt", ChartName: "zarf-override"},
				},
			}},
		},
	}

	for _, p := range packages {
		b, err := json.Marshal(p)
		require.NoError(t, err)
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      p.GetSecretName(),
				Namespace: "zarf",
				Labels:    map[string]string{state.ZarfPackageInfoLabel: p.Name},
			},
			Data: map[string][]byte{"data": b},
		}
		_, err = c.Clientset.CoreV1().Secrets("zarf").Create(ctx, &secret, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	component := api.Component{Name: componentName}

	originalCharts, err := c.GetInstalledChartsForComponent(ctx, packageName, component)
	require.NoError(t, err)
	require.Equal(t, []state.InstalledChart{{Namespace: "dos-games", ChartName: "zarf-original"}}, originalCharts)

	overrideCharts, err := c.GetInstalledChartsForComponent(ctx, packageName, component, state.WithPackageNamespaceOverride("arcade-alt"))
	require.NoError(t, err)
	require.Equal(t, []state.InstalledChart{{Namespace: "arcade-alt", ChartName: "zarf-override"}}, overrideCharts)
}

func TestGetDeployedPackage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := &Cluster{
		Clientset: fake.NewClientset(),
	}

	packages := []state.DeployedPackage{
		{Name: "package1"},
		{Name: "package2", NamespaceOverride: "test2"},
	}

	for _, p := range packages {
		b, err := json.Marshal(p)
		require.NoError(t, err)
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      p.GetSecretName(),
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
		actual, err := c.GetDeployedPackage(ctx, p.Name, state.WithPackageNamespaceOverride(p.NamespaceOverride))
		require.NoError(t, err)
		require.Equal(t, p, *actual)
	}

	nonPackageSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hello-world",
			Namespace: "zarf",
			Labels: map[string]string{
				state.ZarfPackageInfoLabel: "whatever",
			},
		},
	}
	_, err := c.Clientset.CoreV1().Secrets("zarf").Create(ctx, &nonPackageSecret, metav1.CreateOptions{})
	require.NoError(t, err)

	actualList, err := c.GetDeployedZarfPackages(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, packages, actualList)
}

func TestInternalGitServerExists(t *testing.T) {
	tests := []struct {
		name          string
		svc           *corev1.Service
		expectedExist bool
		expectedErr   error
	}{
		{
			name:          "Git server exists",
			svc:           &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: ZarfGitServerName, Namespace: state.ZarfNamespaceName}},
			expectedExist: true,
			expectedErr:   nil,
		},
		{
			name:          "Git server does not exist",
			svc:           nil,
			expectedExist: false,
			expectedErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := fake.NewClientset()
			c := &Cluster{Clientset: cs}
			ctx := context.Background()
			if tt.svc != nil {
				_, err := cs.CoreV1().Services(tt.svc.Namespace).Create(ctx, tt.svc, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			exists, err := c.InternalGitServerExists(ctx)
			require.Equal(t, tt.expectedExist, exists)
			require.Equal(t, tt.expectedErr, err)
		})
	}
}
