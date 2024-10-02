// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package healthchecks run kstatus style health checks on a list of objects
package healthchecks

import (
	"context"

	pkgkubernetes "github.com/defenseunicorns/pkg/kubernetes"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/cli-utils/pkg/object"
)

// Run waits for a list of objects to be reconciled
func Run(ctx context.Context, watcher watcher.StatusWatcher, healthChecks []v1alpha1.NamespacedObjectKindReference) error {
	objs := []object.ObjMetadata{}
	for _, hc := range healthChecks {
		gv, err := schema.ParseGroupVersion(hc.APIVersion)
		if err != nil {
			return err
		}
		obj := object.ObjMetadata{
			GroupKind: schema.GroupKind{
				Group: gv.Group,
				Kind:  hc.Kind,
			},
			Namespace: hc.Namespace,
			Name:      hc.Name,
		}
		objs = append(objs, obj)
	}
	err := pkgkubernetes.WaitForReady(ctx, watcher, objs)
	if err != nil {
		return err
	}
	return nil
}
