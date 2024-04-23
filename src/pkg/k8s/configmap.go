// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReplaceConfigmap deletes and recreates a configmap.
func (k *K8s) ReplaceConfigmap(namespace, name string, data map[string][]byte) (*corev1.ConfigMap, error) {
	if err := k.DeleteConfigmap(namespace, name); err != nil {
		return nil, err
	}

	return k.CreateConfigmap(namespace, name, data)
}

// CreateConfigmap applies a configmap to the cluster.
func (k *K8s) CreateConfigmap(namespace, name string, data map[string][]byte) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    make(Labels),
		},
		BinaryData: data,
	}

	createOptions := metav1.CreateOptions{}
	return k.Clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), configMap, createOptions)
}

// DeleteConfigmap deletes a configmap by name.
func (k *K8s) DeleteConfigmap(namespace, name string) error {
	namespaceConfigmap := k.Clientset.CoreV1().ConfigMaps(namespace)

	err := namespaceConfigmap.Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error deleting the configmap: %w", err)
	}

	return nil
}

// DeleteConfigMapsByLabel deletes a configmap by label(s).
func (k *K8s) DeleteConfigMapsByLabel(namespace string, labels Labels) error {
	labelSelector, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: labels,
	})
	metaOptions := metav1.DeleteOptions{}
	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	}

	return k.Clientset.CoreV1().ConfigMaps(namespace).DeleteCollection(context.TODO(), metaOptions, listOptions)
}
