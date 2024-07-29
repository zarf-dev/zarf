// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cluster

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestUpdateGiteaPVC(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	c := &Cluster{
		Clientset: fake.NewSimpleClientset(),
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "data-zarf-gitea-0",
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}
	_, err := c.Clientset.CoreV1().PersistentVolumeClaims(ZarfNamespaceName).Create(ctx, pvc, metav1.CreateOptions{})
	require.NoError(t, err)

	v, err := c.UpdateGiteaPVC(ctx, "foobar", false)
	require.NoError(t, err)
	require.Equal(t, "false", v)

	v, err = c.UpdateGiteaPVC(ctx, "foobar", true)
	require.EqualError(t, err, "persistentvolumeclaims \"foobar\" not found")
	require.Equal(t, "false", v)

	v, err = c.UpdateGiteaPVC(ctx, "data-zarf-gitea-0", true)
	require.NoError(t, err)
	require.Equal(t, "false", v)

	v, err = c.UpdateGiteaPVC(ctx, "data-zarf-gitea-0", false)
	require.NoError(t, err)
	require.Equal(t, "true", v)
	pvc, err = c.Clientset.CoreV1().PersistentVolumeClaims(ZarfNamespaceName).Get(ctx, "data-zarf-gitea-0", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "Helm", pvc.Labels["app.kubernetes.io/managed-by"])
	require.Equal(t, "zarf-gitea", pvc.Annotations["meta.helm.sh/release-name"])
	require.Equal(t, "zarf", pvc.Annotations["meta.helm.sh/release-namespace"])

	v, err = c.UpdateGiteaPVC(ctx, "data-zarf-gitea-0", true)
	require.NoError(t, err)
	require.Equal(t, "false", v)
	pvc, err = c.Clientset.CoreV1().PersistentVolumeClaims(ZarfNamespaceName).Get(ctx, "data-zarf-gitea-0", metav1.GetOptions{})
	require.NoError(t, err)
	require.Empty(t, pvc.Labels["app.kubernetes.io/managed-by"])
	require.Empty(t, pvc.Labels["meta.helm.sh/release-name"])
	require.Empty(t, pvc.Labels["meta.helm.sh/release-namespace"])
}
