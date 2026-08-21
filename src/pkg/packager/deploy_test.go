// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/packager/assemble"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/test/testutil"
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

func TestDeploySkipsValuesSchemaValidationWhenConfigured(t *testing.T) {
	ctx := testutil.TestContext(t)
	srcDir := filepath.Join("load", "testdata", "package-with-invalid-values")
	defined, err := load.PackageDefinition(ctx, srcDir, load.DefinitionOptions{SkipValuesSchemaValidation: true})
	require.NoError(t, err)
	pkgLayout, err := assemble.AssemblePackage(ctx, defined, srcDir, assemble.AssembleOptions{SkipSBOM: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })

	_, err = Deploy(ctx, pkgLayout, DeployOptions{})
	require.ErrorContains(t, err, "values validation failed")

	_, err = Deploy(ctx, pkgLayout, DeployOptions{SkipValuesSchemaValidation: true})
	require.NoError(t, err)
}

func TestDeployUsesTheClusterTheCallerSupplies(t *testing.T) {
	// A library caller that already holds a connection should not have Zarf reach for
	// the ambient kubeconfig. The supplied cluster is seeded with Zarf state, so if
	// the deploy reads state successfully it can only have used this connection: an
	// ambient cluster would either be absent or not carry this state.
	ctx := testutil.TestContext(t)
	srcDir := filepath.Join("load", "testdata", "package-with-values-schema")
	defined, err := load.PackageDefinition(ctx, srcDir, load.DefinitionOptions{})
	require.NoError(t, err)
	pkgLayout, err := assemble.AssemblePackage(ctx, defined, srcDir, assemble.AssembleOptions{SkipSBOM: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })

	cs := fake.NewClientset()
	c := &cluster.Cluster{
		Clientset: cs,
		Watcher:   healthchecks.NewImmediateWatcher(status.CurrentStatus),
	}
	_, err = cs.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: state.ZarfNamespaceName},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	require.NoError(t, c.SaveState(ctx, &state.State{}))

	_, err = Deploy(ctx, pkgLayout, DeployOptions{Cluster: c})
	// The deploy still fails further along against a fake clientset, but it must get
	// past loading state, which only the supplied cluster has.
	if err != nil {
		require.NotContains(t, err.Error(), "failed to load the Zarf State from the cluster")
	}
}
