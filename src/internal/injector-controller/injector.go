// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package injectorcontroller

import (
	"context"

	"github.com/zarf-dev/zarf/src/pkg/state"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/object"
)

// InjectionExecutor defines the interface for executing injection operations
type InjectionExecutor interface {
	// RunInjection executes the injection process
	RunInjection(ctx context.Context, useRegistryProxy bool, payloadCMNames []string, shasum string, ipFamily state.IPFamily) error
	// WaitForReady waits for the specified objects to be ready
	WaitForReady(ctx context.Context, objs []object.ObjMetadata) error
	// StopInjection stops the injection process
	StopInjection(ctx context.Context, useRegistryProxy bool) error
}

// clusterInjectionExecutor implements InjectionExecutor using cluster operations
type clusterInjectionExecutor struct {
	cluster InjectionExecutor
}

// NewClusterInjectionExecutor creates a new InjectionExecutor using cluster operations
func NewClusterInjectionExecutor(cluster InjectionExecutor) InjectionExecutor {
	return &clusterInjectionExecutor{
		cluster: cluster,
	}
}

// RunInjection executes the injection process
func (e *clusterInjectionExecutor) RunInjection(ctx context.Context, useRegistryProxy bool, payloadCMNames []string, shasum string, ipFamily state.IPFamily) error {
	return e.cluster.RunInjection(ctx, useRegistryProxy, payloadCMNames, shasum, ipFamily)
}

// WaitForReady waits for the specified objects to be ready
func (e *clusterInjectionExecutor) WaitForReady(ctx context.Context, objs []object.ObjMetadata) error {
	return e.cluster.WaitForReady(ctx, objs)
}

// StopInjection stops the injection process
func (e *clusterInjectionExecutor) StopInjection(ctx context.Context, useRegistryProxy bool) error {
	return e.cluster.StopInjection(ctx, useRegistryProxy)
}

// getHealthCheckObjects returns the objects to wait for during health checks
func getHealthCheckObjects() []object.ObjMetadata {
	return []object.ObjMetadata{
		{
			GroupKind: schema.GroupKind{
				Group: "apps",
				Kind:  "DaemonSet",
			},
			Namespace: "zarf",
			Name:      "zarf-registry-proxy",
		},
	}
}
