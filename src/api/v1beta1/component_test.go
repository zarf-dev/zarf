// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetImages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		component Component
		expected  []string
	}{
		{
			name: "no images",
			component: Component{
				Name: "test-component",
			},
			expected: []string{},
		},
		{
			name: "only Images field",
			component: Component{
				Name: "test-component",
				ComponentSpec: ComponentSpec{
					Images: []Image{
						{Name: "docker.io/library/nginx:latest"},
						{Name: "ghcr.io/zarf-dev/zarf:v0.32.6"},
					},
				},
			},
			expected: []string{
				"docker.io/library/nginx:latest",
				"ghcr.io/zarf-dev/zarf:v0.32.6",
			},
		},
		{
			name: "only ImageArchives with images",
			component: Component{
				Name: "test-component",
				ComponentSpec: ComponentSpec{
					ImageArchives: []ImageArchive{
						{
							Path: "/tmp/images.tar",
							Images: []string{
								"docker.io/library/redis:latest",
								"docker.io/library/postgres:14",
							},
						},
					},
				},
			},
			expected: []string{
				"docker.io/library/redis:latest",
				"docker.io/library/postgres:14",
			},
		},
		{
			name: "both Images and ImageArchives",
			component: Component{
				Name: "test-component",
				ComponentSpec: ComponentSpec{
					Images: []Image{
						{Name: "docker.io/library/nginx:latest"},
					},
					ImageArchives: []ImageArchive{
						{
							Path: "/tmp/images1.tar",
							Images: []string{
								"docker.io/library/redis:latest",
							},
						},
						{
							Path: "/tmp/images2.tar",
							Images: []string{
								"docker.io/library/postgres:14",
								"ghcr.io/zarf-dev/zarf:v0.32.6",
							},
						},
					},
				},
			},
			expected: []string{
				"docker.io/library/nginx:latest",
				"docker.io/library/redis:latest",
				"docker.io/library/postgres:14",
				"ghcr.io/zarf-dev/zarf:v0.32.6",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.component.GetImages()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestComponentRequirements(t *testing.T) {
	t.Parallel()

	clusterWait := ComponentAction{Wait: &ComponentActionWait{Cluster: &ComponentActionWaitCluster{Kind: "Pod"}}}
	networkWait := ComponentAction{Wait: &ComponentActionWait{Network: &ComponentActionWaitNetwork{Protocol: "tcp"}}}
	tests := []struct {
		name            string
		component       Component
		requiresCluster bool
		requiresState   bool
	}{
		{name: "local component", component: Component{}},
		{name: "image component", component: Component{ComponentSpec: ComponentSpec{Images: []Image{{Name: "example.com/image:latest"}}}}, requiresCluster: true, requiresState: true},
		{name: "deploy cluster wait", component: Component{ComponentSpec: ComponentSpec{Actions: ComponentActions{OnDeploy: ComponentActionSet{Before: []ComponentAction{clusterWait}}}}}, requiresCluster: true},
		{name: "remove cluster wait", component: Component{ComponentSpec: ComponentSpec{Actions: ComponentActions{OnRemove: ComponentActionSet{Before: []ComponentAction{clusterWait}}}}}},
		{name: "network wait", component: Component{ComponentSpec: ComponentSpec{Actions: ComponentActions{OnDeploy: ComponentActionSet{Before: []ComponentAction{networkWait}}}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.requiresCluster, tt.component.RequiresCluster())
			require.Equal(t, tt.requiresState, tt.component.RequiresState())
		})
	}
}
