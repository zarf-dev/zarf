// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

func TestGetPruneableCharts(t *testing.T) {
	tests := []struct {
		name            string
		deployedPackage *state.DeployedPackage
		opts            PruneOptions
		want            map[string][]state.InstalledChart
		wantErr         string
	}{
		{
			name: "no filters - returns all orphaned charts",
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
							{
								Namespace: "podinfo",
								ChartName: "chart2",
								State:     state.ChartStateActive,
								Status:    state.ChartStatusSucceeded,
							},
						},
					},
					{
						Name: "component2",
						InstalledCharts: []state.InstalledChart{
							{
								Namespace: "monitoring",
								ChartName: "chart3",
								State:     state.ChartStateOrphaned,
								Status:    state.ChartStatusSucceeded,
							},
						},
					},
				},
			},
			opts: PruneOptions{},
			want: map[string][]state.InstalledChart{
				"component1": {
					{
						Namespace: "podinfo",
						ChartName: "chart1",
						State:     state.ChartStateOrphaned,
						Status:    state.ChartStatusSucceeded,
					},
				},
				"component2": {
					{
						Namespace: "monitoring",
						ChartName: "chart3",
						State:     state.ChartStateOrphaned,
						Status:    state.ChartStatusSucceeded,
					},
				},
			},
		},
		{
			name: "component filter - returns only charts from specified component",
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
					{
						Name: "component2",
						InstalledCharts: []state.InstalledChart{
							{
								Namespace: "monitoring",
								ChartName: "chart3",
								State:     state.ChartStateOrphaned,
								Status:    state.ChartStatusSucceeded,
							},
						},
					},
				},
			},
			opts: PruneOptions{
				Component: "component1",
			},
			want: map[string][]state.InstalledChart{
				"component1": {
					{
						Namespace: "podinfo",
						ChartName: "chart1",
						State:     state.ChartStateOrphaned,
						Status:    state.ChartStatusSucceeded,
					},
				},
			},
		},
		{
			name: "component and chart filter - returns specific chart from specific component",
			deployedPackage: &state.DeployedPackage{
				Name: "test-package",
				DeployedComponents: []state.DeployedComponent{
					{
						Name: "component1",
						InstalledCharts: []state.InstalledChart{
							{
								Namespace: "app-ns",
								ChartName: "chart1",
								State:     state.ChartStateOrphaned,
								Status:    state.ChartStatusSucceeded,
							},
							{
								Namespace: "app-ns",
								ChartName: "chart2",
								State:     state.ChartStateOrphaned,
								Status:    state.ChartStatusSucceeded,
							},
						},
					},
					{
						Name: "component2",
						InstalledCharts: []state.InstalledChart{
							{
								Namespace: "monitoring",
								ChartName: "chart1",
								State:     state.ChartStateOrphaned,
								Status:    state.ChartStatusSucceeded,
							},
						},
					},
				},
			},
			opts: PruneOptions{
				Component: "component2",
				Chart:     "chart1",
			},
			want: map[string][]state.InstalledChart{
				"component2": {
					{
						Namespace: "monitoring",
						ChartName: "chart1",
						State:     state.ChartStateOrphaned,
						Status:    state.ChartStatusSucceeded,
					},
				},
			},
		},
		{
			name: "no orphaned charts - returns empty map",
			deployedPackage: &state.DeployedPackage{
				Name: "test-package",
				DeployedComponents: []state.DeployedComponent{
					{
						Name: "component1",
						InstalledCharts: []state.InstalledChart{
							{
								Namespace: "podinfo",
								ChartName: "chart1",
								State:     state.ChartStateActive,
								Status:    state.ChartStatusSucceeded,
							},
						},
					},
				},
			},
			opts: PruneOptions{},
			want: map[string][]state.InstalledChart{},
		},
		{
			name: "component not found - returns error",
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
			opts: PruneOptions{
				Component: "nonexistent",
			},
			wantErr: `component "nonexistent" not found in deployed package`,
		},
		{
			name: "chart filter without component - returns error",
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
			opts: PruneOptions{
				Chart: "chart1",
			},
			wantErr: "component must be specified when chart filter is provided",
		},
		{
			name: "chart not found in specified component - returns error",
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
			opts: PruneOptions{
				Component: "component1",
				Chart:     "nonexistent",
			},
			wantErr: `chart "nonexistent" not found in deployed package`,
		},
		{
			name: "chart found but not orphaned - returns error",
			deployedPackage: &state.DeployedPackage{
				Name: "test-package",
				DeployedComponents: []state.DeployedComponent{
					{
						Name: "component1",
						InstalledCharts: []state.InstalledChart{
							{
								Namespace: "podinfo",
								ChartName: "chart1",
								State:     state.ChartStateActive,
								Status:    state.ChartStatusSucceeded,
							},
						},
					},
				},
			},
			opts: PruneOptions{
				Component: "component1",
				Chart:     "chart1",
			},
			wantErr: `chart "chart1" found in deployed package, but is not in the "Orphaned" state`,
		},
		{
			name: "multiple orphaned charts in same component",
			deployedPackage: &state.DeployedPackage{
				Name: "test-package",
				DeployedComponents: []state.DeployedComponent{
					{
						Name: "component1",
						InstalledCharts: []state.InstalledChart{
							{
								Namespace: "app-ns",
								ChartName: "chart1",
								State:     state.ChartStateOrphaned,
								Status:    state.ChartStatusSucceeded,
							},
							{
								Namespace: "app-ns",
								ChartName: "chart2",
								State:     state.ChartStateOrphaned,
								Status:    state.ChartStatusFailed,
							},
							{
								Namespace: "db-ns",
								ChartName: "chart3",
								State:     state.ChartStateOrphaned,
								Status:    state.ChartStatusSucceeded,
							},
						},
					},
				},
			},
			opts: PruneOptions{},
			want: map[string][]state.InstalledChart{
				"component1": {
					{
						Namespace: "app-ns",
						ChartName: "chart1",
						State:     state.ChartStateOrphaned,
						Status:    state.ChartStatusSucceeded,
					},
					{
						Namespace: "app-ns",
						ChartName: "chart2",
						State:     state.ChartStateOrphaned,
						Status:    state.ChartStatusFailed,
					},
					{
						Namespace: "db-ns",
						ChartName: "chart3",
						State:     state.ChartStateOrphaned,
						Status:    state.ChartStatusSucceeded,
					},
				},
			},
		},
		{
			name: "empty deployed package - returns empty map",
			deployedPackage: &state.DeployedPackage{
				Name:               "test-package",
				DeployedComponents: []state.DeployedComponent{},
			},
			opts: PruneOptions{},
			want: map[string][]state.InstalledChart{},
		},
		{
			name: "filters active charts correctly",
			deployedPackage: &state.DeployedPackage{
				Name: "test-package",
				DeployedComponents: []state.DeployedComponent{
					{
						Name: "component1",
						InstalledCharts: []state.InstalledChart{
							{
								Namespace: "podinfo",
								ChartName: "chart1",
								State:     state.ChartStateActive,
								Status:    state.ChartStatusSucceeded,
							},
							{
								Namespace: "podinfo",
								ChartName: "chart2",
								State:     state.ChartStateOrphaned,
								Status:    state.ChartStatusSucceeded,
							},
							{
								Namespace: "podinfo",
								ChartName: "chart3",
								State:     state.ChartStateActive,
								Status:    state.ChartStatusFailed,
							},
						},
					},
				},
			},
			opts: PruneOptions{},
			want: map[string][]state.InstalledChart{
				"component1": {
					{
						Namespace: "podinfo",
						ChartName: "chart2",
						State:     state.ChartStateOrphaned,
						Status:    state.ChartStatusSucceeded,
					},
				},
			},
		},
		{
			name: "multiple components with mixed orphaned and active charts",
			deployedPackage: &state.DeployedPackage{
				Name: "test-package",
				DeployedComponents: []state.DeployedComponent{
					{
						Name: "web-app",
						InstalledCharts: []state.InstalledChart{
							{
								Namespace: "frontend",
								ChartName: "nginx",
								State:     state.ChartStateActive,
								Status:    state.ChartStatusSucceeded,
							},
							{
								Namespace: "frontend",
								ChartName: "old-nginx",
								State:     state.ChartStateOrphaned,
								Status:    state.ChartStatusSucceeded,
							},
						},
					},
					{
						Name: "database",
						InstalledCharts: []state.InstalledChart{
							{
								Namespace: "backend",
								ChartName: "postgres",
								State:     state.ChartStateActive,
								Status:    state.ChartStatusSucceeded,
							},
						},
					},
					{
						Name: "cache",
						InstalledCharts: []state.InstalledChart{
							{
								Namespace: "backend",
								ChartName: "redis-v1",
								State:     state.ChartStateOrphaned,
								Status:    state.ChartStatusSucceeded,
							},
							{
								Namespace: "backend",
								ChartName: "redis-v2",
								State:     state.ChartStateActive,
								Status:    state.ChartStatusSucceeded,
							},
						},
					},
				},
			},
			opts: PruneOptions{},
			want: map[string][]state.InstalledChart{
				"web-app": {
					{
						Namespace: "frontend",
						ChartName: "old-nginx",
						State:     state.ChartStateOrphaned,
						Status:    state.ChartStatusSucceeded,
					},
				},
				"cache": {
					{
						Namespace: "backend",
						ChartName: "redis-v1",
						State:     state.ChartStateOrphaned,
						Status:    state.ChartStatusSucceeded,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetPruneableCharts(tt.deployedPackage, tt.opts)

			if tt.wantErr != "" {
				require.Error(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, result.PruneableCharts)
		})
	}
}
