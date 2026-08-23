// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

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

	component := v1alpha1.ZarfComponent{Name: componentName}

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
