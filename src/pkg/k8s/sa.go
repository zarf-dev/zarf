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
func (k *K8s) GetAllServiceAccounts() (*corev1.ServiceAccountList, error) {
	return k.GetServiceAccounts(corev1.NamespaceAll)
}

// GetServiceAccounts returns a list of service accounts in a given namespace.
func (k *K8s) GetServiceAccounts(namespace string) (*corev1.ServiceAccountList, error) {
	metaOptions := metav1.ListOptions{}
	return k.Clientset.CoreV1().ServiceAccounts(namespace).List(context.TODO(), metaOptions)
}

// GetServiceAccount returns a single service account by namespace and name.
func (k *K8s) GetServiceAccount(namespace, name string) (*corev1.ServiceAccount, error) {
	metaOptions := metav1.GetOptions{}
	return k.Clientset.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), name, metaOptions)
}

// SaveServiceAccount updates the given service account in the cluster.
func (k *K8s) SaveServiceAccount(svcAccount *corev1.ServiceAccount) (*corev1.ServiceAccount, error) {
	metaOptions := metav1.UpdateOptions{}
	return k.Clientset.CoreV1().ServiceAccounts(svcAccount.Namespace).Update(context.TODO(), svcAccount, metaOptions)
}

// WaitForServiceAccount waits for a service account to be created in the cluster.
func (k *K8s) WaitForServiceAccount(ns, name string, timeout time.Duration) (*corev1.ServiceAccount, error) {
	expired := time.After(timeout)

	for {
		select {
		case <-expired:
			return nil, fmt.Errorf("timed out waiting for service account %s/%s to exist", ns, name)

		default:
			sa, err := k.Clientset.CoreV1().ServiceAccounts(ns).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					time.Sleep(1 * time.Second)
					continue
				}
				return nil, fmt.Errorf("error getting service account %s/%s: %w", ns, name, err)
			}

			return sa, nil
		}
	}
}
