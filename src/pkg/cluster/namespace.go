// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/zarf-dev/zarf/src/pkg/message"
	corev1 "k8s.io/api/core/v1"
	v1ac "k8s.io/client-go/applyconfigurations/core/v1"
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
	err = retry.Do(func() error {
		_, err := c.Clientset.CoreV1().Namespaces().Get(ctx, ZarfNamespaceName, metav1.GetOptions{})
		if kerrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("namespace still exists")
	}, retry.Context(ctx), retry.Attempts(0), retry.DelayType(retry.FixedDelay), retry.Delay(time.Second))
	if err != nil {
		return err
	}
	return nil
}

// NewZarfManagedApplyNamespace returns a v1ac.NamespaceApplyConfiguration with Zarf-managed labels
func NewZarfManagedApplyNamespace(name string) *v1ac.NamespaceApplyConfiguration {
	return v1ac.Namespace(name).WithLabels(AdoptZarfManagedLabels(nil))
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
