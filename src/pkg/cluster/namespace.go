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

	"github.com/zarf-dev/zarf/src/pkg/message"
)

// CreateZarfNamespace creates the Zarf namespace.
func (c *Cluster) CreateZarfNamespace(ctx context.Context) error {
	// Try to create the zarf namespace.
	zarfNamespace := NewZarfManagedNamespace(ZarfNamespaceName)
	err := func() error {
		_, err := c.Clientset.CoreV1().Namespaces().Create(ctx, zarfNamespace, metav1.CreateOptions{})
		if err != nil && !kerrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create the Zarf namespace: %w", err)
		}
		if err == nil {
			return nil
		}
		_, err = c.Clientset.CoreV1().Namespaces().Update(ctx, zarfNamespace, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("unable to update the Zarf namespace: %w", err)
		}
		return nil
	}()
	if err != nil {
		return err
	}

	// Wait up to 2 minutes for the default service account to be created.
	// Some clusters seem to take a while to create this, see https://github.com/kubernetes/kubernetes/issues/66689.
	// The default SA is required for pods to start properly.
	saCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	err = func(ctx context.Context, ns, name string) error {
		timer := time.NewTimer(0)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("failed to get service account %s/%s: %w", ns, name, ctx.Err())
			case <-timer.C:
				_, err := c.Clientset.CoreV1().ServiceAccounts(ns).Get(ctx, name, metav1.GetOptions{})
				if err != nil && !kerrors.IsNotFound(err) {
					return err
				}
				if kerrors.IsNotFound(err) {
					message.Debug("Service account %s/%s not found, retrying...", ns, name)
					timer.Reset(1 * time.Second)
					continue
				}
				return nil
			}
		}
	}(saCtx, ZarfNamespaceName, "default")
	if err != nil {
		return fmt.Errorf("unable get default Zarf service account: %w", err)
	}
	return nil
}

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
	labels[ZarfManagedByLabel] = "zarf"
	return labels
}
