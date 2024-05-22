// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cluster

import (
	"context"
	"testing"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCheckForZarfConnectLabel(t *testing.T) {
	ctx := context.Background()
	cs := fake.NewSimpleClientset()
	c := &Cluster{
		K8s: &k8s.K8s{
			Clientset: cs,
		},
	}

	svcs := []corev1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "missing-label",
				Namespace: "",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wrong-label",
				Namespace: "",
				Labels: map[string]string{
					config.ZarfConnectLabelName: "wrong",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "good-service",
				Namespace: "good-namespace",
				Labels: map[string]string{
					config.ZarfConnectLabelName: "good",
				},
				Annotations: map[string]string{
					config.ZarfConnectAnnotationURL: "foobar",
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						TargetPort: intstr.FromInt(9090),
					},
				},
			},
		},
	}
	for _, svc := range svcs {
		_, err := cs.CoreV1().Services(svc.ObjectMeta.Namespace).Create(ctx, &svc, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	ti, err := c.checkForZarfConnectLabel(ctx, "good")
	require.NoError(t, err)
	require.Equal(t, k8s.SvcResource, ti.resourceType)
	require.Equal(t, "good-service", ti.resourceName)
	require.Equal(t, "good-namespace", ti.namespace)
	require.Equal(t, 9090, ti.remotePort)
	require.Equal(t, "foobar", ti.urlSuffix)
}
