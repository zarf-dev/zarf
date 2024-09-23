// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/zarf-dev/zarf/src/pkg/message"
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
	// returning false, or an error continues polling, true stops it
	err = wait.PollUntilContextCancel(ctx, time.Second, false, func(context.Context) (bool, error) {
		_, ierr := c.Clientset.CoreV1().Namespaces().Get(ctx, ZarfNamespaceName, metav1.GetOptions{})
		if kerrors.IsNotFound(ierr) {
			return true, nil
		}
		if ierr != nil {
			return false, ierr
		}
		return false, fmt.Errorf("namespace still exists")
	})
	if err != nil {
		return err
	}
	return nil
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
	labels[ZarfManagedByLabel] = "zarf"
	return labels
}
