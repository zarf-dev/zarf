// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
	corev1 "k8s.io/api/core/v1"
	v1ac "k8s.io/client-go/applyconfigurations/core/v1"
)

// DeleteZarfNamespace deletes the Zarf namespace from the connected cluster.
func (c *Cluster) DeleteZarfNamespace(ctx context.Context) error {
	start := time.Now()
	l := logger.From(ctx)
	l.Info("deleting the zarf namespace from this cluster")

	err := c.Clientset.CoreV1().Namespaces().Delete(ctx, state.ZarfNamespaceName, metav1.DeleteOptions{})
	if kerrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	err = wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		_, err := c.Clientset.CoreV1().Namespaces().Get(ctx, state.ZarfNamespaceName, metav1.GetOptions{})
		if kerrors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	l.Debug("done deleting the zarf namespace from this cluster", "duration", time.Since(start))
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
	labels[state.ZarfManagedByLabel] = "zarf"
	return labels
}
