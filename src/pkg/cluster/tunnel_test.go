// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cluster

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/state"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func TestListConnections(t *testing.T) {
	t.Parallel()

	c := &Cluster{
		Clientset: fake.NewClientset(),
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
	expectedConnections := state.ConnectStrings{
		"connect name": state.ConnectString{
			Description: "description",
			URL:         "url",
		},
	}
	require.Equal(t, expectedConnections, connections)
}

func TestCheckForZarfConnectLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		svc         corev1.Service
		connectName string
		expectedErr string
		expected    TunnelInfo
	}{
		{
			// A service with no ports (e.g. ExternalName)
			name: "service with no ports",
			svc: corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "no-ports",
					Labels: map[string]string{
						ZarfConnectLabelName: "my-connect",
					},
				},
				Spec: corev1.ServiceSpec{},
			},
			connectName: "my-connect",
			expectedErr: "service default/no-ports has no ports",
		},
		{
			name: "service with ports",
			svc: corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "app-ns",
					Name:      "web",
					Labels: map[string]string{
						ZarfConnectLabelName: "web-ui",
					},
					Annotations: map[string]string{
						ZarfConnectAnnotationURL: "/dashboard",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:       8080,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			},
			connectName: "web-ui",
			expected: TunnelInfo{
				ResourceType: SvcResource,
				ResourceName: "web",
				Namespace:    "app-ns",
				RemotePort:   8080,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &Cluster{
				Clientset: fake.NewClientset(),
			}
			_, err := c.Clientset.CoreV1().Services(tt.svc.Namespace).Create(context.Background(), &tt.svc, metav1.CreateOptions{})
			require.NoError(t, err)

			ti, err := c.checkForZarfConnectLabel(context.Background(), tt.connectName)
			if tt.expectedErr != "" {
				require.EqualError(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected.ResourceType, ti.ResourceType)
			require.Equal(t, tt.expected.ResourceName, ti.ResourceName)
			require.Equal(t, tt.expected.Namespace, ti.Namespace)
			require.Equal(t, tt.expected.RemotePort, ti.RemotePort)
		})
	}
}

func TestFindPodContainerPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		svc          corev1.Service
		pods         []corev1.Pod
		expectedErr  string
		expectedPort int
	}{
		{
			name: "service with no ports",
			svc: corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "no-ports",
				},
				Spec: corev1.ServiceSpec{},
			},
			expectedErr: "service default/no-ports has no ports",
		},
		{
			name: "matching named port on pod",
			svc: corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "app-ns",
					Name:      "web",
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{"app": "web"},
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromString("http"),
						},
					},
				},
			},
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "app-ns",
						Name:      "web-ui",
						Labels:    map[string]string{"app": "web"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "server",
								Ports: []corev1.ContainerPort{
									{
										Name:          "http",
										ContainerPort: 8080,
									},
								},
							},
						},
					},
				},
			},
			expectedPort: 8080,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &Cluster{
				Clientset: fake.NewClientset(),
			}
			for i := range tt.pods {
				_, err := c.Clientset.CoreV1().Pods(tt.pods[i].Namespace).Create(context.Background(), &tt.pods[i], metav1.CreateOptions{})
				require.NoError(t, err)
			}

			port, err := c.findPodContainerPort(context.Background(), tt.svc)
			if tt.expectedErr != "" {
				require.EqualError(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedPort, port)
		})
	}
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
