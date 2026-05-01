// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/pki"
	"github.com/zarf-dev/zarf/src/pkg/state"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
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
	err = d.verifyPackageIsDeployable(ctx, v1alpha1.ZarfPackage{})
	require.NoError(t, err)
}

func TestApplyConnectedDeployAgentState(t *testing.T) {
	t.Parallel()

	t.Run("copies agent state when cluster state has agent configured", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		cs := fake.NewClientset(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: state.ZarfNamespaceName},
		})
		c := &cluster.Cluster{Clientset: cs}
		agentTLS := pki.GeneratedPKI{
			CA:   []byte("ca"),
			Cert: []byte("cert"),
			Key:  []byte("key"),
		}
		require.NoError(t, c.SaveState(ctx, &state.State{
			AgentTLS:             agentTLS,
			AgentTLSUserProvided: true,
		}))

		s, err := state.Default()
		require.NoError(t, err)
		require.NoError(t, applyConnectedDeployAgentState(ctx, s, c))

		require.Equal(t, agentTLS, s.AgentTLS)
		require.True(t, s.AgentTLSUserProvided)
	})

	t.Run("keeps default state when cluster state is absent", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		c := &cluster.Cluster{Clientset: fake.NewClientset()}
		s, err := state.Default()
		require.NoError(t, err)

		require.NoError(t, applyConnectedDeployAgentState(ctx, s, c))
		require.False(t, s.AgentIsConfigured())
	})

	t.Run("returns non-not-found load errors", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		cs := fake.NewClientset()
		cs.Fake.PrependReactor("get", "secrets", func(ktesting.Action) (bool, runtime.Object, error) {
			return true, nil, kerrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, state.ZarfStateSecretName, errors.New("denied"))
		})
		c := &cluster.Cluster{Clientset: cs}
		s, err := state.Default()
		require.NoError(t, err)

		require.Error(t, applyConnectedDeployAgentState(ctx, s, c))
	})
}
