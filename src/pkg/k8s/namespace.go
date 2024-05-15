// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"context"
	"time"

	"cuelang.org/go/pkg/strings"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetNamespaces returns a list of namespaces in the cluster.
func (k *K8s) GetNamespaces(ctx context.Context) (*corev1.NamespaceList, error) {
	metaOptions := metav1.ListOptions{}
	return k.Clientset.CoreV1().Namespaces().List(ctx, metaOptions)
}

// GetNamespace gets a namespace by name
func (k *K8s) GetNamespace(name string) (*corev1.Namespace, error) {
	getOptions := metav1.GetOptions{}
	return k.Clientset.CoreV1().Namespaces().Get(context.TODO(), name, getOptions)
}

// UpdateNamespace updates the given namespace in the cluster.
func (k *K8s) UpdateNamespace(ctx context.Context, namespace *corev1.Namespace) (*corev1.Namespace, error) {
	updateOptions := metav1.UpdateOptions{}
	return k.Clientset.CoreV1().Namespaces().Update(ctx, namespace, updateOptions)
}

// CreateNamespace creates the given namespace or returns it if it already exists in the cluster.
func (k *K8s) CreateNamespace(ctx context.Context, namespace *corev1.Namespace) (*corev1.Namespace, error) {
	metaOptions := metav1.GetOptions{}
	createOptions := metav1.CreateOptions{}

	match, err := k.Clientset.CoreV1().Namespaces().Get(ctx, namespace.Name, metaOptions)

	if err != nil || match.Name != namespace.Name {
		return k.Clientset.CoreV1().Namespaces().Create(ctx, namespace, createOptions)
	}

	return match, err
}

// DeleteNamespace deletes the given namespace from the cluster.
func (k *K8s) DeleteNamespace(ctx context.Context, name string) error {
	// Attempt to delete the namespace immediately
	gracePeriod := int64(0)
	err := k.Clientset.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			_, err := k.Clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return nil
			}

			timer.Reset(1 * time.Second)
		}
	}
}

// IsInitialNamespace returns true if the given namespace name is an initial k8s namespace: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/#initial-namespaces
func (k *K8s) IsInitialNamespace(name string) bool {
	if name == "default" {
		return true
	} else if strings.HasPrefix(name, "kube-") {
		return true
	}

	return false
}
