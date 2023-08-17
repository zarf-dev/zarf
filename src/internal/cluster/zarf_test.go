// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"errors"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8sTesting "k8s.io/client-go/testing"
)

// TestPackageSecretNeedsWait verifies that Zarf waits for webhooks to complete correctly.
func TestPackageSecretNeedsWait(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		secretName    string
		webhookStatus *corev1.Secret
		needsWait     bool
		waitSeconds   int
		expectedError error
	}

	testCases := []testCase{
		{
			name:       "NoWebhooks",
			secretName: "test-secret",
			webhookStatus: &corev1.Secret{
				Data: map[string][]byte{
					"data": []byte(`{
						"name": "test-package",
						"data": {},
						"cliVersion": "1.0",
						"generation": 1,
						"deployedComponents": [],
						"componentWebhooks": {},
						"connectStrings": {}
					}`),
				},
			},
			needsWait:     false,
			waitSeconds:   0,
			expectedError: nil,
		},
		{
			name:       "WebhookRunning",
			secretName: "test-secret",
			webhookStatus: &corev1.Secret{
				Data: map[string][]byte{
					"data": []byte(`{
						"name": "test-package",
						"data": {},
						"cliVersion": "1.0",
						"generation": 1,
						"deployedComponents": [],
						"componentWebhooks": {
							"componentA": {
								"webhookA": {
									"status": "Running",
									"waitDurationSeconds": 10
								}
							}
						},
						"connectStrings": {}
					}`),
				},
			},
			needsWait:     true,
			waitSeconds:   10,
			expectedError: nil,
		},
		{
			name:       "WebhookSucceeded",
			secretName: "test-secret",
			webhookStatus: &corev1.Secret{
				Data: map[string][]byte{
					"data": []byte(`{
						"name": "test-package",
						"data": {},
						"cliVersion": "1.0",
						"generation": 1,
						"deployedComponents": [],
						"componentWebhooks": {
							"componentA": {
								"webhookA": {
									"status": "Succeeded"
								}
							}
						},
						"connectStrings": {}
					}`),
				},
			},
			needsWait:     false,
			waitSeconds:   0,
			expectedError: nil,
		},
		{
			name:       "WebhookFailed",
			secretName: "test-secret",
			webhookStatus: &corev1.Secret{
				Data: map[string][]byte{
					"data": []byte(`{
						"name": "test-package",
						"data": {},
						"cliVersion": "1.0",
						"generation": 1,
						"deployedComponents": [],
						"componentWebhooks": {
							"componentA": {
								"webhookA": {
									"status": "Failed"
								}
							}
						},
						"connectStrings": {}
					}`),
				},
			},
			needsWait:     false,
			waitSeconds:   0,
			expectedError: nil,
		},
		{
			name:       "WebhookRemoving",
			secretName: "test-secret",
			webhookStatus: &corev1.Secret{
				Data: map[string][]byte{
					"data": []byte(`{
						"name": "test-package",
						"data": {},
						"cliVersion": "1.0",
						"generation": 1,
						"deployedComponents": [],
						"componentWebhooks": {
							"componentA": {
								"webhookA": {
									"status": "Removing"
								}
							}
						},
						"connectStrings": {}
					}`),
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
				if action.(k8sTesting.GetAction).GetName() == testCase.secretName {
					return true, testCase.webhookStatus, nil
				}
				return false, nil, errors.New("actual secret name does not equal expected secret name")
			})

			needsWait, waitSeconds, err := c.PackageSecretNeedsWait(testCase.secretName)

			require.Equal(t, testCase.needsWait, needsWait)
			require.Equal(t, testCase.waitSeconds, waitSeconds)
			require.Equal(t, testCase.expectedError, err)
		})
	}
}
