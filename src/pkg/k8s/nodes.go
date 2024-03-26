// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetNodes returns a list of nodes from the k8s cluster.
func (k *K8s) GetNodes(ctx context.Context) (*corev1.NodeList, error) {
	metaOptions := metav1.ListOptions{}
	return k.Clientset.CoreV1().Nodes().List(ctx, metaOptions)
}

// GetNode returns a node from the k8s cluster.
func (k *K8s) GetNode(ctx context.Context, nodeName string) (*corev1.Node, error) {
	return k.Clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
}
