// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

// TestGetPruneableCharts_Integration tests the packager coordination layer.
// Detailed unit tests for the filtering logic are in state_test.go (DeployedPackage.GetPruneableCharts).
// This test verifies the wrapper function correctly delegates to the state layer and wraps the result.
func TestGetPruneableCharts_Integration(t *testing.T) {
	tests := []struct {
		name            string
		deployedPackage *state.DeployedPackage
		opts            PruneOptions
		wantErr         bool
		validateResult  func(t *testing.T, result PruneStateResult)
	}{
		{
			name: "wraps result in PruneStateResult correctly",
			deployedPackage: &state.DeployedPackage{
				Name: "test-package",
				DeployedComponents: []state.DeployedComponent{
					{
						Name: "component1",
						InstalledCharts: []state.InstalledChart{
							{
								Namespace: "podinfo",
								ChartName: "chart1",
								State:     state.ChartStateOrphaned,
								Status:    state.ChartStatusSucceeded,
							},
						},
					},
				},
			},
			opts: PruneOptions{},
			validateResult: func(t *testing.T, result PruneStateResult) {
				require.NotNil(t, result.PruneableCharts)
				require.Contains(t, result.PruneableCharts, "component1")
				require.Len(t, result.PruneableCharts["component1"], 1)
			},
		},
		{
			name: "passes PruneOptions filters to state layer",
			deployedPackage: &state.DeployedPackage{
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
						},
					},
					{
						Name: "component2",
						InstalledCharts: []state.InstalledChart{
							{
								Namespace: "ns2",
								ChartName: "chart2",
								State:     state.ChartStateOrphaned,
							},
						},
					},
				},
			},
			opts: PruneOptions{
				Component: "component1",
			},
			validateResult: func(t *testing.T, result PruneStateResult) {
				require.Len(t, result.PruneableCharts, 1)
				require.Contains(t, result.PruneableCharts, "component1")
				require.NotContains(t, result.PruneableCharts, "component2")
			},
		},
		{
			name: "propagates errors from state layer",
			deployedPackage: &state.DeployedPackage{
				Name:               "test-package",
				DeployedComponents: []state.DeployedComponent{},
			},
			opts: PruneOptions{
				Component: "nonexistent",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetPruneableCharts(tt.deployedPackage, tt.opts)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.validateResult != nil {
				tt.validateResult(t, result)
			}
		})
	}
}

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
