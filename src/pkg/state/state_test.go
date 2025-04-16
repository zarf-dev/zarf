// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package state manages references to a logical zarf deployment in k8s.
package state

import (
	"fmt"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/pki"
	"github.com/zarf-dev/zarf/src/types"
)

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
				Address:  fmt.Sprintf("%s:%d", helpers.IPV4Localhost, 1),
				NodePort: 1,
			},
			expectedRegistry: types.RegistryInfo{
				Address:  fmt.Sprintf("%s:%d", helpers.IPV4Localhost, 1),
				NodePort: 1,
			},
		},
		{
			name: "init options merged",
			oldRegistry: types.RegistryInfo{
				PushUsername: "doesn't matter",
				PullUsername: "doesn't matter",
				Address:      "doesn't matter",
				NodePort:     0,
				Secret:       "doesn't matter",
			},
			initRegistry: types.RegistryInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
				Address:      "address",
				NodePort:     1,
				Secret:       "secret",
			},
			expectedRegistry: types.RegistryInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
				Address:      "address",
				NodePort:     1,
				Secret:       "secret",
			},
		},
		{
			name: "init options not merged",
			expectedRegistry: types.RegistryInfo{
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

			oldState := &types.ZarfState{
				RegistryInfo: tt.oldRegistry,
			}
			newState, err := MergeZarfState(oldState, types.ZarfInitOptions{RegistryInfo: tt.initRegistry}, []string{message.RegistryKey})
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
func TestMergeZarfStateGit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		initGitServer     types.GitServerInfo
		oldGitServer      types.GitServerInfo
		expectedGitServer types.GitServerInfo
	}{
		{
			name: "address and usernames are unmodified",
			oldGitServer: types.GitServerInfo{
				Address:      "address",
				PushUsername: "push-user",
				PullUsername: "pull-user",
			},
			expectedGitServer: types.GitServerInfo{
				Address:      "address",
				PushUsername: "push-user",
				PullUsername: "pull-user",
			},
		},
		{
			name: "internal server auto generate",
			oldGitServer: types.GitServerInfo{
				Address: types.ZarfInClusterGitServiceURL,
			},
			expectedGitServer: types.GitServerInfo{
				Address: types.ZarfInClusterGitServiceURL,
			},
		},
		{
			name: "init options merged",
			oldGitServer: types.GitServerInfo{
				Address:      "doesn't matter",
				PushUsername: "doesn't matter",
				PullUsername: "doesn't matter",
			},
			initGitServer: types.GitServerInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
				Address:      "address",
			},
			expectedGitServer: types.GitServerInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
				Address:      "address",
			},
		},
		{
			name: "empty init options not merged",
			expectedGitServer: types.GitServerInfo{
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

			oldState := &types.ZarfState{
				GitServer: tt.oldGitServer,
			}
			newState, err := MergeZarfState(oldState, types.ZarfInitOptions{GitServer: tt.initGitServer}, []string{message.GitKey})
			require.NoError(t, err)
			require.Equal(t, tt.expectedGitServer.PushUsername, newState.GitServer.PushUsername)
			require.Equal(t, tt.expectedGitServer.PullUsername, newState.GitServer.PullUsername)
			require.Equal(t, tt.expectedGitServer.Address, newState.GitServer.Address)
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
				PushToken: "foobar",
				Address:   types.ZarfInClusterArtifactServiceURL,
			},
			expectedArtifactServer: types.ArtifactServerInfo{
				PushToken: "",
				Address:   types.ZarfInClusterArtifactServiceURL,
			},
		},
		{
			name: "old state is not internal server auto generate push token but init options does not match",
			initArtifactServer: types.ArtifactServerInfo{
				PushToken: "hello world",
			},
			oldArtifactServer: types.ArtifactServerInfo{
				PushToken: "foobar",
				Address:   types.ZarfInClusterArtifactServiceURL,
			},
			expectedArtifactServer: types.ArtifactServerInfo{
				PushToken: "hello world",
				Address:   types.ZarfInClusterArtifactServiceURL,
			},
		},
		{
			name: "init options merged",
			oldArtifactServer: types.ArtifactServerInfo{
				PushUsername: "doesn't matter",
				PushToken:    "doesn't matter",
				Address:      "doesn't matter",
			},
			initArtifactServer: types.ArtifactServerInfo{
				PushUsername: "user",
				PushToken:    "token",
				Address:      "address",
			},
			expectedArtifactServer: types.ArtifactServerInfo{
				PushUsername: "user",
				PushToken:    "token",
				Address:      "address",
			},
		},
		{
			name: "empty init options not merged",
			expectedArtifactServer: types.ArtifactServerInfo{
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
