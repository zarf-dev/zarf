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
func (k *K8s) GetAllHPAs() (*autoscalingV2.HorizontalPodAutoscalerList, error) {
	return k.GetHPAs(corev1.NamespaceAll)
}

// GetHPAs returns a list of horizontal pod autoscalers in a given namespace.
func (k *K8s) GetHPAs(namespace string) (*autoscalingV2.HorizontalPodAutoscalerList, error) {
	metaOptions := metav1.ListOptions{}
	return k.Clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(context.TODO(), metaOptions)
}

// GetHPA returns a single horizontal pod autoscaler by namespace and name.
func (k *K8s) GetHPA(namespace, name string) (*autoscalingV2.HorizontalPodAutoscaler, error) {
	metaOptions := metav1.GetOptions{}
	return k.Clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(context.TODO(), name, metaOptions)
}

// UpdateHPA updates the given horizontal pod autoscaler in the cluster.
func (k *K8s) UpdateHPA(hpa *autoscalingV2.HorizontalPodAutoscaler) (*autoscalingV2.HorizontalPodAutoscaler, error) {
	metaOptions := metav1.UpdateOptions{}
	return k.Clientset.AutoscalingV2().HorizontalPodAutoscalers(hpa.Namespace).Update(context.TODO(), hpa, metaOptions)
}
