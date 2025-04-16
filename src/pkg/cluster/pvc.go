// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cluster

import (
	"context"
	"github.com/zarf-dev/zarf/src/pkg/state"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UpdateGiteaPVC updates the existing Gitea persistent volume claim and tells Gitea whether to create or not.
// TODO(mkcp): We return both string true/false and errors here so our callers get a string. This should be returning an
// empty val if we error, but we'll have to refactor upstream beforehand.
func (c *Cluster) UpdateGiteaPVC(ctx context.Context, pvcName string, shouldRollBack bool) (string, error) {
	if shouldRollBack {
		pvc, err := c.Clientset.CoreV1().PersistentVolumeClaims(state.ZarfNamespaceName).Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			return "false", err
		}
		delete(pvc.Labels, "app.kubernetes.io/managed-by")
		delete(pvc.Annotations, "meta.helm.sh/release-name")
		delete(pvc.Annotations, "meta.helm.sh/release-namespace")
		_, err = c.Clientset.CoreV1().PersistentVolumeClaims(state.ZarfNamespaceName).Update(ctx, pvc, metav1.UpdateOptions{})
		if err != nil {
			return "false", err
		}
		return "false", nil
	}

	if pvcName == "data-zarf-gitea-0" {
		pvc, err := c.Clientset.CoreV1().PersistentVolumeClaims(state.ZarfNamespaceName).Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			return "true", err
		}
		pvc.Labels["app.kubernetes.io/managed-by"] = "Helm"
		pvc.Annotations["meta.helm.sh/release-name"] = "zarf-gitea"
		pvc.Annotations["meta.helm.sh/release-namespace"] = "zarf"
		_, err = c.Clientset.CoreV1().PersistentVolumeClaims(state.ZarfNamespaceName).Update(ctx, pvc, metav1.UpdateOptions{})
		if err != nil {
			return "true", err
		}
		return "true", nil
	}

	return "false", nil
}
