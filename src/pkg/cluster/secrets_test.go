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

	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/types"
)

func TestGenerateRegistryPullCreds(t *testing.T) {
	c := &Cluster{K8s: &k8s.K8s{Clientset: fake.NewSimpleClientset()}}
	ctx := context.Background()
	// ns := corev1.Namespace{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name: "bar",
	// 	},
	// }

	// c.K8s.Clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	ri := types.RegistryInfo{
		PushUsername: "push-user",
		PushPassword: "push-password",
		PullUsername: "pull-user",
		PullPassword: "pull-password",
		Address:      "example.com",
	}
	secret := c.GenerateRegistryPullCreds(ctx, "foo", "bar", ri)
	expectedSecret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "foo",
			Labels: map[string]string{
				k8s.ZarfManagedByLabel: "zarf",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{"example.com":{"auth":"cHVsbC11c2VyOnB1bGwtcGFzc3dvcmQ="}}}`),
		},
	}
	require.Equal(t, expectedSecret, *secret)
}

func TestGenerateGitPullCreds(t *testing.T) {
	c := &Cluster{}
	gi := types.GitServerInfo{
		PushUsername: "push-user",
		PushPassword: "push-password",
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
				k8s.ZarfManagedByLabel: "zarf",
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
