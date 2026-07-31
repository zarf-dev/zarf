// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/value"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
)

func TestInternalServicesFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		components []v1alpha1.ZarfComponent
		opts       DeployOptions
		expected   state.ServiceSet
	}{
		{
			name:       "no components",
			components: nil,
			expected:   state.NewServiceSet(),
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
			expected: state.NewServiceSet(state.RegistryKey, state.AgentKey, state.GitKey, state.ArtifactKey),
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
			expected: state.NewServiceSet(state.AgentKey, state.GitKey, state.ArtifactKey),
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
			expected: state.NewServiceSet(state.RegistryKey, state.GitKey, state.ArtifactKey),
		},
		{
			name: "registry components dedupe to registry key",
			components: []v1alpha1.ZarfComponent{
				{Name: "zarf-injector"},
				{Name: "zarf-seed-registry"},
				{Name: "zarf-registry"},
			},
			expected: state.NewServiceSet(state.RegistryKey),
		},
		{
			name: "unknown components ignored",
			components: []v1alpha1.ZarfComponent{
				{Name: "k3s"},
				{Name: "some-custom-component"},
			},
			expected: state.NewServiceSet(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := internalServicesFor(tt.components, tt.opts)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestComponentRequiresCluster(t *testing.T) {
	t.Parallel()

	initPackage := v1alpha1.ZarfPackage{Kind: v1alpha1.ZarfInitConfig}
	standardPackage := v1alpha1.ZarfPackage{Kind: v1alpha1.ZarfPackageConfig}
	requiresCluster := v1alpha1.ZarfComponent{Charts: []v1alpha1.ZarfChart{{Name: "chart"}}}

	require.True(t, componentRequiresCluster(initPackage, v1alpha1.ZarfComponent{Name: "zarf-injector"}))
	require.False(t, componentRequiresCluster(standardPackage, v1alpha1.ZarfComponent{Name: "zarf-injector"}))
	require.True(t, componentRequiresCluster(standardPackage, requiresCluster))
}

func TestInjectorPayloadSources(t *testing.T) {
	t.Parallel()

	pkg := v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{{
		Name:   "zarf-seed-registry",
		Images: []string{"registry:3", "proxy:1"},
	}}}
	images, err := injectorPayloadSources(pkg)
	require.NoError(t, err)
	require.Equal(t, []string{"registry:3", "proxy:1"}, images)

	_, err = injectorPayloadSources(v1alpha1.ZarfPackage{})
	require.ErrorContains(t, err, "requires a zarf-seed-registry component")

	_, err = injectorPayloadSources(v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{{Name: "zarf-seed-registry"}}})
	require.ErrorContains(t, err, "at least one bootstrap image")
}

func TestVerifyPackageIsDeployableSkipsAgentCertCheckWhenAgentIsNotConfigured(t *testing.T) {
	ctx := context.Background()
	cs := fake.NewClientset()
	c := &cluster.Cluster{
		Clientset: cs,
		Watcher:   healthchecks.NewImmediateWatcher(status.CurrentStatus),
	}
	_, err := cs.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: state.ZarfNamespaceName},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	require.NoError(t, c.SaveState(ctx, &state.State{}))

	d := deployer{c: c}
	err = d.verifyPackageIsDeployable(ctx, &layout.PackageLayout{})
	require.NoError(t, err)
}

func TestProxyInjectorPodSpec(t *testing.T) {
	t.Parallel()
	d := deployer{vals: value.Values{
		"injector": map[string]any{
			"proxy": map[string]any{
				"daemonSet": map[string]any{
					"podTemplate": map[string]any{
						"spec": map[string]any{
							"nodeSelector": map[string]any{"pool": "injector"},
							"tolerations": []any{map[string]any{
								"key":      "dedicated",
								"operator": "Exists",
							}},
						},
						"container": map[string]any{"image": "registry.k8s.io/pause:3.10"},
					}},
			},
		},
	}}

	podSpec, override, err := d.proxyInjectorPodSpec()
	require.NoError(t, err)
	require.Equal(t, map[string]string{"pool": "injector"}, podSpec.NodeSelector)
	require.Len(t, podSpec.Tolerations, 1)
	require.Equal(t, "registry.k8s.io/pause:3.10", override)
}
