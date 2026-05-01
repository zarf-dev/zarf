// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/pkg/pki"
	"github.com/zarf-dev/zarf/src/pkg/state"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
)

func TestGetIPFamily(t *testing.T) {
	tests := []struct {
		name          string
		protocolsUsed []corev1.IPFamily
		expected      state.IPFamily
	}{
		{
			name:          "dual stack support",
			protocolsUsed: []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol},
			expected:      state.IPFamilyDualStack,
		},
		{
			name:          "ipv4 only support",
			protocolsUsed: []corev1.IPFamily{corev1.IPv4Protocol},
			expected:      state.IPFamilyIPv4,
		},
		{
			name:          "ipv6 only support",
			protocolsUsed: []corev1.IPFamily{corev1.IPv6Protocol},
			expected:      state.IPFamilyIPv6,
		},
		{
			name:          "ipv6 only support",
			protocolsUsed: []corev1.IPFamily{corev1.IPv6Protocol},
			expected:      state.IPFamilyIPv6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cs := fake.NewClientset()
			immediateWatcher := healthchecks.NewImmediateWatcher(status.CurrentStatus)

			c := &Cluster{
				Clientset: cs,
				Watcher:   immediateWatcher,
			}

			// Create the service with the IP families based on the test case
			testService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "zarf-ip-family-test",
					Namespace: state.ZarfNamespaceName,
				},
				Spec: corev1.ServiceSpec{
					IPFamilies: tt.protocolsUsed,
				},
			}

			// mimic the cluster setting setting the IP family
			cs.PrependReactor("patch", "services", func(_ k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, testService, nil
			})

			cs.PrependReactor("get", "services", func(_ k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, testService, nil
			})

			cs.PrependReactor("delete", "services", func(_ k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, nil
			})

			ipFamily, err := c.GetIPFamily(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.expected, ipFamily)
		})
	}
}

func TestInit(t *testing.T) {
	s, err := state.Default()
	require.NoError(t, err)
	sData, err := json.Marshal(s)
	require.NoError(t, err)

	existingState := state.State{
		Distro: DistroIsK3d,
		RegistryInfo: state.RegistryInfo{
			PushUsername: "push-user",
			PullUsername: "pull-user",
			Address:      "address",
			NodePort:     1,
			Secret:       "secret",
		},
	}

	existingStateData, err := json.Marshal(existingState)
	require.NoError(t, err)

	tests := []struct {
		name        string
		initOpts    InitStateOptions
		nodes       []corev1.Node
		namespaces  []corev1.Namespace
		secrets     []corev1.Secret
		expectedErr string
	}{
		{
			name:        "no nodes in cluster",
			expectedErr: "cannot init Zarf state in empty cluster",
		},
		{
			name:     "no namespaces exist",
			initOpts: InitStateOptions{},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node",
					},
				},
			},
		},
		{
			name: "namespaces exists",
			initOpts: InitStateOptions{
				InternalServices: state.NewServiceSet(state.AgentKey),
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node",
					},
				},
			},
			namespaces: []corev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kube-system",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "default",
					},
				},
			},
		},
		{
			name: "namespaces exist without agent service",
			initOpts: InitStateOptions{
				InternalServices: state.NewServiceSet(state.RegistryKey),
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node",
					},
				},
			},
			namespaces: []corev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kube-system",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "default",
					},
				},
			},
		},
		{
			name: "Zarf namespace exists",
			initOpts: InitStateOptions{
				InternalServices: state.NewServiceSet(state.AgentKey),
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node",
					},
				},
			},
			namespaces: []corev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: state.ZarfNamespaceName,
					},
				},
			},
		},
		{
			name: "empty Zarf state exists",
			initOpts: InitStateOptions{
				InternalServices: state.NewServiceSet(state.AgentKey),
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node",
					},
				},
			},
			namespaces: []corev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: state.ZarfNamespaceName,
					},
				},
			},
			secrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: state.ZarfNamespaceName,
						Name:      state.ZarfStateSecretName,
					},
					Data: map[string][]byte{
						state.ZarfStateDataKey: sData,
					},
				},
			},
		},
		{
			name: "Zarf state exists",
			initOpts: InitStateOptions{
				InternalServices: state.NewServiceSet(state.AgentKey),
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node",
					},
				},
			},
			namespaces: []corev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: state.ZarfNamespaceName,
					},
				},
			},
			secrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: state.ZarfNamespaceName,
						Name:      state.ZarfStateSecretName,
					},
					Data: map[string][]byte{
						state.ZarfStateDataKey: existingStateData,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			immediateWatcher := healthchecks.NewImmediateWatcher(status.CurrentStatus)
			ctx := context.Background()
			cs := fake.NewClientset()
			for _, node := range tt.nodes {
				_, err := cs.CoreV1().Nodes().Create(ctx, &node, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			for _, namespace := range tt.namespaces {
				_, err := cs.CoreV1().Namespaces().Create(ctx, &namespace, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			for _, secret := range tt.secrets {
				_, err := cs.CoreV1().Secrets(secret.ObjectMeta.Namespace).Create(ctx, &secret, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			c := &Cluster{
				Clientset: cs,
				Watcher:   immediateWatcher,
			}

			// Create default service account in Zarf namespace
			go func() {
				for {
					time.Sleep(1 * time.Second)
					ns, err := cs.CoreV1().Namespaces().Get(ctx, state.ZarfNamespaceName, metav1.GetOptions{})
					if err != nil {
						continue
					}
					sa := &corev1.ServiceAccount{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: ns.Name,
							Name:      "default",
						},
					}
					//nolint:errcheck // ignore
					cs.CoreV1().ServiceAccounts(ns.Name).Create(ctx, sa, metav1.CreateOptions{})
					break
				}
			}()

			svc := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "zarf-ip-family-test",
					Namespace: state.ZarfNamespaceName,
				},
				Spec: corev1.ServiceSpec{
					IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol},
				},
			}
			_, err := cs.CoreV1().Services(svc.Namespace).Create(ctx, svc, metav1.CreateOptions{})
			require.NoError(t, err)

			_, err = c.InitState(ctx, tt.initOpts)
			if tt.expectedErr != "" {
				require.EqualError(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			s, err := cs.CoreV1().Secrets(state.ZarfNamespaceName).Get(ctx, state.ZarfStateSecretName, metav1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, map[string]string{"app.kubernetes.io/managed-by": "zarf"}, s.Labels)
			if tt.secrets != nil {
				return
			}
			zarfNs, err := cs.CoreV1().Namespaces().Get(ctx, state.ZarfNamespaceName, metav1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, map[string]string{"app.kubernetes.io/managed-by": "zarf"}, zarfNs.Labels)
			for _, ns := range tt.namespaces {
				if ns.Name == zarfNs.Name {
					continue
				}
				ns, err := cs.CoreV1().Namespaces().Get(ctx, ns.Name, metav1.GetOptions{})
				require.NoError(t, err)
				if tt.initOpts.InternalServices.Has(state.AgentKey) {
					require.Equal(t, map[string]string{AgentLabel: "ignore"}, ns.Labels)
				} else {
					require.NotContains(t, ns.Labels, AgentLabel)
				}
			}
		})
	}
}

func TestInitStateRegistryModeSwitch(t *testing.T) {
	tests := []struct {
		name     string
		current  state.State
		opts     InitStateOptions
		expected state.State
	}{
		{
			name: "nodeport to proxy resets injector port, port defaults to 5000, and enables mTLS",
			current: state.State{
				RegistryInfo: state.RegistryInfo{
					RegistryMode: state.RegistryModeNodePort,
					MTLSStrategy: state.MTLSStrategyNone,
				},
				InjectorInfo: state.InjectorInfo{Port: 31999},
			},
			opts: InitStateOptions{
				RegistryInfo: state.RegistryInfo{RegistryMode: state.RegistryModeProxy},
			},
			expected: state.State{
				RegistryInfo: state.RegistryInfo{
					RegistryMode: state.RegistryModeProxy,
					MTLSStrategy: state.MTLSStrategyZarfManaged,
					NodePort:     state.ZarfRegistryHostPort,
				},
				InjectorInfo: state.InjectorInfo{Port: 0},
			},
		},
		{
			name: "proxy to nodeport resets injector port and corrects out-of-range port",
			current: state.State{
				RegistryInfo: state.RegistryInfo{
					RegistryMode: state.RegistryModeProxy,
					MTLSStrategy: state.MTLSStrategyZarfManaged,
				},
				InjectorInfo: state.InjectorInfo{Port: 5000},
			},
			opts: InitStateOptions{
				RegistryInfo: state.RegistryInfo{RegistryMode: state.RegistryModeNodePort},
			},
			expected: state.State{
				RegistryInfo: state.RegistryInfo{
					RegistryMode: state.RegistryModeNodePort,
					MTLSStrategy: state.MTLSStrategyNone,
					NodePort:     state.ZarfInClusterContainerRegistryNodePort,
				},
				InjectorInfo: state.InjectorInfo{Port: 0},
			},
		},
		{
			name: "proxy to nodeport with explicit valid port uses provided port",
			current: state.State{
				RegistryInfo: state.RegistryInfo{
					RegistryMode: state.RegistryModeProxy,
					MTLSStrategy: state.MTLSStrategyZarfManaged,
				},
				InjectorInfo: state.InjectorInfo{Port: 5000},
			},
			opts: InitStateOptions{
				RegistryInfo: state.RegistryInfo{
					RegistryMode: state.RegistryModeNodePort,
					Port:         30500,
				},
			},
			expected: state.State{
				RegistryInfo: state.RegistryInfo{
					RegistryMode: state.RegistryModeNodePort,
					MTLSStrategy: state.MTLSStrategyNone,
					Port:         30500,
					NodePort:     30500,
				},
				InjectorInfo: state.InjectorInfo{Port: 0},
			},
		},
		{
			name: "nodeport to proxy with explicit port uses provided port",
			current: state.State{
				RegistryInfo: state.RegistryInfo{
					RegistryMode: state.RegistryModeNodePort,
					MTLSStrategy: state.MTLSStrategyNone,
				},
				InjectorInfo: state.InjectorInfo{Port: 31999},
			},
			opts: InitStateOptions{
				RegistryInfo: state.RegistryInfo{
					RegistryMode: state.RegistryModeProxy,
					Port:         8080,
				},
			},
			expected: state.State{
				RegistryInfo: state.RegistryInfo{
					RegistryMode: state.RegistryModeProxy,
					MTLSStrategy: state.MTLSStrategyZarfManaged,
					Port:         8080,
					NodePort:     8080,
				},
				InjectorInfo: state.InjectorInfo{Port: 0},
			},
		},
		{
			name: "nodeport to nodeport preserves existing port and injector port",
			current: state.State{
				RegistryInfo: state.RegistryInfo{
					RegistryMode: state.RegistryModeNodePort,
					MTLSStrategy: state.MTLSStrategyNone,
					Port:         30500,
					NodePort:     30500,
				},
				InjectorInfo: state.InjectorInfo{Port: 31999},
			},
			opts: InitStateOptions{
				RegistryInfo: state.RegistryInfo{RegistryMode: state.RegistryModeNodePort},
			},
			expected: state.State{
				RegistryInfo: state.RegistryInfo{
					RegistryMode: state.RegistryModeNodePort,
					MTLSStrategy: state.MTLSStrategyNone,
					Port:         30500,
					NodePort:     30500,
				},
				InjectorInfo: state.InjectorInfo{Port: 31999},
			},
		},
		{
			name: "proxy to proxy preserves injector port and refreshes mTLS",
			current: state.State{
				RegistryInfo: state.RegistryInfo{
					RegistryMode: state.RegistryModeProxy,
					MTLSStrategy: state.MTLSStrategyZarfManaged,
				},
				InjectorInfo: state.InjectorInfo{Port: 5000},
			},
			opts: InitStateOptions{
				RegistryInfo: state.RegistryInfo{RegistryMode: state.RegistryModeProxy},
			},
			expected: state.State{
				RegistryInfo: state.RegistryInfo{
					RegistryMode: state.RegistryModeProxy,
					MTLSStrategy: state.MTLSStrategyZarfManaged,
				},
				InjectorInfo: state.InjectorInfo{Port: 5000},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cs := fake.NewClientset()
			c := &Cluster{
				Clientset: cs,
				Watcher:   healthchecks.NewImmediateWatcher(status.CurrentStatus),
			}

			// Seed the fake cluster with the minimum objects InitState expects:
			// a node, the zarf namespace, the state secret, and the IP family service.
			tt.current.Distro = DistroIsK3d
			tt.current.RegistryInfo.PushUsername = "push-user"
			tt.current.RegistryInfo.PullUsername = "pull-user"
			tt.current.RegistryInfo.Secret = "secret"
			if tt.current.RegistryInfo.Port == 0 {
				tt.current.RegistryInfo.Port = state.ZarfInClusterContainerRegistryNodePort
			}
			tt.current.RegistryInfo.Address = fmt.Sprintf("127.0.0.1:%d", tt.current.RegistryInfo.Port)
			currentData, err := json.Marshal(tt.current)
			require.NoError(t, err)

			_, err = cs.CoreV1().Nodes().Create(ctx, &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node"},
			}, metav1.CreateOptions{})
			require.NoError(t, err)
			_, err = cs.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: state.ZarfNamespaceName},
			}, metav1.CreateOptions{})
			require.NoError(t, err)
			_, err = cs.CoreV1().Secrets(state.ZarfNamespaceName).Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: state.ZarfNamespaceName, Name: state.ZarfStateSecretName},
				Data:       map[string][]byte{state.ZarfStateDataKey: currentData},
			}, metav1.CreateOptions{})
			require.NoError(t, err)
			_, err = cs.CoreV1().Services(state.ZarfNamespaceName).Create(ctx, &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "zarf-ip-family-test", Namespace: state.ZarfNamespaceName},
				Spec:       corev1.ServiceSpec{IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol}},
			}, metav1.CreateOptions{})
			require.NoError(t, err)

			tt.opts.InternalServices = state.NewServiceSet(state.RegistryKey)
			result, err := c.InitState(ctx, tt.opts)
			require.NoError(t, err)

			require.Equal(t, tt.expected.RegistryInfo.RegistryMode, result.RegistryInfo.RegistryMode)
			require.Equal(t, tt.expected.InjectorInfo.Port, result.InjectorInfo.Port)
			require.Equal(t, tt.expected.RegistryInfo.MTLSStrategy, result.RegistryInfo.MTLSStrategy)
			if tt.expected.RegistryInfo.Port != 0 {
				require.Equal(t, tt.expected.RegistryInfo.Port, result.RegistryInfo.Port)
				require.Equal(t, tt.expected.RegistryInfo.Port, result.RegistryInfo.NodePort) //nolint:staticcheck // verify backwards compat sync
			}
		})
	}
}

func newFakeInitStateCluster(ctx context.Context, t *testing.T, existing *state.State) *Cluster {
	t.Helper()
	cs := fake.NewClientset()
	c := &Cluster{
		Clientset: cs,
		Watcher:   healthchecks.NewImmediateWatcher(status.CurrentStatus),
	}

	_, err := cs.CoreV1().Nodes().Create(ctx, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node"}},
		metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = cs.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: state.ZarfNamespaceName}},
		metav1.CreateOptions{})
	require.NoError(t, err)
	if existing != nil {
		data, err := json.Marshal(existing)
		require.NoError(t, err)
		_, err = cs.CoreV1().Secrets(state.ZarfNamespaceName).Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: state.ZarfNamespaceName, Name: state.ZarfStateSecretName},
			Data:       map[string][]byte{state.ZarfStateDataKey: data},
		}, metav1.CreateOptions{})
		require.NoError(t, err)
	}
	_, err = cs.CoreV1().Services(state.ZarfNamespaceName).Create(ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "zarf-ip-family-test", Namespace: state.ZarfNamespaceName},
		Spec:       corev1.ServiceSpec{IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol}},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	if existing == nil {
		_, err = cs.CoreV1().ServiceAccounts(state.ZarfNamespaceName).Create(ctx, &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Namespace: state.ZarfNamespaceName, Name: "default"},
		}, metav1.CreateOptions{})
		require.NoError(t, err)
	}
	return c
}

func TestInitStateServicesGating(t *testing.T) {
	t.Run("new cluster without git service leaves git and artifact empty", func(t *testing.T) {
		ctx := context.Background()
		c := newFakeInitStateCluster(ctx, t, nil)
		s, err := c.InitState(ctx, InitStateOptions{
			InternalServices: state.NewServiceSet(state.RegistryKey, state.AgentKey),
		})
		require.NoError(t, err)
		require.Empty(t, s.GitServer.Address)
		require.Empty(t, s.GitServer.PushPassword)
		require.Empty(t, s.ArtifactServer.Address)
		require.False(t, s.GitServer.IsConfigured())
		require.NotEmpty(t, s.RegistryInfo.Address)
		require.NotEmpty(t, s.AgentTLS.Cert)
	})

	t.Run("new cluster without agent service leaves agent TLS empty", func(t *testing.T) {
		ctx := context.Background()
		c := newFakeInitStateCluster(ctx, t, nil)
		s, err := c.InitState(ctx, InitStateOptions{
			InternalServices: state.NewServiceSet(state.RegistryKey),
		})
		require.NoError(t, err)
		require.Empty(t, s.AgentTLS.Cert)
		require.False(t, s.AgentTLSUserProvided)
	})

	t.Run("new cluster with all services populates everything", func(t *testing.T) {
		ctx := context.Background()
		c := newFakeInitStateCluster(ctx, t, nil)
		s, err := c.InitState(ctx, InitStateOptions{
			InternalServices: state.NewServiceSet(state.RegistryKey, state.GitKey, state.ArtifactKey, state.AgentKey),
		})
		require.NoError(t, err)
		require.True(t, s.GitServer.IsConfigured())
		require.True(t, s.ArtifactServer.IsInternal())
		require.NotEmpty(t, s.RegistryInfo.Address)
		require.NotEmpty(t, s.AgentTLS.Cert)
	})

	t.Run("re-init adds a missing git service without overwriting existing registry", func(t *testing.T) {
		ctx := context.Background()
		existing := &state.State{
			Distro: DistroIsK3d,
			RegistryInfo: state.RegistryInfo{
				Address:      "127.0.0.1:31999",
				Port:         31999,
				RegistryMode: state.RegistryModeNodePort,
				PushUsername: "push-user",
				PullUsername: "pull-user",
				Secret:       "secret",
			},
		}
		c := newFakeInitStateCluster(ctx, t, existing)
		s, err := c.InitState(ctx, InitStateOptions{
			InternalServices: state.NewServiceSet(state.GitKey, state.ArtifactKey),
		})
		require.NoError(t, err)
		require.True(t, s.GitServer.IsConfigured())
		require.Equal(t, "127.0.0.1:31999", s.RegistryInfo.Address)
		require.NotEmpty(t, s.ArtifactServer.Address)
	})

	t.Run("re-init adds missing agent and ignores existing namespaces", func(t *testing.T) {
		ctx := context.Background()
		existing := &state.State{
			Distro: DistroIsK3d,
			RegistryInfo: state.RegistryInfo{
				Address:      "127.0.0.1:31999",
				Port:         31999,
				RegistryMode: state.RegistryModeNodePort,
				PushUsername: "push-user",
				PullUsername: "pull-user",
				Secret:       "secret",
			},
		}
		c := newFakeInitStateCluster(ctx, t, existing)
		_, err := c.Clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "app"},
		}, metav1.CreateOptions{})
		require.NoError(t, err)

		s, err := c.InitState(ctx, InitStateOptions{
			InternalServices: state.NewServiceSet(state.AgentKey),
		})
		require.NoError(t, err)
		require.True(t, s.AgentIsConfigured())
		require.False(t, s.AgentTLSUserProvided)

		ns, err := c.Clientset.CoreV1().Namespaces().Get(ctx, "app", metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "ignore", ns.Labels[AgentLabel])
	})

	t.Run("re-init uses provided agent TLS when adding missing agent", func(t *testing.T) {
		ctx := context.Background()
		existing := &state.State{Distro: DistroIsK3d}
		c := newFakeInitStateCluster(ctx, t, existing)
		agentTLS := pki.GeneratedPKI{
			CA:   []byte("ca"),
			Cert: []byte("cert"),
			Key:  []byte("key"),
		}

		s, err := c.InitState(ctx, InitStateOptions{
			InternalServices: state.NewServiceSet(state.AgentKey),
			AgentTLS:         &agentTLS,
		})
		require.NoError(t, err)
		require.Equal(t, agentTLS, s.AgentTLS)
		require.True(t, s.AgentTLSUserProvided)
	})

	t.Run("new cluster with external git URL persists without being in InternalServices", func(t *testing.T) {
		ctx := context.Background()
		c := newFakeInitStateCluster(ctx, t, nil)
		s, err := c.InitState(ctx, InitStateOptions{
			InternalServices: state.NewServiceSet(state.RegistryKey, state.AgentKey),
			GitServer: state.GitServerInfo{
				Address:      "https://git.example.com",
				PushUsername: "pusher",
				PushPassword: "pass",
			},
		})
		require.NoError(t, err)
		require.Equal(t, "https://git.example.com", s.GitServer.Address)
		require.False(t, s.GitServer.IsInternal())
	})

	t.Run("re-init does not wipe services not listed", func(t *testing.T) {
		ctx := context.Background()
		existing := &state.State{
			Distro: DistroIsK3d,
			GitServer: state.GitServerInfo{
				Address:      state.ZarfInClusterGitServiceURL,
				PushUsername: "zarf-git-user",
				PushPassword: "keep-me",
			},
			RegistryInfo: state.RegistryInfo{
				Address:      "127.0.0.1:31999",
				Port:         31999,
				RegistryMode: state.RegistryModeNodePort,
				PushUsername: "push-user",
				PullUsername: "pull-user",
				Secret:       "secret",
			},
		}
		c := newFakeInitStateCluster(ctx, t, existing)
		s, err := c.InitState(ctx, InitStateOptions{
			InternalServices: state.NewServiceSet(state.RegistryKey),
		})
		require.NoError(t, err)
		require.Equal(t, "keep-me", s.GitServer.PushPassword)
	})

	t.Run("re-init propagates RegistryMode when registry is not in InternalServices", func(t *testing.T) {
		ctx := context.Background()
		existing := &state.State{
			Distro: DistroIsK3d,
			RegistryInfo: state.RegistryInfo{
				Address:      "127.0.0.1:31999",
				Port:         31999,
				RegistryMode: state.RegistryModeNodePort,
				PushUsername: "push-user",
				PullUsername: "pull-user",
				Secret:       "secret",
			},
			InjectorInfo: state.InjectorInfo{Port: 31999},
		}
		c := newFakeInitStateCluster(ctx, t, existing)
		s, err := c.InitState(ctx, InitStateOptions{
			InternalServices: state.NewServiceSet(state.AgentKey),
			RegistryInfo: state.RegistryInfo{
				RegistryMode: state.RegistryModeExternal,
			},
		})
		require.NoError(t, err)
		require.Equal(t, state.RegistryModeExternal, s.RegistryInfo.RegistryMode)
		require.False(t, s.RegistryInfo.IsInternal())
		require.Equal(t, 0, s.InjectorInfo.Port, "injector port must reset when mode changes")
	})
}

func TestIgnoreExistingNamespacesForAgent(t *testing.T) {
	t.Parallel()

	t.Run("adds agent ignore label and preserves existing labels while skipping zarf namespace", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		c := &Cluster{Clientset: fake.NewClientset(
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app",
					Labels: map[string]string{
						"keep": "me",
					},
				},
			},
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "default"},
			},
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: state.ZarfNamespaceName},
			},
		)}

		require.NoError(t, c.ignoreExistingNamespacesForAgent(ctx))

		app, err := c.Clientset.CoreV1().Namespaces().Get(ctx, "app", metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "me", app.Labels["keep"])
		require.Equal(t, "ignore", app.Labels[AgentLabel])

		defaultNS, err := c.Clientset.CoreV1().Namespaces().Get(ctx, "default", metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "ignore", defaultNS.Labels[AgentLabel])

		zarfNS, err := c.Clientset.CoreV1().Namespaces().Get(ctx, state.ZarfNamespaceName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotContains(t, zarfNS.Labels, AgentLabel)
	})

	t.Run("returns list namespaces errors", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		cs := fake.NewClientset()
		cs.PrependReactor("list", "namespaces", func(_ k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("list failed")
		})
		c := &Cluster{Clientset: cs}

		require.ErrorContains(t, c.ignoreExistingNamespacesForAgent(ctx), "unable to get the Kubernetes namespaces")
	})

	t.Run("returns update namespace errors", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		cs := fake.NewClientset(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "app"},
		})
		cs.PrependReactor("update", "namespaces", func(_ k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, kerrors.NewForbidden(schema.GroupResource{Resource: "namespaces"}, "app", errors.New("denied"))
		})
		c := &Cluster{Clientset: cs}

		require.ErrorContains(t, c.ignoreExistingNamespacesForAgent(ctx), "unable to mark the namespace app as ignored by Zarf Agent")
	})
}
