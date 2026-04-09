// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cluster

import (
	"context"
	"fmt"

	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UpdateGiteaPVC updates the existing Gitea persistent volume claim and tells Gitea whether to create or not.
func (c *Cluster) UpdateGiteaPVC(ctx context.Context, pvcName string, shouldRollBack bool) (bool, error) {
	var pvcExists bool
	// check if the object exists
	pvc, err := c.Clientset.CoreV1().
		PersistentVolumeClaims(state.ZarfNamespaceName).
		Get(ctx,
			pvcName,
			metav1.GetOptions{})
	if err != nil {
		logger.From(ctx).Debug(err.Error())
		if !errors.IsNotFound(err) {
			return false, err
		}
	} else {
		pvcExists = true
	}

	// If a rollback is requested on a nonexistent resource, return an error.
	if !pvcExists && shouldRollBack {
		return false, fmt.Errorf("cannot rollback Gitea PVC %q: resource does not exist", pvcName)
	}

	// If it exists and shouldRollBack, delete the labels from the object and update it.
	if pvcExists && shouldRollBack {
		delete(pvc.Labels, "app.kubernetes.io/managed-by")
		delete(pvc.Annotations, "meta.helm.sh/release-name")
		delete(pvc.Annotations, "meta.helm.sh/release-namespace")
		_, err := c.Clientset.CoreV1().
			PersistentVolumeClaims(state.ZarfNamespaceName).
			Update(ctx,
				pvc,
				metav1.UpdateOptions{})
		return false, err
	}

	// If the pvc exists, update the fields
	if pvcExists {
		pvc.Labels["app.kubernetes.io/managed-by"] = "Helm"
		pvc.Annotations["meta.helm.sh/release-name"] = "zarf-gitea"
		pvc.Annotations["meta.helm.sh/release-namespace"] = "zarf"
		_, err := c.Clientset.CoreV1().
			PersistentVolumeClaims(state.ZarfNamespaceName).
			Update(ctx, pvc, metav1.UpdateOptions{})
		return false, err
	}

	// pvc does not exist we should create it
	return true, nil
}
