// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cluster

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/state"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestResolveRegistryMode(t *testing.T) {
	t.Parallel()

	registryService := func(serviceType corev1.ServiceType, nodePort int32) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: ZarfRegistryName, Namespace: state.ZarfNamespaceName},
			Spec: corev1.ServiceSpec{
				Type:  serviceType,
				Ports: []corev1.ServicePort{{Port: ZarfRegistryPort, NodePort: nodePort}},
			},
		}
	}
	registryProxy := func(hostNetwork bool, hostPort int32) *appsv1.DaemonSet {
		return &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Name: zarfRegistryProxyName, Namespace: state.ZarfNamespaceName},
			Spec: appsv1.DaemonSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
				HostNetwork: hostNetwork,
				Containers: []corev1.Container{{Ports: []corev1.ContainerPort{{
					ContainerPort: hostPort,
					HostPort:      hostPort,
					HostIP:        "127.0.0.1",
				}}}},
			}}},
		}
	}

	tests := []struct {
		name         string
		address      string
		objects      []runtime.Object
		expectedMode state.RegistryMode
		expectedPort int
	}{
		{
			name:         "missing Zarf registry is external",
			address:      "localhost:31999",
			expectedMode: state.RegistryModeExternal,
		},
		{
			name:         "matching Zarf NodePort is internal",
			address:      "http://localhost:28000",
			objects:      []runtime.Object{registryService(corev1.ServiceTypeNodePort, 28000)},
			expectedMode: state.RegistryModeNodePort,
			expectedPort: 28000,
		},
		{
			name:         "different localhost port is external",
			address:      "localhost:31777",
			objects:      []runtime.Object{registryService(corev1.ServiceTypeNodePort, 31999)},
			expectedMode: state.RegistryModeExternal,
		},
		{
			name:         "matching proxy host port is internal",
			address:      "127.0.0.1:5000",
			objects:      []runtime.Object{registryService(corev1.ServiceTypeClusterIP, 0), registryProxy(false, 5000)},
			expectedMode: state.RegistryModeProxy,
			expectedPort: 5000,
		},
		{
			name:         "matching host-network proxy port is internal",
			address:      "[::1]:5000",
			objects:      []runtime.Object{registryService(corev1.ServiceTypeClusterIP, 0), registryProxy(true, 5000)},
			expectedMode: state.RegistryModeProxy,
			expectedPort: 5000,
		},
		{
			name:         "proxy service without proxy daemonset is external",
			address:      "localhost:5000",
			objects:      []runtime.Object{registryService(corev1.ServiceTypeClusterIP, 0)},
			expectedMode: state.RegistryModeExternal,
		},
		{
			name:         "non-loopback address is external",
			address:      "registry.example.com:31999",
			objects:      []runtime.Object{registryService(corev1.ServiceTypeNodePort, 31999)},
			expectedMode: state.RegistryModeExternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &Cluster{Clientset: fake.NewClientset(tt.objects...)}
			mode, port, err := c.ResolveRegistryMode(context.Background(), tt.address)
			require.NoError(t, err)
			require.Equal(t, tt.expectedMode, mode)
			require.Equal(t, tt.expectedPort, port)
		})
	}
}
