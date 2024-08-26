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

	"github.com/zarf-dev/zarf/src/types"
)

func TestListConnections(t *testing.T) {
	t.Parallel()

	c := &Cluster{
		Clientset: fake.NewSimpleClientset(),
	}
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "connect",
			Labels: map[string]string{
				ZarfConnectLabelName: "connect name",
			},
			Annotations: map[string]string{
				ZarfConnectAnnotationDescription: "description",
				ZarfConnectAnnotationURL:         "url",
			},
		},
		Spec: corev1.ServiceSpec{},
	}
	_, err := c.Clientset.CoreV1().Services(svc.ObjectMeta.Namespace).Create(context.Background(), &svc, metav1.CreateOptions{})
	require.NoError(t, err)

	connections, err := c.ListConnections(context.Background())
	require.NoError(t, err)
	expectedConnections := types.ConnectStrings{
		"connect name": types.ConnectString{
			Description: "description",
			URL:         "url",
		},
	}
	require.Equal(t, expectedConnections, connections)
}

func TestServiceInfoFromNodePortURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		services          []corev1.Service
		nodePortURL       string
		expectedErr       string
		expectedNamespace string
		expectedName      string
		expectedIP        string
		expectedPort      int
	}{
		{
			name:        "invalid node port",
			nodePortURL: "example.com",
			expectedErr: "node port services should be on localhost",
		},
		{
			name:        "invalid port range",
			nodePortURL: "http://localhost:8080",
			expectedErr: "node port services should use the port range 30000-32767",
		},
		{
			name:        "no services",
			nodePortURL: "http://localhost:30001",
			services:    []corev1.Service{},
			expectedErr: "no matching node port services found",
		},
		{
			name:        "found service",
			nodePortURL: "http://localhost:30001",
			services: []corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "wrong-type",
						Namespace: "wrong-type",
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeClusterIP,
						Ports: []corev1.ServicePort{
							{
								Port: 1111,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "wrong-node-port",
						Namespace: "wrong-node-port",
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeNodePort,
						Ports: []corev1.ServicePort{
							{
								NodePort: 30002,
								Port:     2222,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "good-service",
						Namespace: "good-namespace",
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeNodePort,
						Ports: []corev1.ServicePort{
							{
								NodePort: 30001,
								Port:     3333,
							},
						},
						ClusterIP: "good-ip",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "too-late",
						Namespace: "too-late",
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeNodePort,
						Ports: []corev1.ServicePort{
							{
								NodePort: 30001,
								Port:     4444,
							},
						},
					},
				},
			},
			expectedNamespace: "good-namespace",
			expectedName:      "good-service",
			expectedIP:        "good-ip",
			expectedPort:      3333,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, port, err := serviceInfoFromNodePortURL(tt.services, tt.nodePortURL)
			if tt.expectedErr != "" {
				require.EqualError(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedNamespace, svc.Namespace)
			require.Equal(t, tt.expectedName, svc.Name)
			require.Equal(t, tt.expectedPort, port)
			require.Equal(t, tt.expectedIP, svc.Spec.ClusterIP)
		})
	}
}
