// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cluster

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
)

func TestUpdateZarfManagedSecrets(t *testing.T) {
	ctx := testutil.TestContext(t)

	tests := []struct {
		name               string
		namespaceLabels    map[string]string
		secretLabels       map[string]string
		updatedImageSecret bool
		updatedGitSecret   bool
	}{
		{
			name:               "modify",
			updatedImageSecret: true,
			updatedGitSecret:   true,
		},
		{
			name: "skip namespace",
			namespaceLabels: map[string]string{
				AgentLabel: "skip",
			},
		},
		{
			name: "ignore namespace",
			namespaceLabels: map[string]string{
				AgentLabel: "ignore",
			},
		},
		{
			name: "skip namespace managed secret",
			namespaceLabels: map[string]string{
				AgentLabel: "skip",
			},
			secretLabels: map[string]string{
				ZarfManagedByLabel: "zarf",
			},
			updatedImageSecret: true,
			updatedGitSecret:   true,
		},
		{
			name: "ignore namespace managed secret",
			namespaceLabels: map[string]string{
				AgentLabel: "ignore",
			},
			secretLabels: map[string]string{
				ZarfManagedByLabel: "zarf",
			},
			updatedImageSecret: true,
			updatedGitSecret:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cluster{
				Clientset: fake.NewSimpleClientset(),
			}

			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test",
					Labels: tt.namespaceLabels,
				},
			}
			_, err := c.Clientset.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
			require.NoError(t, err)
			svc := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "good-service",
					Namespace: namespace.ObjectMeta.Name,
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
					Ports: []corev1.ServicePort{
						{
							NodePort: 30001,
							Port:     3333,
						},
					},
					ClusterIP: "10.11.12.13",
				},
			}
			_, err = c.Clientset.CoreV1().Services(namespace.ObjectMeta.Name).Create(ctx, svc, metav1.CreateOptions{})
			require.NoError(t, err)
			imageSecret := &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.ZarfImagePullSecretName,
					Namespace: namespace.ObjectMeta.Name,
					Labels:    tt.secretLabels,
				},
			}
			_, err = c.Clientset.CoreV1().Secrets(imageSecret.ObjectMeta.Namespace).Create(ctx, imageSecret, metav1.CreateOptions{})
			require.NoError(t, err)
			gitSecret := &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.ZarfGitServerSecretName,
					Namespace: namespace.ObjectMeta.Name,
					Labels:    tt.secretLabels,
				},
			}
			_, err = c.Clientset.CoreV1().Secrets(gitSecret.ObjectMeta.Namespace).Create(ctx, gitSecret, metav1.CreateOptions{})
			require.NoError(t, err)

			state := &types.ZarfState{
				GitServer: types.GitServerInfo{
					PullUsername: "pull-user",
					PullPassword: "pull-password",
				},
				RegistryInfo: types.RegistryInfo{
					PullUsername: "pull-user",
					PullPassword: "pull-password",
					Address:      "127.0.0.1:30001",
				},
			}
			err = c.UpdateZarfManagedImageSecrets(ctx, state)
			require.NoError(t, err)
			err = c.UpdateZarfManagedGitSecrets(ctx, state)
			require.NoError(t, err)

			// Make sure no new namespaces or secrets have been created.
			namespaceList, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
			require.NoError(t, err)
			require.Len(t, namespaceList.Items, 1)
			for _, ns := range namespaceList.Items {
				secretList, err := c.Clientset.CoreV1().Secrets(ns.ObjectMeta.Name).List(ctx, metav1.ListOptions{})
				require.NoError(t, err)
				require.Len(t, secretList.Items, 2)
			}

			// Check image registry secret
			updatedImageSecret, err := c.Clientset.CoreV1().Secrets(namespace.ObjectMeta.Name).Get(ctx, config.ZarfImagePullSecretName, metav1.GetOptions{})
			require.NoError(t, err)
			expectedImageSecret := corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.ZarfImagePullSecretName,
					Namespace: namespace.ObjectMeta.Name,
					Labels: map[string]string{
						ZarfManagedByLabel: "zarf",
					},
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					".dockerconfigjson": []byte(`{"auths":{"10.11.12.13:3333":{"auth":"cHVsbC11c2VyOnB1bGwtcGFzc3dvcmQ="},"127.0.0.1:30001":{"auth":"cHVsbC11c2VyOnB1bGwtcGFzc3dvcmQ="}}}`),
				},
			}
			if !tt.updatedImageSecret {
				expectedImageSecret = *imageSecret
			}
			require.Equal(t, expectedImageSecret, *updatedImageSecret)

			// Check git secret
			updatedGitSecret, err := c.Clientset.CoreV1().Secrets(namespace.ObjectMeta.Name).Get(ctx, config.ZarfGitServerSecretName, metav1.GetOptions{})
			require.NoError(t, err)
			expectedGitSecret := corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.ZarfGitServerSecretName,
					Namespace: namespace.ObjectMeta.Name,
					Labels: map[string]string{
						ZarfManagedByLabel: "zarf",
					},
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{},
				StringData: map[string]string{
					"username": state.GitServer.PullUsername,
					"password": state.GitServer.PullPassword,
				},
			}
			if !tt.updatedGitSecret {
				expectedGitSecret = *gitSecret
			}
			require.Equal(t, expectedGitSecret, *updatedGitSecret)
		})
	}
}
