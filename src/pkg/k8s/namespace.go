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
func (k *K8s) GetNamespaces() (*corev1.NamespaceList, error) {
	metaOptions := metav1.ListOptions{}
	return k.Clientset.CoreV1().Namespaces().List(context.TODO(), metaOptions)
}

// UpdateNamespace updates the given namespace in the cluster.
func (k *K8s) UpdateNamespace(namespace *corev1.Namespace) (*corev1.Namespace, error) {
	updateOptions := metav1.UpdateOptions{}
	return k.Clientset.CoreV1().Namespaces().Update(context.TODO(), namespace, updateOptions)
}

// CreateNamespace creates the given namespace or returns it if it already exists in the cluster.
func (k *K8s) CreateNamespace(namespace *corev1.Namespace) (*corev1.Namespace, error) {
	metaOptions := metav1.GetOptions{}
	createOptions := metav1.CreateOptions{}

	match, err := k.Clientset.CoreV1().Namespaces().Get(context.TODO(), namespace.Name, metaOptions)

	if err != nil || match.Name != namespace.Name {
		return k.Clientset.CoreV1().Namespaces().Create(context.TODO(), namespace, createOptions)
	}

	return match, err
}

// DeleteNamespace deletes the given namespace from the cluster.
func (k *K8s) DeleteNamespace(ctx context.Context, name string) error {
	// Attempt to delete the namespace immediately
	gracePeriod := int64(0)
	err := k.Clientset.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod})
	// If an error besides "not found" is returned, return it
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	// Indefinitely wait for the namespace to be deleted, use context.WithTimeout to limit this
	for {
		// Keep checking for the namespace to be deleted
		_, err := k.Clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
}

// NewZarfManagedNamespace returns a corev1.Namespace with Zarf-managed labels
func (k *K8s) NewZarfManagedNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				zarfManagedByLabel: "zarf",
			},
		},
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
