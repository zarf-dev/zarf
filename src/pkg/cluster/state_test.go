// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/defenseunicorns/pkg/helpers/v2"

	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/pki"
	"github.com/zarf-dev/zarf/src/types"
)

func TestInitZarfState(t *testing.T) {
	emptyState := types.ZarfState{}
	emptyStateData, err := json.Marshal(emptyState)
	require.NoError(t, err)

	existingState := types.ZarfState{
		Distro: DistroIsK3d,
		RegistryInfo: types.RegistryInfo{
			PushUsername:     "push-user",
			PullUsername:     "pull-user",
			Address:          "address",
			NodePort:         1,
			InternalRegistry: false,
			Secret:           "secret",
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
						Name: ZarfNamespaceName,
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
						Name: ZarfNamespaceName,
					},
				},
			},
			secrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ZarfNamespaceName,
						Name:      ZarfStateSecretName,
					},
					Data: map[string][]byte{
						ZarfStateDataKey: emptyStateData,
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
						Name: ZarfNamespaceName,
					},
				},
			},
			secrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ZarfNamespaceName,
						Name:      ZarfStateSecretName,
					},
					Data: map[string][]byte{
						ZarfStateDataKey: existingStateData,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cs := fake.NewSimpleClientset()
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
					ns, err := cs.CoreV1().Namespaces().Get(ctx, ZarfNamespaceName, metav1.GetOptions{})
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

			err := c.InitZarfState(ctx, tt.initOpts)
			if tt.expectedErr != "" {
				require.EqualError(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			state, err := cs.CoreV1().Secrets(ZarfNamespaceName).Get(ctx, ZarfStateSecretName, metav1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, map[string]string{"app.kubernetes.io/managed-by": "zarf"}, state.Labels)
			if tt.secrets != nil {
				return
			}
			zarfNs, err := cs.CoreV1().Namespaces().Get(ctx, ZarfNamespaceName, metav1.GetOptions{})
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

// TODO: Change password gen method to make testing possible.
func TestMergeZarfStateRegistry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		initRegistry     types.RegistryInfo
		oldRegistry      types.RegistryInfo
		expectedRegistry types.RegistryInfo
	}{
		{
			name: "username is unmodified",
			oldRegistry: types.RegistryInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
			},
			expectedRegistry: types.RegistryInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
			},
		},
		{
			name: "internal server auto generate",
			oldRegistry: types.RegistryInfo{
				Address:          fmt.Sprintf("%s:%d", helpers.IPV4Localhost, 1),
				NodePort:         1,
				InternalRegistry: true,
			},
			expectedRegistry: types.RegistryInfo{
				Address:          fmt.Sprintf("%s:%d", helpers.IPV4Localhost, 1),
				NodePort:         1,
				InternalRegistry: true,
			},
		},
		{
			name: "external server",
			oldRegistry: types.RegistryInfo{
				Address:          "example.com",
				InternalRegistry: false,
				PushPassword:     "push",
				PullPassword:     "pull",
			},
			expectedRegistry: types.RegistryInfo{
				Address:          "example.com",
				InternalRegistry: false,
				PushPassword:     "push",
				PullPassword:     "pull",
			},
		},
		{
			name: "init options merged",
			initRegistry: types.RegistryInfo{
				PushUsername:     "push-user",
				PullUsername:     "pull-user",
				Address:          "address",
				NodePort:         1,
				InternalRegistry: false,
				Secret:           "secret",
			},
			expectedRegistry: types.RegistryInfo{
				PushUsername:     "push-user",
				PullUsername:     "pull-user",
				Address:          "address",
				NodePort:         1,
				InternalRegistry: false,
				Secret:           "secret",
			},
		},
		{
			name: "init options not merged",
			expectedRegistry: types.RegistryInfo{
				PushUsername:     "",
				PullUsername:     "",
				Address:          "",
				NodePort:         0,
				InternalRegistry: false,
				Secret:           "",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			oldState := &types.ZarfState{
				RegistryInfo: tt.oldRegistry,
			}
			newState, err := MergeZarfState(oldState, types.ZarfInitOptions{RegistryInfo: tt.initRegistry}, []string{message.RegistryKey})
			require.NoError(t, err)
			require.Equal(t, tt.expectedRegistry.PushUsername, newState.RegistryInfo.PushUsername)
			require.Equal(t, tt.expectedRegistry.PullUsername, newState.RegistryInfo.PullUsername)
			require.Equal(t, tt.expectedRegistry.Address, newState.RegistryInfo.Address)
			require.Equal(t, tt.expectedRegistry.NodePort, newState.RegistryInfo.NodePort)
			require.Equal(t, tt.expectedRegistry.InternalRegistry, newState.RegistryInfo.InternalRegistry)
			require.Equal(t, tt.expectedRegistry.Secret, newState.RegistryInfo.Secret)
		})
	}
}

// TODO: Change password gen method to make testing possible.
func TestMergeZarfStateGit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		initGitServer     types.GitServerInfo
		oldGitServer      types.GitServerInfo
		expectedGitServer types.GitServerInfo
	}{
		{
			name: "username is unmodified",
			oldGitServer: types.GitServerInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
			},
			expectedGitServer: types.GitServerInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
			},
		},
		{
			name: "internal server auto generate",
			oldGitServer: types.GitServerInfo{
				Address:        types.ZarfInClusterGitServiceURL,
				InternalServer: true,
			},
			expectedGitServer: types.GitServerInfo{
				Address:        types.ZarfInClusterGitServiceURL,
				InternalServer: true,
			},
		},
		{
			name: "external server",
			oldGitServer: types.GitServerInfo{
				Address:        "example.com",
				InternalServer: false,
				PushPassword:   "push",
				PullPassword:   "pull",
			},
			expectedGitServer: types.GitServerInfo{
				Address:        "example.com",
				InternalServer: false,
				PushPassword:   "push",
				PullPassword:   "pull",
			},
		},
		{
			name: "init options merged",
			initGitServer: types.GitServerInfo{
				PushUsername:   "push-user",
				PullUsername:   "pull-user",
				Address:        "address",
				InternalServer: false,
			},
			expectedGitServer: types.GitServerInfo{
				PushUsername:   "push-user",
				PullUsername:   "pull-user",
				Address:        "address",
				InternalServer: false,
			},
		},
		{
			name: "empty init options not merged",
			expectedGitServer: types.GitServerInfo{
				PushUsername:   "",
				PullUsername:   "",
				Address:        "",
				InternalServer: false,
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			oldState := &types.ZarfState{
				GitServer: tt.oldGitServer,
			}
			newState, err := MergeZarfState(oldState, types.ZarfInitOptions{GitServer: tt.initGitServer}, []string{message.GitKey})
			require.NoError(t, err)
			require.Equal(t, tt.expectedGitServer.PushUsername, newState.GitServer.PushUsername)
			require.Equal(t, tt.expectedGitServer.PullUsername, newState.GitServer.PullUsername)
			require.Equal(t, tt.expectedGitServer.Address, newState.GitServer.Address)
			require.Equal(t, tt.expectedGitServer.InternalServer, newState.GitServer.InternalServer)
		})
	}
}

func TestMergeZarfStateArtifact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		initArtifactServer     types.ArtifactServerInfo
		oldArtifactServer      types.ArtifactServerInfo
		expectedArtifactServer types.ArtifactServerInfo
	}{
		{
			name: "username is unmodified",
			oldArtifactServer: types.ArtifactServerInfo{
				PushUsername: "push-user",
			},
			expectedArtifactServer: types.ArtifactServerInfo{
				PushUsername: "push-user",
			},
		},
		{
			name: "old state is internal server auto generate push token",
			oldArtifactServer: types.ArtifactServerInfo{
				PushToken:      "foobar",
				Address:        types.ZarfInClusterArtifactServiceURL,
				InternalServer: true,
			},
			expectedArtifactServer: types.ArtifactServerInfo{
				PushToken:      "",
				Address:        types.ZarfInClusterArtifactServiceURL,
				InternalServer: true,
			},
		},
		{
			name: "old state is not internal server auto generate push token but init options does not match",
			initArtifactServer: types.ArtifactServerInfo{
				PushToken: "hello world",
			},
			oldArtifactServer: types.ArtifactServerInfo{
				PushToken:      "foobar",
				Address:        types.ZarfInClusterArtifactServiceURL,
				InternalServer: false,
			},
			expectedArtifactServer: types.ArtifactServerInfo{
				PushToken:      "hello world",
				Address:        types.ZarfInClusterArtifactServiceURL,
				InternalServer: true,
			},
		},
		{
			name: "external server same push token",
			oldArtifactServer: types.ArtifactServerInfo{
				PushToken:      "foobar",
				Address:        "http://example.com",
				InternalServer: false,
			},
			expectedArtifactServer: types.ArtifactServerInfo{
				PushToken:      "foobar",
				Address:        "http://example.com",
				InternalServer: false,
			},
		},
		{
			name: "init options merged",
			initArtifactServer: types.ArtifactServerInfo{
				PushUsername:   "user",
				PushToken:      "token",
				Address:        "address",
				InternalServer: false,
			},
			expectedArtifactServer: types.ArtifactServerInfo{
				PushUsername:   "user",
				PushToken:      "token",
				Address:        "address",
				InternalServer: false,
			},
		},
		{
			name: "empty init options not merged",
			expectedArtifactServer: types.ArtifactServerInfo{
				PushUsername:   "",
				PushToken:      "",
				Address:        "",
				InternalServer: false,
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			oldState := &types.ZarfState{
				ArtifactServer: tt.oldArtifactServer,
			}
			newState, err := MergeZarfState(oldState, types.ZarfInitOptions{ArtifactServer: tt.initArtifactServer}, []string{message.ArtifactKey})
			require.NoError(t, err)
			require.Equal(t, tt.expectedArtifactServer, newState.ArtifactServer)
		})
	}
}

func TestMergeZarfStateAgent(t *testing.T) {
	t.Parallel()

	agentTLS, err := pki.GeneratePKI("example.com")
	require.NoError(t, err)
	oldState := &types.ZarfState{
		AgentTLS: agentTLS,
	}
	newState, err := MergeZarfState(oldState, types.ZarfInitOptions{}, []string{message.AgentKey})
	require.NoError(t, err)
	require.NotEqual(t, oldState.AgentTLS, newState.AgentTLS)
}
