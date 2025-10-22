// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains operations for working with helm charts.
package packager

import (
	"context"
	"fmt"
	"time"

	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

// PruneOptions are the options for Prune
type PruneOptions struct {
	Cluster   *cluster.Cluster
	Component string
	Chart     string
	Timeout   time.Duration
	Pending   bool
}

// PruneCharts removes the specified charts from the cluster and updates the deployed package state.
// This function handles the infrastructure concerns (helm operations and cluster state updates)
// while delegating the data manipulation to the DeployedPackage methods.
func PruneCharts(ctx context.Context, deployedPackage *state.DeployedPackage, prunableCharts map[string][]state.InstalledChart, opts PruneOptions) error {
	// Remove each chart from the cluster using helm
	for componentName, charts := range prunableCharts {
		for _, chart := range charts {
			err := helm.RemoveChart(ctx, chart.Namespace, chart.ChartName, opts.Timeout)
			if err != nil {
				return fmt.Errorf("failed to remove chart %s from component %s: %w", chart.ChartName, componentName, err)
			}
		}
	}

	// Update the deployed package state to remove the pruned charts
	deployedPackage.RemovePrunedCharts(prunableCharts)

	// Save the updated state back to the cluster
	err := opts.Cluster.UpdateDeployedPackage(ctx, *deployedPackage)
	if err != nil {
		return fmt.Errorf("unable to update deployed package state: %w", err)
	}

	return nil
}
