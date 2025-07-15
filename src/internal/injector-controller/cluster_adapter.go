// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package injectorcontroller

import (
	"context"

	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"sigs.k8s.io/cli-utils/pkg/object"
)

// clusterAdapter adapts the cluster.Cluster to implement ClusterInterface
type clusterAdapter struct {
	cluster *cluster.Cluster
}

// NewClusterAdapter creates a new ClusterInterface adapter for cluster.Cluster
func NewClusterAdapter(cluster *cluster.Cluster) InjectionExecutor {
	return &clusterAdapter{
		cluster: cluster,
	}
}

// RunInjection executes the injection process
func (a *clusterAdapter) RunInjection(ctx context.Context, useRegistryProxy bool, payloadCMNames []string, shasum string, ipFamily state.IPFamily) error {
	return a.cluster.RunInjection(ctx, useRegistryProxy, payloadCMNames, shasum, ipFamily)
}

// WaitForReady waits for the specified objects to be ready
func (a *clusterAdapter) WaitForReady(ctx context.Context, objs []object.ObjMetadata) error {
	return healthchecks.WaitForReady(ctx, a.cluster.Watcher, objs)
}

// StopInjection stops the injection process
func (a *clusterAdapter) StopInjection(ctx context.Context, useRegistryProxy bool) error {
	return a.cluster.StopInjection(ctx, useRegistryProxy)
}
