// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8sTesting "k8s.io/client-go/testing"
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
		needsWait     bool
		waitSeconds   int
		expectedError error
	}

	var (
		componentName = "test-component"
		packageName   = "test-package"
		secretName    = "test-secret"
		webhookName   = "test-webhook"
	)

	testCases := []testCase{
		{
			name: "NoWebhooks",
			secret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name: secretName,
				},
				Data: map[string][]byte{
					"data": marshalDeployedPackage(&types.DeployedPackage{
						Name:              packageName,
						ComponentWebhooks: map[string]map[string]types.Webhook{},
					}),
				},
			},
			needsWait:     false,
			waitSeconds:   0,
			expectedError: nil,
		},
		{
			name: "WebhookRunning",
			secret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name: secretName,
				},
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
			expectedError: nil,
		},
		{
			name: "WebhookSucceeded",
			secret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name: secretName,
				},
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
			expectedError: nil,
		},
		{
			name: "WebhookFailed",
			secret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name: secretName,
				},
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
			expectedError: nil,
		},
		{
			name: "WebhookRemoving",
			secret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name: secretName,
				},
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
			expectedError: nil,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Create a mock client and set up a GetSecret call.
			mockClient := fake.NewSimpleClientset()

			c := &Cluster{
				K8s: &k8s.K8s{
					Clientset: mockClient,
				},
			}

			mockClient.PrependReactor("get", "secrets", func(action k8sTesting.Action) (handled bool, ret runtime.Object, err error) {
				if action.(k8sTesting.GetAction).GetName() == testCase.secret.Name {
					return true, testCase.secret, nil
				}
				return false, nil, errors.New("actual secret name does not equal expected secret name")
			})

			needsWait, waitSeconds, err := c.PackageSecretNeedsWait(testCase.secret)

			require.Equal(t, testCase.needsWait, needsWait)
			require.Equal(t, testCase.waitSeconds, waitSeconds)
			require.Equal(t, testCase.expectedError, err)
		})
	}
}
