// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReplaceService deletes and re-creates a service.
func (k *K8s) ReplaceService(service *corev1.Service) (*corev1.Service, error) {
	if err := k.DeleteService(service.Namespace, service.Name); err != nil {
		return nil, err
	}

	return k.CreateService(service)
}

// GenerateService returns a K8s service struct without writing to the cluster.
func (k *K8s) GenerateService(namespace, name string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: make(Labels),
			Labels:      k.Labels,
		},
	}
}

// DeleteService removes a service from the cluster by namespace and name.
func (k *K8s) DeleteService(namespace, name string) error {
	return k.Clientset.CoreV1().Services(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

// CreateService creates the given service in the cluster.
func (k *K8s) CreateService(service *corev1.Service) (*corev1.Service, error) {
	createOptions := metav1.CreateOptions{}
	return k.Clientset.CoreV1().Services(service.Namespace).Create(context.TODO(), service, createOptions)
}

// GetService returns a Kubernetes service resource in the provided namespace with the given name.
func (k *K8s) GetService(namespace, serviceName string) (*corev1.Service, error) {
	return k.Clientset.CoreV1().Services(namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
}

// GetServices returns a list of services in the provided namespace.  To search all namespaces, pass "" in the namespace arg.
func (k *K8s) GetServices(namespace string) (*corev1.ServiceList, error) {
	return k.Clientset.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{})
}

// GetServicesByLabel returns a list of matched services given a label and value.  To search all namespaces, pass "" in the namespace arg.
func (k *K8s) GetServicesByLabel(namespace, label, value string) (*corev1.ServiceList, error) {
	// Creat the selector and add the requirement
	labelSelector, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: Labels{
			label: value,
		},
	})

	// Run the query with the selector and return as a ServiceList
	return k.Clientset.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector.String()})
}

// GetServicesByLabelExists returns a list of matched services given a label.  To search all namespaces, pass "" in the namespace arg.
func (k *K8s) GetServicesByLabelExists(namespace, label string) (*corev1.ServiceList, error) {
	// Creat the selector and add the requirement
	labelSelector, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      label,
			Operator: metav1.LabelSelectorOpExists,
		}},
	})

	// Run the query with the selector and return as a ServiceList
	return k.Clientset.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector.String()})
}
