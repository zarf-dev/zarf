// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/types"
)

func TestGetDeployedPackage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := &Cluster{
		Clientset: fake.NewClientset(),
	}

	packages := []types.DeployedPackage{
		{Name: "package1"},
		{Name: "package2"},
	}

	for _, p := range packages {
		b, err := json.Marshal(p)
		require.NoError(t, err)
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      strings.Join([]string{config.ZarfPackagePrefix, p.Name}, ""),
				Namespace: "zarf",
				Labels: map[string]string{
					ZarfPackageInfoLabel: p.Name,
				},
			},
			Data: map[string][]byte{
				"data": b,
			},
		}
		_, err = c.Clientset.CoreV1().Secrets("zarf").Create(ctx, &secret, metav1.CreateOptions{})
		require.NoError(t, err)
		actual, err := c.GetDeployedPackage(ctx, p.Name)
		require.NoError(t, err)
		require.Equal(t, p, *actual)
	}

	nonPackageSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hello-world",
			Namespace: "zarf",
			Labels: map[string]string{
				ZarfPackageInfoLabel: "whatever",
			},
		},
	}
	_, err := c.Clientset.CoreV1().Secrets("zarf").Create(ctx, &nonPackageSecret, metav1.CreateOptions{})
	require.NoError(t, err)

	actualList, err := c.GetDeployedZarfPackages(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, packages, actualList)
}

func TestRegistryHPA(t *testing.T) {
	ctx := context.Background()
	cs := fake.NewClientset()
	hpa := autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zarf-docker-registry",
			Namespace: ZarfNamespaceName,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
				ScaleDown: &autoscalingv2.HPAScalingRules{},
			},
		},
	}
	_, err := cs.AutoscalingV2().HorizontalPodAutoscalers(hpa.Namespace).Create(ctx, &hpa, metav1.CreateOptions{})
	require.NoError(t, err)
	c := &Cluster{
		Clientset: cs,
	}

	err = c.EnableRegHPAScaleDown(ctx)
	require.NoError(t, err)
	enableHpa, err := cs.AutoscalingV2().HorizontalPodAutoscalers(hpa.Namespace).Get(ctx, hpa.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, autoscalingv2.MinChangePolicySelect, *enableHpa.Spec.Behavior.ScaleDown.SelectPolicy)

	err = c.DisableRegHPAScaleDown(ctx)
	require.NoError(t, err)
	disableHpa, err := cs.AutoscalingV2().HorizontalPodAutoscalers(hpa.Namespace).Get(ctx, hpa.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, autoscalingv2.DisabledPolicySelect, *disableHpa.Spec.Behavior.ScaleDown.SelectPolicy)
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
			svc:           &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: ZarfGitServerName, Namespace: ZarfNamespaceName}},
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
