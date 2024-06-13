// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"time"

	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeleteZarfNamespace deletes the Zarf namespace from the connected cluster.
func (c *Cluster) DeleteZarfNamespace(ctx context.Context) error {
	spinner := message.NewProgressSpinner("Deleting the zarf namespace from this cluster")
	defer spinner.Stop()

	err := c.Clientset.CoreV1().Namespaces().Delete(ctx, ZarfNamespaceName, metav1.DeleteOptions{})
	if kerrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			_, err := c.Clientset.CoreV1().Namespaces().Get(ctx, ZarfNamespaceName, metav1.GetOptions{})
			if kerrors.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return err
			}
			timer.Reset(1 * time.Second)
		}
	}
}

// NewZarfManagedNamespace returns a corev1.Namespace with Zarf-managed labels
func NewZarfManagedNamespace(name string) *corev1.Namespace {
	namespace := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	namespace.Labels = AdoptZarfManagedLabels(namespace.Labels)
	return namespace
}

// AdoptZarfManagedLabels adds & deletes the necessary labels that signal to Zarf it should manage a namespace
func AdoptZarfManagedLabels(labels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[k8s.ZarfManagedByLabel] = "zarf"
	delete(labels, k8s.AgentLabel) // remove
	return labels
}
