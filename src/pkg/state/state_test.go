// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package state manages references to a logical zarf deployment in k8s.
package state

import (
	"fmt"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/pki"
)

// TODO: Change password gen method to make testing possible.
func TestMergeStateRegistry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		initRegistry     RegistryInfo
		oldRegistry      RegistryInfo
		expectedRegistry RegistryInfo
	}{
		{
			name: "username is unmodified",
			oldRegistry: RegistryInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
			},
			expectedRegistry: RegistryInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
			},
		},
		{
			name: "internal server auto generate",
			oldRegistry: RegistryInfo{
				Address:  fmt.Sprintf("%s:%d", helpers.IPV4Localhost, 1),
				NodePort: 1,
			},
			expectedRegistry: RegistryInfo{
				Address:  fmt.Sprintf("%s:%d", helpers.IPV4Localhost, 1),
				NodePort: 1,
			},
		},
		{
			name: "init options merged",
			oldRegistry: RegistryInfo{
				PushUsername: "doesn't matter",
				PullUsername: "doesn't matter",
				Address:      "doesn't matter",
				NodePort:     0,
				Secret:       "doesn't matter",
			},
			initRegistry: RegistryInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
				Address:      "address",
				NodePort:     1,
				Secret:       "secret",
			},
			expectedRegistry: RegistryInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
				Address:      "address",
				NodePort:     1,
				Secret:       "secret",
			},
		},
		{
			name: "init options not merged",
			expectedRegistry: RegistryInfo{
				PushUsername: "",
				PullUsername: "",
				Address:      "",
				NodePort:     0,
				Secret:       "",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			oldState := &State{
				RegistryInfo: tt.oldRegistry,
			}
			newState, err := Merge(oldState, MergeOptions{
				RegistryInfo: tt.initRegistry,
				Services:     []string{RegistryKey},
			})
			require.NoError(t, err)
			require.Equal(t, tt.expectedRegistry.PushUsername, newState.RegistryInfo.PushUsername)
			require.Equal(t, tt.expectedRegistry.PullUsername, newState.RegistryInfo.PullUsername)
			require.Equal(t, tt.expectedRegistry.Address, newState.RegistryInfo.Address)
			require.Equal(t, tt.expectedRegistry.NodePort, newState.RegistryInfo.NodePort)
			require.Equal(t, tt.expectedRegistry.Secret, newState.RegistryInfo.Secret)
		})
	}
}

// TODO: Change password gen method to make testing possible.
func TestMergeStateGit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		initGitServer     GitServerInfo
		oldGitServer      GitServerInfo
		expectedGitServer GitServerInfo
	}{
		{
			name: "address and usernames are unmodified",
			oldGitServer: GitServerInfo{
				Address:      "address",
				PushUsername: "push-user",
				PullUsername: "pull-user",
			},
			expectedGitServer: GitServerInfo{
				Address:      "address",
				PushUsername: "push-user",
				PullUsername: "pull-user",
			},
		},
		{
			name: "internal server auto generate",
			oldGitServer: GitServerInfo{
				Address: ZarfInClusterGitServiceURL,
			},
			expectedGitServer: GitServerInfo{
				Address: ZarfInClusterGitServiceURL,
			},
		},
		{
			name: "init options merged",
			oldGitServer: GitServerInfo{
				Address:      "doesn't matter",
				PushUsername: "doesn't matter",
				PullUsername: "doesn't matter",
			},
			initGitServer: GitServerInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
				Address:      "address",
			},
			expectedGitServer: GitServerInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
				Address:      "address",
			},
		},
		{
			name: "empty init options not merged",
			expectedGitServer: GitServerInfo{
				PushUsername: "",
				PullUsername: "",
				Address:      "",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			oldState := &State{
				GitServer: tt.oldGitServer,
			}
			newState, err := Merge(oldState, MergeOptions{
				GitServer: tt.initGitServer,
				Services:  []string{GitKey},
			})
			require.NoError(t, err)
			require.Equal(t, tt.expectedGitServer.PushUsername, newState.GitServer.PushUsername)
			require.Equal(t, tt.expectedGitServer.PullUsername, newState.GitServer.PullUsername)
			require.Equal(t, tt.expectedGitServer.Address, newState.GitServer.Address)
		})
	}
}

func TestMergeStateArtifact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		initArtifactServer     ArtifactServerInfo
		oldArtifactServer      ArtifactServerInfo
		expectedArtifactServer ArtifactServerInfo
	}{
		{
			name: "username is unmodified",
			oldArtifactServer: ArtifactServerInfo{
				PushUsername: "push-user",
			},
			expectedArtifactServer: ArtifactServerInfo{
				PushUsername: "push-user",
			},
		},
		{
			name: "old state is internal server auto generate push token",
			oldArtifactServer: ArtifactServerInfo{
				PushToken: "foobar",
				Address:   ZarfInClusterArtifactServiceURL,
			},
			expectedArtifactServer: ArtifactServerInfo{
				PushToken: "",
				Address:   ZarfInClusterArtifactServiceURL,
			},
		},
		{
			name: "old state is not internal server auto generate push token but init options does not match",
			initArtifactServer: ArtifactServerInfo{
				PushToken: "hello world",
			},
			oldArtifactServer: ArtifactServerInfo{
				PushToken: "foobar",
				Address:   ZarfInClusterArtifactServiceURL,
			},
			expectedArtifactServer: ArtifactServerInfo{
				PushToken: "hello world",
				Address:   ZarfInClusterArtifactServiceURL,
			},
		},
		{
			name: "init options merged",
			oldArtifactServer: ArtifactServerInfo{
				PushUsername: "doesn't matter",
				PushToken:    "doesn't matter",
				Address:      "doesn't matter",
			},
			initArtifactServer: ArtifactServerInfo{
				PushUsername: "user",
				PushToken:    "token",
				Address:      "address",
			},
			expectedArtifactServer: ArtifactServerInfo{
				PushUsername: "user",
				PushToken:    "token",
				Address:      "address",
			},
		},
		{
			name: "empty init options not merged",
			expectedArtifactServer: ArtifactServerInfo{
				PushUsername: "",
				PushToken:    "",
				Address:      "",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			oldState := &State{
				ArtifactServer: tt.oldArtifactServer,
			}
			newState, err := Merge(oldState, MergeOptions{
				ArtifactServer: tt.initArtifactServer,
				Services:       []string{ArtifactKey},
			})
			require.NoError(t, err)
			require.Equal(t, tt.expectedArtifactServer, newState.ArtifactServer)
		})
	}
}

func TestMergeStateAgent(t *testing.T) {
	t.Parallel()

	agentTLS, err := pki.GeneratePKI("example.com")
	require.NoError(t, err)
	oldState := &State{
		AgentTLS: agentTLS,
	}
	newState, err := Merge(oldState, MergeOptions{
		Services: []string{AgentKey},
	})
	require.NoError(t, err)
	require.NotEqual(t, oldState.AgentTLS, newState.AgentTLS)
}

func TestMergeInstalledChartsForComponent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		partial         bool
		existingCharts  []InstalledChart
		installedCharts []InstalledChart
		expectedCharts  []InstalledChart
	}{
		{
			name: "existing charts are merged - no existing state",
			existingCharts: []InstalledChart{
				{
					Namespace: "default",
					ChartName: "chart1",
				},
				{
					Namespace: "default",
					ChartName: "chart2",
				},
			},
			installedCharts: []InstalledChart{
				{
					Namespace: "default",
					ChartName: "chart3",
					Status:    ChartStatusSucceeded,
				},
			},
			expectedCharts: []InstalledChart{
				{
					Namespace: "default",
					ChartName: "chart1",
					State:     ChartStateOrphaned,
				},
				{
					Namespace: "default",
					ChartName: "chart2",
					State:     ChartStateOrphaned,
				},
				{
					Namespace: "default",
					ChartName: "chart3",
					State:     ChartStateActive,
					Status:    ChartStatusSucceeded,
				},
			},
		},
		{
			name: "existing charts are merged - existing state",
			existingCharts: []InstalledChart{
				{
					Namespace: "default",
					ChartName: "chart1",
					State:     ChartStateActive,
				},
				{
					Namespace: "default",
					ChartName: "chart2",
					State:     ChartStateOrphaned,
				},
			},
			installedCharts: []InstalledChart{
				{
					Namespace: "default",
					ChartName: "chart3",
					Status:    ChartStatusSucceeded,
				},
			},
			expectedCharts: []InstalledChart{
				{
					Namespace: "default",
					ChartName: "chart1",
					State:     ChartStateOrphaned,
				},
				{
					Namespace: "default",
					ChartName: "chart2",
					State:     ChartStateOrphaned,
				},
				{
					Namespace: "default",
					ChartName: "chart3",
					State:     ChartStateActive,
					Status:    ChartStatusSucceeded,
				},
			},
		},
		{
			name: "overlapping charts are merged",
			existingCharts: []InstalledChart{
				{
					Namespace: "default",
					ChartName: "chart1",
				},
				{
					Namespace: "default",
					ChartName: "chart2",
				},
			},
			installedCharts: []InstalledChart{
				{
					Namespace: "default",
					ChartName: "chart1",
					Status:    ChartStatusSucceeded,
				},
			},
			expectedCharts: []InstalledChart{
				{
					Namespace: "default",
					ChartName: "chart1",
					State:     ChartStateActive,
					Status:    ChartStatusSucceeded,
				},
				{
					Namespace: "default",
					ChartName: "chart2",
					State:     ChartStateOrphaned,
				},
			},
		},
		{
			name:    "existing charts are merged with partial true",
			partial: true,
			existingCharts: []InstalledChart{
				{
					Namespace: "default",
					ChartName: "chart1",
					State:     ChartStateActive,
				},
				{
					Namespace: "default",
					ChartName: "chart2",
					State:     ChartStateOrphaned,
				},
			},
			installedCharts: []InstalledChart{
				{
					Namespace: "default",
					ChartName: "chart3",
					Status:    ChartStatusFailed,
				},
			},
			expectedCharts: []InstalledChart{
				{
					Namespace: "default",
					ChartName: "chart1",
					State:     ChartStateActive,
				},
				{
					Namespace: "default",
					ChartName: "chart2",
					State:     ChartStateOrphaned,
				},
				{
					Namespace: "default",
					ChartName: "chart3",
					State:     ChartStateActive,
					Status:    ChartStatusFailed,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Parallel()
			actual := MergeInstalledChartsForComponent(tt.existingCharts, tt.installedCharts, tt.partial)
			require.ElementsMatch(t, tt.expectedCharts, actual)
		})
	}
}

func TestDeployedPackage_GetPrunableCharts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		deployedPackage *DeployedPackage
		componentFilter string
		chartFilter     string
		wantCharts      map[string][]InstalledChart
		wantErr         string
	}{
		{
			name: "no filters - returns all orphaned charts",
			deployedPackage: &DeployedPackage{
				DeployedComponents: []DeployedComponent{
					{
						Name: "comp1",
						InstalledCharts: []InstalledChart{
							{Namespace: "ns1", ChartName: "chart1", State: ChartStateOrphaned},
							{Namespace: "ns1", ChartName: "chart2", State: ChartStateActive},
						},
					},
					{
						Name: "comp2",
						InstalledCharts: []InstalledChart{
							{Namespace: "ns2", ChartName: "chart3", State: ChartStateOrphaned},
						},
					},
				},
			},
			componentFilter: "",
			chartFilter:     "",
			wantCharts: map[string][]InstalledChart{
				"comp1": {{Namespace: "ns1", ChartName: "chart1", State: ChartStateOrphaned}},
				"comp2": {{Namespace: "ns2", ChartName: "chart3", State: ChartStateOrphaned}},
			},
		},
		{
			name: "component filter - returns only charts from specified component",
			deployedPackage: &DeployedPackage{
				DeployedComponents: []DeployedComponent{
					{
						Name: "comp1",
						InstalledCharts: []InstalledChart{
							{Namespace: "ns1", ChartName: "chart1", State: ChartStateOrphaned},
						},
					},
					{
						Name: "comp2",
						InstalledCharts: []InstalledChart{
							{Namespace: "ns2", ChartName: "chart2", State: ChartStateOrphaned},
						},
					},
				},
			},
			componentFilter: "comp1",
			chartFilter:     "",
			wantCharts: map[string][]InstalledChart{
				"comp1": {{Namespace: "ns1", ChartName: "chart1", State: ChartStateOrphaned}},
			},
		},
		{
			name: "component and chart filter - returns specific chart",
			deployedPackage: &DeployedPackage{
				DeployedComponents: []DeployedComponent{
					{
						Name: "comp1",
						InstalledCharts: []InstalledChart{
							{Namespace: "ns1", ChartName: "chart1", State: ChartStateOrphaned},
							{Namespace: "ns1", ChartName: "chart2", State: ChartStateOrphaned},
						},
					},
				},
			},
			componentFilter: "comp1",
			chartFilter:     "chart1",
			wantCharts: map[string][]InstalledChart{
				"comp1": {{Namespace: "ns1", ChartName: "chart1", State: ChartStateOrphaned}},
			},
		},
		{
			name: "no orphaned charts - returns empty map",
			deployedPackage: &DeployedPackage{
				DeployedComponents: []DeployedComponent{
					{
						Name: "comp1",
						InstalledCharts: []InstalledChart{
							{Namespace: "ns1", ChartName: "chart1", State: ChartStateActive},
						},
					},
				},
			},
			componentFilter: "",
			chartFilter:     "",
			wantCharts:      map[string][]InstalledChart{},
		},
		{
			name: "component not found - returns error",
			deployedPackage: &DeployedPackage{
				DeployedComponents: []DeployedComponent{
					{Name: "comp1", InstalledCharts: []InstalledChart{}},
				},
			},
			componentFilter: "nonexistent",
			chartFilter:     "",
			wantErr:         "component \"nonexistent\" not found in deployed package",
		},
		{
			name: "chart filter without component - returns error",
			deployedPackage: &DeployedPackage{
				DeployedComponents: []DeployedComponent{},
			},
			componentFilter: "",
			chartFilter:     "chart1",
			wantErr:         "component must be specified when chart filter is provided",
		},
		{
			name: "chart not found in component - returns error",
			deployedPackage: &DeployedPackage{
				DeployedComponents: []DeployedComponent{
					{
						Name: "comp1",
						InstalledCharts: []InstalledChart{
							{Namespace: "ns1", ChartName: "chart1", State: ChartStateOrphaned},
						},
					},
				},
			},
			componentFilter: "comp1",
			chartFilter:     "nonexistent",
			wantErr:         "chart \"nonexistent\" not found in deployed package",
		},
		{
			name: "chart found but not orphaned - returns error",
			deployedPackage: &DeployedPackage{
				DeployedComponents: []DeployedComponent{
					{
						Name: "comp1",
						InstalledCharts: []InstalledChart{
							{Namespace: "ns1", ChartName: "chart1", State: ChartStateActive},
						},
					},
				},
			},
			componentFilter: "comp1",
			chartFilter:     "chart1",
			wantErr:         "chart \"chart1\" found in deployed package, but is not in the \"Orphaned\" state",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := tt.deployedPackage.GetPrunableCharts(tt.componentFilter, tt.chartFilter)

			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantCharts, result)
		})
	}
}

func TestDeployedPackage_RemovePrunedCharts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		deployedPackage *DeployedPackage
		prunedCharts    map[string][]InstalledChart
		expectedCharts  map[string][]InstalledChart
	}{
		{
			name: "removes specified charts from component",
			deployedPackage: &DeployedPackage{
				DeployedComponents: []DeployedComponent{
					{
						Name: "comp1",
						InstalledCharts: []InstalledChart{
							{Namespace: "ns1", ChartName: "chart1", State: ChartStateOrphaned},
							{Namespace: "ns1", ChartName: "chart2", State: ChartStateActive},
							{Namespace: "ns1", ChartName: "chart3", State: ChartStateOrphaned},
						},
					},
				},
			},
			prunedCharts: map[string][]InstalledChart{
				"comp1": {
					{Namespace: "ns1", ChartName: "chart1", State: ChartStateOrphaned},
					{Namespace: "ns1", ChartName: "chart3", State: ChartStateOrphaned},
				},
			},
			expectedCharts: map[string][]InstalledChart{
				"comp1": {
					{Namespace: "ns1", ChartName: "chart2", State: ChartStateActive},
				},
			},
		},
		{
			name: "removes charts from multiple components",
			deployedPackage: &DeployedPackage{
				DeployedComponents: []DeployedComponent{
					{
						Name: "comp1",
						InstalledCharts: []InstalledChart{
							{Namespace: "ns1", ChartName: "chart1", State: ChartStateOrphaned},
							{Namespace: "ns1", ChartName: "chart2", State: ChartStateActive},
						},
					},
					{
						Name: "comp2",
						InstalledCharts: []InstalledChart{
							{Namespace: "ns2", ChartName: "chart3", State: ChartStateOrphaned},
							{Namespace: "ns2", ChartName: "chart4", State: ChartStateActive},
						},
					},
				},
			},
			prunedCharts: map[string][]InstalledChart{
				"comp1": {{Namespace: "ns1", ChartName: "chart1", State: ChartStateOrphaned}},
				"comp2": {{Namespace: "ns2", ChartName: "chart3", State: ChartStateOrphaned}},
			},
			expectedCharts: map[string][]InstalledChart{
				"comp1": {{Namespace: "ns1", ChartName: "chart2", State: ChartStateActive}},
				"comp2": {{Namespace: "ns2", ChartName: "chart4", State: ChartStateActive}},
			},
		},
		{
			name: "does nothing if component not in pruned list",
			deployedPackage: &DeployedPackage{
				DeployedComponents: []DeployedComponent{
					{
						Name: "comp1",
						InstalledCharts: []InstalledChart{
							{Namespace: "ns1", ChartName: "chart1", State: ChartStateActive},
						},
					},
					{
						Name: "comp2",
						InstalledCharts: []InstalledChart{
							{Namespace: "ns2", ChartName: "chart2", State: ChartStateOrphaned},
						},
					},
				},
			},
			prunedCharts: map[string][]InstalledChart{
				"comp2": {{Namespace: "ns2", ChartName: "chart2", State: ChartStateOrphaned}},
			},
			expectedCharts: map[string][]InstalledChart{
				"comp1": {{Namespace: "ns1", ChartName: "chart1", State: ChartStateActive}},
				"comp2": {},
			},
		},
		{
			name: "removes all charts from component",
			deployedPackage: &DeployedPackage{
				DeployedComponents: []DeployedComponent{
					{
						Name: "comp1",
						InstalledCharts: []InstalledChart{
							{Namespace: "ns1", ChartName: "chart1", State: ChartStateOrphaned},
						},
					},
				},
			},
			prunedCharts: map[string][]InstalledChart{
				"comp1": {{Namespace: "ns1", ChartName: "chart1", State: ChartStateOrphaned}},
			},
			expectedCharts: map[string][]InstalledChart{
				"comp1": {},
			},
		},
		{
			name: "handles empty pruned charts map",
			deployedPackage: &DeployedPackage{
				DeployedComponents: []DeployedComponent{
					{
						Name: "comp1",
						InstalledCharts: []InstalledChart{
							{Namespace: "ns1", ChartName: "chart1", State: ChartStateActive},
						},
					},
				},
			},
			prunedCharts: map[string][]InstalledChart{},
			expectedCharts: map[string][]InstalledChart{
				"comp1": {{Namespace: "ns1", ChartName: "chart1", State: ChartStateActive}},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a copy to avoid modifying the test data
			testPackage := &DeployedPackage{
				DeployedComponents: make([]DeployedComponent, len(tt.deployedPackage.DeployedComponents)),
			}
			copy(testPackage.DeployedComponents, tt.deployedPackage.DeployedComponents)

			testPackage.RemovePrunedCharts(tt.prunedCharts)

			// Verify each component has the expected charts
			for i, component := range testPackage.DeployedComponents {
				expectedCharts := tt.expectedCharts[component.Name]
				require.ElementsMatch(t, expectedCharts, component.InstalledCharts,
					"component %s charts mismatch at index %d", component.Name, i)
			}
		})
	}
}
