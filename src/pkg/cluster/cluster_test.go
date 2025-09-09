// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/pkg/state"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
				require.Equal(t, map[string]string{AgentLabel: "ignore"}, ns.Labels)
			}
		})
	}
}
