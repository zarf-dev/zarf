// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

func TestPrunePackageCharts(t *testing.T) {
	t.Log("E2E: Prune orphaned helm charts")

	tmpdir := t.TempDir()
	testCreateV1 := filepath.Join("src", "test", "packages", "43-prune-test", "v1")
	testCreateV2 := filepath.Join("src", "test", "packages", "43-prune-test", "v2")
	packageV1Name := fmt.Sprintf("zarf-package-prune-test-%s-0.0.1.tar.zst", e2e.Arch)
	packageV2Name := fmt.Sprintf("zarf-package-prune-test-%s-0.0.2.tar.zst", e2e.Arch)
	packageV1Path := filepath.Join(tmpdir, packageV1Name)
	packageV2Path := filepath.Join(tmpdir, packageV2Name)

	// Create the v1 test package (with two charts)
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", testCreateV1, "-o", tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the v1 package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", packageV1Path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify both charts are deployed
	c, err := cluster.New(t.Context())
	require.NoError(t, err)
	deployedPackageV1, err := c.GetDeployedPackage(t.Context(), "prune-test")
	require.NoError(t, err)
	require.Len(t, deployedPackageV1.DeployedComponents, 1)
	require.Equal(t, "test-component", deployedPackageV1.DeployedComponents[0].Name)
	require.Len(t, deployedPackageV1.DeployedComponents[0].InstalledCharts, 2)

	// Verify both charts are Active
	for _, chart := range deployedPackageV1.DeployedComponents[0].InstalledCharts {
		require.Equal(t, state.ChartStateActive, chart.State)
	}

	// Create the v2 test package (with only one chart - chart-to-remove is gone)
	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", testCreateV2, "-o", tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the v2 package (this should mark chart-to-remove as orphaned)
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", packageV2Path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify that one chart is Active and one is Orphaned
	// Get fresh state from cluster
	deployedPackageV2, err := c.GetDeployedPackage(t.Context(), "prune-test")
	require.NoError(t, err)
	require.Len(t, deployedPackageV2.DeployedComponents, 1)
	require.Len(t, deployedPackageV2.DeployedComponents[0].InstalledCharts, 2)

	var activeCount, orphanedCount int
	var orphanedChartName string
	for _, chart := range deployedPackageV2.DeployedComponents[0].InstalledCharts {
		switch chart.State {
		case state.ChartStateActive:
			activeCount++
			require.Equal(t, "chart-to-keep", chart.ChartName)
		case state.ChartStateOrphaned:
			orphanedCount++
			orphanedChartName = chart.ChartName
			require.Equal(t, "chart-to-remove", chart.ChartName)
		}
	}
	require.Equal(t, 1, activeCount, "Expected 1 active chart")
	require.Equal(t, 1, orphanedCount, "Expected 1 orphaned chart")

	// Test prune without --confirm flag (should fail)
	stdOut, stdErr, err = e2e.Zarf(t, "package", "prune", "prune-test")
	require.Error(t, err, stdOut, stdErr)

	// Test prune with non-existent component (should fail)
	stdOut, stdErr, err = e2e.Zarf(t, "package", "prune", "prune-test", "--component=nonexistent", "--confirm")
	require.Error(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, `component "nonexistent" not found`)

	// Test prune with chart filter but no component filter (should fail due to validation)
	stdOut, stdErr, err = e2e.Zarf(t, "package", "prune", "prune-test", "--chart=chart-to-remove", "--confirm")
	require.Error(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "component must be specified when chart filter is provided")

	// Test prune with valid component and chart filters
	stdOut, stdErr, err = e2e.Zarf(t, "package", "prune", "prune-test", "--component=test-component", "--chart="+orphanedChartName, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the orphaned chart was pruned
	// Get fresh state after prune operation
	deployedPackageAfterPrune, err := c.GetDeployedPackage(t.Context(), "prune-test")
	require.NoError(t, err)
	require.Len(t, deployedPackageAfterPrune.DeployedComponents, 1)
	require.Len(t, deployedPackageAfterPrune.DeployedComponents[0].InstalledCharts, 1)
	require.Equal(t, "chart-to-keep", deployedPackageAfterPrune.DeployedComponents[0].InstalledCharts[0].ChartName)
	require.Equal(t, state.ChartStateActive, deployedPackageAfterPrune.DeployedComponents[0].InstalledCharts[0].State)

	// Clean up - remove the package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", packageV2Path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the package is no longer in state
	_, err = c.GetDeployedPackage(t.Context(), "prune-test")
	require.Error(t, err)
}
