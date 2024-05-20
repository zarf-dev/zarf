// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"fmt"
	"testing"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/pki"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
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
			name: "old state is not internal server auto generate push token",
			oldArtifactServer: types.ArtifactServerInfo{
				PushToken:      "foobar",
				Address:        types.ZarfInClusterArtifactServiceURL,
				InternalServer: false,
			},
			expectedArtifactServer: types.ArtifactServerInfo{
				PushToken:      "foobar",
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

	oldState := &types.ZarfState{
		AgentTLS: pki.GeneratePKI("example.com"),
	}
	newState, err := MergeZarfState(oldState, types.ZarfInitOptions{}, []string{message.AgentKey})
	require.NoError(t, err)
	require.NotEqual(t, oldState.AgentTLS, newState.AgentTLS)
}
