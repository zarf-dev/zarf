package cluster

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
	"time"
)

func TestInit(t *testing.T) {
	emptyState := types.ZarfState{}
	emptyStateData, err := json.Marshal(emptyState)
	require.NoError(t, err)

	existingState := types.ZarfState{
		Distro: DistroIsK3d,
		RegistryInfo: types.RegistryInfo{
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
		initOpts    types.ZarfInitOptions
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
			initOpts: types.ZarfInitOptions{},
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
						state.ZarfStateDataKey: emptyStateData,
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
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

			err := c.Init(ctx, tt.initOpts)
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
