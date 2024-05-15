// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"context"

	autoscalingV2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetAllHPAs returns a list of horizontal pod autoscalers for all namespaces.
func (k *K8s) GetAllHPAs(ctx context.Context) (*autoscalingV2.HorizontalPodAutoscalerList, error) {
	return k.GetHPAs(ctx, corev1.NamespaceAll)
}

// GetHPAs returns a list of horizontal pod autoscalers in a given namespace.
func (k *K8s) GetHPAs(ctx context.Context, namespace string) (*autoscalingV2.HorizontalPodAutoscalerList, error) {
	metaOptions := metav1.ListOptions{}
	return k.Clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metaOptions)
}

// GetHPA returns a single horizontal pod autoscaler by namespace and name.
func (k *K8s) GetHPA(ctx context.Context, namespace, name string) (*autoscalingV2.HorizontalPodAutoscaler, error) {
	metaOptions := metav1.GetOptions{}
	return k.Clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(ctx, name, metaOptions)
}

// UpdateHPA updates the given horizontal pod autoscaler in the cluster.
func (k *K8s) UpdateHPA(ctx context.Context, hpa *autoscalingV2.HorizontalPodAutoscaler) (*autoscalingV2.HorizontalPodAutoscaler, error) {
	metaOptions := metav1.UpdateOptions{}
	return k.Clientset.AutoscalingV2().HorizontalPodAutoscalers(hpa.Namespace).Update(ctx, hpa, metaOptions)
}
