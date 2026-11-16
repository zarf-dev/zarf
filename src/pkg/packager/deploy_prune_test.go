// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"
	"testing"
	"time"

	"helm.sh/helm/v4/pkg/storage/driver"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

func TestReconcileComponentCharts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var removedCharts []string
	d := &deployer{
		removeChart: func(ctx context.Context, namespace, name string, timeout time.Duration) error {
			removedCharts = append(removedCharts, name)
			return nil
		},
	}

	depComp := &state.DeployedComponent{
		Name: "test-component",
		InstalledCharts: []state.InstalledChart{
			{Namespace: "default", ChartName: "chart-to-keep"},
			{Namespace: "default", ChartName: "chart-to-remove"},
		},
	}

	newCharts := []state.InstalledChart{
		{Namespace: "default", ChartName: "chart-to-keep"},
	}

	err := d.reconcileComponentCharts(ctx, depComp, newCharts, time.Second)
	require.NoError(t, err)
	require.Len(t, depComp.InstalledCharts, 1)
	require.Equal(t, "chart-to-keep", depComp.InstalledCharts[0].ChartName)
	require.Equal(t, []string{"chart-to-remove"}, removedCharts)
}

func TestReconcileComponentChartsReleaseNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	d := &deployer{
		removeChart: func(ctx context.Context, namespace, name string, timeout time.Duration) error {
			return driver.ErrReleaseNotFound
		},
	}

	depComp := &state.DeployedComponent{
		Name: "test-component",
		InstalledCharts: []state.InstalledChart{
			{Namespace: "default", ChartName: "missing-chart"},
		},
	}

	err := d.reconcileComponentCharts(ctx, depComp, nil, time.Second)
	require.NoError(t, err)
	require.Empty(t, depComp.InstalledCharts)
}

func TestReconcileComponentChartsError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	d := &deployer{
		removeChart: func(ctx context.Context, namespace, name string, timeout time.Duration) error {
			return errors.New("cluster error")
		},
	}

	depComp := &state.DeployedComponent{
		Name: "test-component",
		InstalledCharts: []state.InstalledChart{
			{Namespace: "default", ChartName: "failing-chart"},
		},
	}

	err := d.reconcileComponentCharts(ctx, depComp, nil, time.Second)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unable to remove orphaned chart failing-chart")
}

func TestReconcileDroppedComponentsNoCluster(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := &deployer{}

	// When not connected to cluster, reconcileDroppedComponents returns nil without error
	err := d.reconcileDroppedComponents(ctx, nil, DeployOptions{})
	require.NoError(t, err)
}

func TestReconcileDroppedComponentsKeepsActive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	depPkg := &state.DeployedPackage{
		Name: "test-pkg",
		DeployedComponents: []state.DeployedComponent{
			{Name: "comp-active"},
		},
	}

	d := &deployer{
		deployedPkg: depPkg,
	}

	// pkg contains comp-active, so it should not be removed
	err := d.reconcileDroppedComponents(ctx, nil, DeployOptions{})
	require.NoError(t, err)
	require.Len(t, depPkg.DeployedComponents, 1)
	require.Equal(t, "comp-active", depPkg.DeployedComponents[0].Name)
}

func TestRemoveComponentFallback(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	depPkg := &state.DeployedPackage{
		Name: "test-pkg",
		DeployedComponents: []state.DeployedComponent{
			{Name: "c1", InstalledCharts: []state.InstalledChart{
				{Namespace: "ns", ChartName: "orphaned-chart"},
			}},
		},
	}

	err := removeComponent(ctx, removeComponentOptions{
		depPkg:       depPkg,
		compIdx:      0,
		comp:         api.Component{Name: "c1"},
		timeout:      time.Second,
		persistState: false,
	})
	require.NoError(t, err)
	require.Empty(t, depPkg.DeployedComponents)
}

func TestPruneOrphanedImagesNoCluster(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := &deployer{}

	err := d.pruneOrphanedImages(ctx, nil)
	require.NoError(t, err)
}
