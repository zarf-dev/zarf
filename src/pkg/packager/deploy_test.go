// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

func TestInternalServicesFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		components []v1alpha1.ZarfComponent
		opts       DeployOptions
		expected   []state.ServiceKey
	}{
		{
			name:       "no components",
			components: nil,
			expected:   nil,
		},
		{
			name: "full init package with no external URLs populates all four",
			components: []v1alpha1.ZarfComponent{
				{Name: "k3s"},
				{Name: "zarf-injector"},
				{Name: "zarf-seed-registry"},
				{Name: "zarf-registry"},
				{Name: "zarf-agent"},
				{Name: "git-server"},
			},
			expected: []state.ServiceKey{state.RegistryKey, state.AgentKey, state.GitKey, state.ArtifactKey},
		},
		{
			name: "external registry URL drops registry key even though registry components are present",
			components: []v1alpha1.ZarfComponent{
				{Name: "zarf-injector"},
				{Name: "zarf-seed-registry"},
				{Name: "zarf-registry"},
				{Name: "zarf-agent"},
				{Name: "git-server"},
			},
			opts: DeployOptions{
				RegistryInfo: state.RegistryInfo{Address: "https://registry.example.com"},
			},
			expected: []state.ServiceKey{state.AgentKey, state.GitKey, state.ArtifactKey},
		},
		{
			name: "external git URL does not drop git or artifact keys — git-server deploys regardless",
			components: []v1alpha1.ZarfComponent{
				{Name: "zarf-registry"},
				{Name: "git-server"},
			},
			opts: DeployOptions{
				GitServer:      state.GitServerInfo{Address: "https://git.example.com"},
				ArtifactServer: state.ArtifactServerInfo{Address: "https://artifact.example.com"},
			},
			expected: []state.ServiceKey{state.RegistryKey, state.GitKey, state.ArtifactKey},
		},
		{
			name: "registry components dedupe to registry key",
			components: []v1alpha1.ZarfComponent{
				{Name: "zarf-injector"},
				{Name: "zarf-seed-registry"},
				{Name: "zarf-registry"},
			},
			expected: []state.ServiceKey{state.RegistryKey},
		},
		{
			name: "unknown components ignored",
			components: []v1alpha1.ZarfComponent{
				{Name: "k3s"},
				{Name: "some-custom-component"},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := internalServicesFor(tt.components, tt.opts)
			require.ElementsMatch(t, tt.expected, got)
		})
	}
}
