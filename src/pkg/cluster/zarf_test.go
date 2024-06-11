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

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/types"
)

// TestPackageSecretNeedsWait verifies that Zarf waits for webhooks to complete correctly.
func TestPackageSecretNeedsWait(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		deployedPackage *types.DeployedPackage
		component       types.ZarfComponent
		skipWebhooks    bool
		needsWait       bool
		waitSeconds     int
		hookName        string
	}

	var (
		componentName = "test-component"
		packageName   = "test-package"
		webhookName   = "test-webhook"
	)

	testCases := []testCase{
		{
			name:      "NoWebhooks",
			component: types.ZarfComponent{Name: componentName},
			deployedPackage: &types.DeployedPackage{
				Name:              packageName,
				ComponentWebhooks: map[string]map[string]types.Webhook{},
			},
			needsWait:   false,
			waitSeconds: 0,
			hookName:    "",
		},
		{
			name:      "WebhookRunning",
			component: types.ZarfComponent{Name: componentName},
			deployedPackage: &types.DeployedPackage{
				Name: packageName,
				ComponentWebhooks: map[string]map[string]types.Webhook{
					componentName: {
						webhookName: types.Webhook{
							Status:              types.WebhookStatusRunning,
							WaitDurationSeconds: 10,
						},
					},
				},
			},
			needsWait:   true,
			waitSeconds: 10,
			hookName:    webhookName,
		},
		// Ensure we only wait on running webhooks for the provided component
		{
			name:      "WebhookRunningOnDifferentComponent",
			component: types.ZarfComponent{Name: componentName},
			deployedPackage: &types.DeployedPackage{
				Name: packageName,
				ComponentWebhooks: map[string]map[string]types.Webhook{
					"different-component": {
						webhookName: types.Webhook{
							Status:              types.WebhookStatusRunning,
							WaitDurationSeconds: 10,
						},
					},
				},
			},
			needsWait:   false,
			waitSeconds: 0,
			hookName:    "",
		},
		{
			name:      "WebhookSucceeded",
			component: types.ZarfComponent{Name: componentName},
			deployedPackage: &types.DeployedPackage{
				Name: packageName,
				ComponentWebhooks: map[string]map[string]types.Webhook{
					componentName: {
						webhookName: types.Webhook{
							Status: types.WebhookStatusSucceeded,
						},
					},
				},
			},
			needsWait:   false,
			waitSeconds: 0,
			hookName:    "",
		},
		{
			name:      "WebhookFailed",
			component: types.ZarfComponent{Name: componentName},
			deployedPackage: &types.DeployedPackage{
				Name: packageName,
				ComponentWebhooks: map[string]map[string]types.Webhook{
					componentName: {
						webhookName: types.Webhook{
							Status: types.WebhookStatusFailed,
						},
					},
				},
			},
			needsWait:   false,
			waitSeconds: 0,
			hookName:    "",
		},
		{
			name:      "WebhookRemoving",
			component: types.ZarfComponent{Name: componentName},
			deployedPackage: &types.DeployedPackage{
				Name: packageName,
				ComponentWebhooks: map[string]map[string]types.Webhook{
					componentName: {
						webhookName: types.Webhook{
							Status: types.WebhookStatusRemoving,
						},
					},
				},
			},
			needsWait:   false,
			waitSeconds: 0,
			hookName:    "",
		},
		{
			name:      "SkipWaitForYOLO",
			component: types.ZarfComponent{Name: componentName},
			deployedPackage: &types.DeployedPackage{
				Name: packageName,
				Data: types.ZarfPackage{
					Metadata: types.ZarfMetadata{
						YOLO: true,
					},
				},
				ComponentWebhooks: map[string]map[string]types.Webhook{
					componentName: {
						webhookName: types.Webhook{
							Status:              types.WebhookStatusRunning,
							WaitDurationSeconds: 10,
						},
					},
				},
			},
			needsWait:   false,
			waitSeconds: 0,
			hookName:    "",
		},
		{
			name:         "SkipWebhooksFlagUsed",
			component:    types.ZarfComponent{Name: componentName},
			skipWebhooks: true,
			deployedPackage: &types.DeployedPackage{
				Name: packageName,
				ComponentWebhooks: map[string]map[string]types.Webhook{
					componentName: {
						webhookName: types.Webhook{
							Status:              types.WebhookStatusRunning,
							WaitDurationSeconds: 10,
						},
					},
				},
			},
			needsWait:   false,
			waitSeconds: 0,
			hookName:    "",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			c := &Cluster{}

			needsWait, waitSeconds, hookName := c.PackageSecretNeedsWait(testCase.deployedPackage, testCase.component, testCase.skipWebhooks)

			require.Equal(t, testCase.needsWait, needsWait)
			require.Equal(t, testCase.waitSeconds, waitSeconds)
			require.Equal(t, testCase.hookName, hookName)
		})
	}
}

func TestGetDeployedPackage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := &Cluster{&k8s.K8s{Clientset: fake.NewSimpleClientset()}}

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
	cs := fake.NewSimpleClientset()
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
		&k8s.K8s{
			Clientset: cs,
		},
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
