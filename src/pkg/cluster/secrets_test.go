// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cluster

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/defenseunicorns/zarf/src/types"
)

func TestGenerateRegistryPullCredsWithOutSvc(t *testing.T) {
	c := &Cluster{Clientset: fake.NewSimpleClientset()}
	ctx := context.Background()
	ri := types.RegistryInfo{
		PullUsername: "pull-user",
		PullPassword: "pull-password",
		Address:      "example.com",
	}
	secret, err := c.GenerateRegistryPullCreds(ctx, "foo", "bar", ri)
	require.NoError(t, err)
	expectedSecret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "foo",
			Labels: map[string]string{
				ZarfManagedByLabel: "zarf",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{"example.com":{"auth":"cHVsbC11c2VyOnB1bGwtcGFzc3dvcmQ="}}}`),
		},
	}
	require.Equal(t, expectedSecret, *secret)
}

func TestGenerateRegistryPullCredsWithSvc(t *testing.T) {
	c := &Cluster{Clientset: fake.NewSimpleClientset()}
	ctx := context.Background()
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "good-service",
			Namespace: "whatever",
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

	_, err := c.Clientset.CoreV1().Services("whatever").Create(ctx, svc, metav1.CreateOptions{})
	require.NoError(t, err)

	ri := types.RegistryInfo{
		PullUsername: "pull-user",
		PullPassword: "pull-password",
		Address:      "127.0.0.1:30001",
	}
	secret, err := c.GenerateRegistryPullCreds(ctx, "foo", "bar", ri)
	require.NoError(t, err)
	expectedSecret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "foo",
			Labels: map[string]string{
				ZarfManagedByLabel: "zarf",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{"10.11.12.13:3333":{"auth":"cHVsbC11c2VyOnB1bGwtcGFzc3dvcmQ="},"127.0.0.1:30001":{"auth":"cHVsbC11c2VyOnB1bGwtcGFzc3dvcmQ="}}}`),
		},
	}
	require.Equal(t, expectedSecret, *secret)
}

func TestGenerateGitPullCreds(t *testing.T) {
	c := &Cluster{}
	gi := types.GitServerInfo{
		PullUsername: "pull-user",
		PullPassword: "pull-password",
	}
	secret := c.GenerateGitPullCreds("foo", "bar", gi)
	expectedSecret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "foo",
			Labels: map[string]string{
				ZarfManagedByLabel: "zarf",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{},
		StringData: map[string]string{
			"username": "pull-user",
			"password": "pull-password",
		},
	}
	require.Equal(t, expectedSecret, *secret)
}
