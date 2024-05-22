// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDetectDistro(t *testing.T) {
	t.Parallel()

	tests := []struct {
		distro     string
		node       corev1.Node
		namespaces []corev1.Namespace
	}{
		{
			distro: DistroIsUnknown,
			node: corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo": "bar",
					},
				},
				Spec: corev1.NodeSpec{
					ProviderID: "hello world",
				},
			},
			namespaces: []corev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
					},
				},
			},
		},
		{
			distro: DistroIsK3s,
			node: corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node.kubernetes.io/instance-type": "k3s",
					},
				},
			},
		},
		{
			distro: DistroIsK3d,
			node: corev1.Node{
				Spec: corev1.NodeSpec{
					ProviderID: "k3s://k3d-k3s-default-server-0",
				},
			},
		},
		{
			distro: DistroIsKind,
			node: corev1.Node{
				Spec: corev1.NodeSpec{
					ProviderID: "kind://docker/kind/kind-control-plane",
				},
			},
		},
		{
			distro: DistroIsMicroK8s,
			node: corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"microk8s.io/cluster": "true",
					},
				},
			},
		},
		{
			distro: DistroIsEKS,
			node: corev1.Node{
				Spec: corev1.NodeSpec{
					ProviderID: "aws:////i-112bac41a19da1819",
				},
			},
		},
		{
			distro: DistroIsEKSAnywhere,
			namespaces: []corev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "eksa-system",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "baz",
					},
				},
			},
		},
		{
			distro: DistroIsDockerDesktop,
			node: corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "docker-desktop",
				},
			},
		},
		{
			distro: DistroIsGKE,
			node: corev1.Node{
				Spec: corev1.NodeSpec{
					ProviderID: "gce://kthw-239419/us-central1-f/gk3-autopilot-cluster-1-pool-2-e87e560a-7gvw",
				},
			},
		},
		{
			distro: DistroIsAKS,
			node: corev1.Node{
				Spec: corev1.NodeSpec{
					ProviderID: "azure:///subscriptions/9107f2fb-e486-a434-a948-52e2929b6f18/resourceGroups/MC_rg_capz-managed-aks_eastus/providers/Microsoft.Compute/virtualMachineScaleSets/aks-agentpool0-10226072-vmss/virtualMachines/0",
				},
			},
		},
		{
			distro: DistroIsRKE2,
			node: corev1.Node{
				Status: corev1.NodeStatus{
					Images: []corev1.ContainerImage{
						{
							Names: []string{"docker.io/library/ubuntu:latest"},
						},
						{
							Names: []string{"rancher/rancher-agent:v2"},
						},
					},
				},
			},
		},
		{
			distro: DistroIsTKG,
			node: corev1.Node{
				Status: corev1.NodeStatus{
					Images: []corev1.ContainerImage{
						{
							Names: []string{"docker.io/library/ubuntu:latest"},
						},
						{
							Names: []string{"projects.registry.vmware.com/tkg/tanzu_core/"},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.distro, func(t *testing.T) {
			t.Parallel()

			distro := detectDistro(tt.node, tt.namespaces)
			require.Equal(t, tt.distro, distro)
		})
	}
}
