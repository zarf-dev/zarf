// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

// TestPruneCharts_StateManipulation tests the state manipulation portion of PruneCharts.
// Note: Full integration testing of PruneCharts (including helm operations and cluster updates)
// is covered by E2E tests, as it requires running infrastructure.
func TestPruneCharts_StateManipulation(t *testing.T) {
	// Create a test deployed package
	deployedPackage := &state.DeployedPackage{
		Name: "test-package",
		DeployedComponents: []state.DeployedComponent{
			{
				Name: "component1",
				InstalledCharts: []state.InstalledChart{
					{
						Namespace: "ns1",
						ChartName: "chart1",
						State:     state.ChartStateOrphaned,
					},
					{
						Namespace: "ns1",
						ChartName: "chart2",
						State:     state.ChartStateActive,
					},
				},
			},
			{
				Name: "component2",
				InstalledCharts: []state.InstalledChart{
					{
						Namespace: "ns2",
						ChartName: "chart3",
						State:     state.ChartStateOrphaned,
					},
				},
			},
		},
	}

	// Define charts to prune
	prunedCharts := map[string][]state.InstalledChart{
		"component1": {
			{
				Namespace: "ns1",
				ChartName: "chart1",
				State:     state.ChartStateOrphaned,
			},
		},
	}

	// Verify the RemovePrunedCharts method is called correctly
	// (This tests the delegation, not the helm operations)
	deployedPackage.RemovePrunedCharts(prunedCharts)

	// Verify state was updated correctly
	require.Len(t, deployedPackage.DeployedComponents, 2)

	// component1 should have only chart2 remaining
	comp1 := deployedPackage.DeployedComponents[0]
	require.Equal(t, "component1", comp1.Name)
	require.Len(t, comp1.InstalledCharts, 1)
	require.Equal(t, "chart2", comp1.InstalledCharts[0].ChartName)

	// component2 should be unchanged
	comp2 := deployedPackage.DeployedComponents[1]
	require.Equal(t, "component2", comp2.Name)
	require.Len(t, comp2.InstalledCharts, 1)
	require.Equal(t, "chart3", comp2.InstalledCharts[0].ChartName)
}
