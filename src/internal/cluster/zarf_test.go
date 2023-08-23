// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"encoding/json"
	"testing"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func marshalDeployedPackage(deployedPackage *types.DeployedPackage) (rawData []byte) {
	rawData, _ = json.Marshal(deployedPackage)
	return rawData
}

// TestPackageSecretNeedsWait verifies that Zarf waits for webhooks to complete correctly.
func TestPackageSecretNeedsWait(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		secret        *corev1.Secret
		component     types.ZarfComponent
		needsWait     bool
		waitSeconds   int
		hookName      string
		expectedError error
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
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"data": marshalDeployedPackage(&types.DeployedPackage{
						Name:              packageName,
						ComponentWebhooks: map[string]map[string]types.Webhook{},
					}),
				},
			},
			needsWait:     false,
			waitSeconds:   0,
			hookName:      "",
			expectedError: nil,
		},
		{
			name:      "WebhookRunning",
			component: types.ZarfComponent{Name: componentName},
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"data": marshalDeployedPackage(&types.DeployedPackage{
						Name: packageName,
						ComponentWebhooks: map[string]map[string]types.Webhook{
							componentName: {
								webhookName: types.Webhook{
									Status:              string(types.WebhookStatusRunning),
									WaitDurationSeconds: 10,
								},
							},
						},
					}),
				},
			},
			needsWait:     true,
			waitSeconds:   10,
			hookName:      webhookName,
			expectedError: nil,
		},
		// Ensure we only wait on running webhooks for the provided component
		{
			name:      "WebhookRunningOnDifferentComponent",
			component: types.ZarfComponent{Name: componentName},
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"data": marshalDeployedPackage(&types.DeployedPackage{
						Name: packageName,
						ComponentWebhooks: map[string]map[string]types.Webhook{
							"different-component": {
								webhookName: types.Webhook{
									Status:              string(types.WebhookStatusRunning),
									WaitDurationSeconds: 10,
								},
							},
						},
					}),
				},
			},
			needsWait:     false,
			waitSeconds:   0,
			hookName:      "",
			expectedError: nil,
		},
		{
			name:      "WebhookSucceeded",
			component: types.ZarfComponent{Name: componentName},
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"data": marshalDeployedPackage(&types.DeployedPackage{
						Name: packageName,
						ComponentWebhooks: map[string]map[string]types.Webhook{
							componentName: {
								webhookName: types.Webhook{
									Status: string(types.WebhookStatusSucceeded),
								},
							},
						},
					}),
				},
			},
			needsWait:     false,
			waitSeconds:   0,
			hookName:      "",
			expectedError: nil,
		},
		{
			name:      "WebhookFailed",
			component: types.ZarfComponent{Name: componentName},
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"data": marshalDeployedPackage(&types.DeployedPackage{
						Name: packageName,
						ComponentWebhooks: map[string]map[string]types.Webhook{
							componentName: {
								webhookName: types.Webhook{
									Status: string(types.WebhookStatusFailed),
								},
							},
						},
					}),
				},
			},
			needsWait:     false,
			waitSeconds:   0,
			hookName:      "",
			expectedError: nil,
		},
		{
			name:      "WebhookRemoving",
			component: types.ZarfComponent{Name: componentName},
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"data": marshalDeployedPackage(&types.DeployedPackage{
						Name: packageName,
						ComponentWebhooks: map[string]map[string]types.Webhook{
							componentName: {
								webhookName: types.Webhook{
									Status: string(types.WebhookStatusRemoving),
								},
							},
						},
					}),
				},
			},
			needsWait:     false,
			waitSeconds:   0,
			hookName:      "",
			expectedError: nil,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			c := &Cluster{}

			needsWait, waitSeconds, hookName, err := c.PackageSecretNeedsWait(testCase.secret, testCase.component)

			require.Equal(t, testCase.needsWait, needsWait)
			require.Equal(t, testCase.waitSeconds, waitSeconds)
			require.Equal(t, testCase.hookName, hookName)
			require.Equal(t, testCase.expectedError, err)
		})
	}
}
