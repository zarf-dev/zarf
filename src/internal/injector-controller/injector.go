// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package injectorcontroller

import (
	"context"

	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/state"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/object"
)

// InjectionExecutor defines the interface for executing injection operations
type InjectionExecutor interface {
	// RunInjection executes the injection process
	Run(ctx context.Context) error
}

// clusterInjectionExecutor implements InjectionExecutor using cluster operations
type clusterInjectionExecutor struct {
	cluster *cluster.Cluster
}

// NewClusterInjectionExecutor creates a new InjectionExecutor using cluster operations
func NewClusterInjectionExecutor(cluster *cluster.Cluster) InjectionExecutor {
	return &clusterInjectionExecutor{
		cluster: cluster,
	}
}

// RunInjection executes the injection process
func (e *clusterInjectionExecutor) Run(ctx context.Context) error {
	payloadCMNames := []string{}
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			"zarf-injector": "payload",
		},
	})
	if err != nil {
		return err
	}
	cmList, err := e.cluster.Clientset.CoreV1().ConfigMaps(state.ZarfNamespaceName).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	for _, cm := range cmList.Items {
		payloadCMNames = append(payloadCMNames, cm.Name)
	}
	if err != nil {
		return err
	}
	// FIXME: get shasum dynamically from cluster
	shasum := "4a3ba3eed0b5104c6aa07298a4ccb9159389226be56c4bb3c6821f2cdbe69245"
	// FIXME: Get ipFamily dynamically from state
	err = e.cluster.RunInjection(ctx, true, payloadCMNames, shasum, state.IPFamilyIPv4)
	if err != nil {
		return err
	}
	objs := []object.ObjMetadata{
		{
			GroupKind: schema.GroupKind{
				Group: "apps",
				Kind:  "DaemonSet",
			},
			Namespace: "zarf",
			Name:      "zarf-registry-proxy",
		},
	}
	err = healthchecks.WaitForReady(ctx, e.cluster.Watcher, objs)
	if err != nil {
		return err
	}
	err = e.cluster.StopInjection(ctx, true)
	if err != nil {
		return err
	}
	return nil
}
