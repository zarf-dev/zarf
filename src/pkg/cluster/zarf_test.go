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
	"time"

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
	legacy := v1alpha1.ZarfPackage{
		APIVersion: v1alpha1.APIVersion,
		Metadata:   v1alpha1.ZarfMetadata{Name: "future-package"},
	}
	deployed := state.DeployedPackage{
		Name: "future-package",
		Data: legacy,
		PackageData: map[string]json.RawMessage{
			futureVersion: json.RawMessage(`{"apiVersion":"zarf.dev/v9"}`),
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
	known.PackageData = nil
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
	require.NotContains(t, recorded.PackageData, v1alpha1.APIVersion)
	require.Contains(t, recorded.PackageData, v1beta1.APIVersion)
	require.Equal(t, convert.PackageToV1alpha1(definition), recorded.Data)

	secret, err := c.Clientset.CoreV1().Secrets(state.ZarfNamespaceName).Get(ctx, recorded.GetSecretName(), metav1.GetOptions{})
	require.NoError(t, err)
	var serialized struct {
		Data        v1alpha1.ZarfPackage       `json:"data"`
		PackageData map[string]json.RawMessage `json:"packageData"`
	}
	require.NoError(t, json.Unmarshal(secret.Data["data"], &serialized))
	require.Equal(t, convert.PackageToV1alpha1(definition), serialized.Data)
	require.Len(t, serialized.PackageData, 1)
	require.Contains(t, serialized.PackageData, v1beta1.APIVersion)

	loaded, err := c.GetDeployedPackage(ctx, "beta-package")
	require.NoError(t, err)
	loadedDefinition, err := loaded.Definition()
	require.NoError(t, err)
	require.Equal(t, convert.PackageToV1beta1(definition), convert.PackageToV1beta1(loadedDefinition))
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

func TestUpdateDeployedPackagePreservesLifecycleState(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	c := &Cluster{Clientset: fake.NewClientset()}
	definition := convert.PackageFromV1beta1(v1beta1.Package{
		APIVersion: v1beta1.APIVersion,
		Metadata:   v1beta1.PackageMetadata{Name: "lifecycle-package", Version: "1.2.3"},
	})
	deployed, err := state.NewDeployedPackage(definition, "sha256:manifest", "v1", []state.DeployedComponent{{
		Name:      "component",
		Status:    state.ComponentStatusDeploying,
		LastEvent: state.ComponentEvent{Type: state.EventTypeDeploy, Outcome: state.EventOutcomeInProgress, Timestamp: time.Unix(1, 0).UTC()},
	}}, 1)
	require.NoError(t, err)
	deployed.AppendEvent(state.PackageEvent{Type: state.EventTypeDeploy, Outcome: state.EventOutcomeSucceeded, Timestamp: time.Unix(2, 0).UTC()})
	deployed.SetDeployConfig(state.DeployConfig{SetVariables: map[string]string{"VARIABLE": "value"}})
	require.NoError(t, c.UpdateDeployedPackage(ctx, *deployed))

	deployed.DeployedComponents[0].Status = state.ComponentStatusSucceeded //nolint:staticcheck // Verify legacy status remains serialized.
	deployed.DeployedComponents[0].LastEvent = state.ComponentEvent{Type: state.EventTypeDeploy, Outcome: state.EventOutcomeSucceeded, Timestamp: time.Unix(3, 0).UTC()}
	require.NoError(t, c.UpdateDeployedPackage(ctx, *deployed))

	loaded, err := c.GetDeployedPackage(ctx, definition.Metadata.Name)
	require.NoError(t, err)
	require.Equal(t, deployed.Events, loaded.Events)
	require.Equal(t, deployed.DeployConfig, loaded.DeployConfig)
	require.Equal(t, deployed.DeployedComponents[0].LastEvent, loaded.DeployedComponents[0].LastEvent)

	secret, err := c.Clientset.CoreV1().Secrets(state.ZarfNamespaceName).Get(ctx, deployed.GetSecretName(), metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, state.ZarfNamespaceName, secret.Namespace)
	require.Equal(t, map[string]string{
		state.ZarfManagedByLabel:   "zarf",
		state.ZarfPackageInfoLabel: definition.Metadata.Name,
	}, secret.Labels)
	require.Contains(t, secret.Data, "data")
}
