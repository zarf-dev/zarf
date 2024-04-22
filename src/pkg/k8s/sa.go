// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetAllServiceAccounts returns a list of services accounts for all namespaces.
func (k *K8s) GetAllServiceAccounts(ctx context.Context) (*corev1.ServiceAccountList, error) {
	return k.GetServiceAccounts(ctx, corev1.NamespaceAll)
}

// GetServiceAccounts returns a list of service accounts in a given namespace.
func (k *K8s) GetServiceAccounts(ctx context.Context, namespace string) (*corev1.ServiceAccountList, error) {
	metaOptions := metav1.ListOptions{}
	return k.Clientset.CoreV1().ServiceAccounts(namespace).List(ctx, metaOptions)
}

// GetServiceAccount returns a single service account by namespace and name.
func (k *K8s) GetServiceAccount(ctx context.Context, namespace, name string) (*corev1.ServiceAccount, error) {
	metaOptions := metav1.GetOptions{}
	return k.Clientset.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metaOptions)
}

// UpdateServiceAccount updates the given service account in the cluster.
func (k *K8s) UpdateServiceAccount(ctx context.Context, svcAccount *corev1.ServiceAccount) (*corev1.ServiceAccount, error) {
	metaOptions := metav1.UpdateOptions{}
	return k.Clientset.CoreV1().ServiceAccounts(svcAccount.Namespace).Update(ctx, svcAccount, metaOptions)
}

// WaitForServiceAccount waits for a service account to be created in the cluster.
func (k *K8s) WaitForServiceAccount(ctx context.Context, ns, name string) (*corev1.ServiceAccount, error) {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for service account %s/%s to exist: %w", ns, name, ctx.Err())
		case <-timer.C:
			sa, err := k.Clientset.CoreV1().ServiceAccounts(ns).Get(ctx, name, metav1.GetOptions{})
			if err == nil {
				return sa, nil
			}

			if errors.IsNotFound(err) {
				k.Log("Service account %s/%s not found, retrying...", ns, name)
			} else {
				return nil, fmt.Errorf("error getting service account %s/%s: %w", ns, name, err)
			}

			timer.Reset(1 * time.Second)
		}
	}
}
