// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package state manages references to a logical zarf deployment in k8s.
package state

import (
	"context"
	"fmt"
	"slices"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/pki"
)

// Declares secrets and metadata keys and values.
// TODO(mkcp): Remove Zarf prefix, that's the project name.
// TODO(mkcp): Provide semantic doccomments for how these are used.
const (
	ZarfManagedByLabel   = "app.kubernetes.io/managed-by"
	ZarfNamespaceName    = "zarf"
	ZarfStateSecretName  = "zarf-state"
	ZarfStateDataKey     = "state"
	ZarfPackageInfoLabel = "package-deploy-info"
)

// Credential keys
// TODO(mkcp): Provide semantic doccomments for how these are used.
const (
	RegistryKey     = "registry"
	RegistryReadKey = "registry-readonly"
	GitKey          = "git"
	GitReadKey      = "git-readonly"
	ArtifactKey     = "artifact"
	AgentKey        = "agent"
)

// Values during setup of the initial zarf state
const (
	ZarfGeneratedPasswordLen               = 24
	ZarfGeneratedSecretLen                 = 48
	ZarfInClusterContainerRegistryNodePort = 31999
	ZarfRegistryPushUser                   = "zarf-push"
	ZarfRegistryPullUser                   = "zarf-pull"

	ZarfGitPushUser = "zarf-git-user"
	ZarfGitReadUser = "zarf-git-read-user"
	ZarfAgentHost   = "agent-hook.zarf.svc"

	ZarfInClusterGitServiceURL      = "http://zarf-gitea-http.zarf.svc.cluster.local:3000"
	ZarfInClusterArtifactServiceURL = ZarfInClusterGitServiceURL + "/api/packages/" + ZarfGitPushUser
)

// State is maintained as a secret in the Zarf namespace to track Zarf init data.
type State struct {
	// Indicates if Zarf was initialized while deploying its own k8s cluster
	ZarfAppliance bool `json:"zarfAppliance"`
	// K8s distribution of the cluster Zarf was deployed to
	Distro string `json:"distro"`
	// Machine architecture of the k8s node(s)
	Architecture string `json:"architecture"`
	// Default StorageClass value Zarf uses for variable templating
	StorageClass string `json:"storageClass"`
	// PKI certificate information for the agent pods Zarf manages
	AgentTLS pki.GeneratedPKI `json:"agentTLS"`

	// Information about the repository Zarf is configured to use
	GitServer GitServerInfo `json:"gitServer"`
	// Information about the container registry Zarf is configured to use
	RegistryInfo RegistryInfo `json:"registryInfo"`
	// Information about the artifact registry Zarf is configured to use
	ArtifactServer ArtifactServerInfo `json:"artifactServer"`
}

// GitServerInfo contains information Zarf uses to communicate with a git repository to push/pull repositories to.
type GitServerInfo struct {
	// Username of a user with push access to the git repository
	PushUsername string `json:"pushUsername"`
	// Password of a user with push access to the git repository
	PushPassword string `json:"pushPassword"`
	// Username of a user with pull-only access to the git repository. If not provided for an external repository then the push-user is used
	PullUsername string `json:"pullUsername"`
	// Password of a user with pull-only access to the git repository. If not provided for an external repository then the push-user is used
	PullPassword string `json:"pullPassword"`
	// URL address of the git server
	Address string `json:"address"`
}

// IsInternal returns true if the git server URL is equivalent to a git server deployed through the default init package
func (gs GitServerInfo) IsInternal() bool {
	return gs.Address == ZarfInClusterGitServiceURL
}

// FillInEmptyValues sets every necessary value that's currently empty to a reasonable default
func (gs *GitServerInfo) FillInEmptyValues() error {
	var err error
	// Set default svc url if an external repository was not provided
	if gs.Address == "" {
		gs.Address = ZarfInClusterGitServiceURL
	}

	// Generate a push-user password if not provided by init flag
	if gs.PushPassword == "" {
		if gs.PushPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
			return fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
		}
	}

	if gs.PushUsername == "" && gs.IsInternal() {
		gs.PushUsername = ZarfGitPushUser
	}

	// Set read-user information if using an internal repository, otherwise copy from the push-user
	if gs.PullUsername == "" {
		if gs.IsInternal() {
			gs.PullUsername = ZarfGitReadUser
		} else {
			gs.PullUsername = gs.PushUsername
		}
	}
	if gs.PullPassword == "" {
		if gs.IsInternal() {
			if gs.PullPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
				return fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		} else {
			gs.PullPassword = gs.PushPassword
		}
	}

	return nil
}

// ArtifactServerInfo contains information Zarf uses to communicate with a artifact registry to push/pull repositories to.
type ArtifactServerInfo struct {
	// Username of a user with push access to the artifact registry
	PushUsername string `json:"pushUsername"`
	// Password of a user with push access to the artifact registry
	PushToken string `json:"pushPassword"`
	// URL address of the artifact registry
	Address string `json:"address"`
}

// IsInternal returns true if the artifact server URL is equivalent to the artifact server deployed through the default init package
func (as ArtifactServerInfo) IsInternal() bool {
	return as.Address == ZarfInClusterArtifactServiceURL
}

// FillInEmptyValues sets every necessary value that's currently empty to a reasonable default
func (as *ArtifactServerInfo) FillInEmptyValues() {
	// Set default svc url if an external registry was not provided
	if as.Address == "" {
		as.Address = ZarfInClusterArtifactServiceURL
	}

	// Set the push username to the git push user if not specified
	if as.PushUsername == "" {
		as.PushUsername = ZarfGitPushUser
	}
}

// RegistryInfo contains information Zarf uses to communicate with a container registry to push/pull images.
type RegistryInfo struct {
	// Username of a user with push access to the registry
	PushUsername string `json:"pushUsername"`
	// Password of a user with push access to the registry
	PushPassword string `json:"pushPassword"`
	// Username of a user with pull-only access to the registry. If not provided for an external registry than the push-user is used
	PullUsername string `json:"pullUsername"`
	// Password of a user with pull-only access to the registry. If not provided for an external registry than the push-user is used
	PullPassword string `json:"pullPassword"`
	// URL address of the registry
	Address string `json:"address"`
	// Nodeport of the registry. Only needed if the registry is running inside the kubernetes cluster
	NodePort int `json:"nodePort"`
	// Secret value that the registry was seeded with
	Secret string `json:"secret"`
}

// IsInternal returns true if the registry URL is equivalent to the registry deployed through the default init package
func (ri RegistryInfo) IsInternal() bool {
	return ri.Address == fmt.Sprintf("%s:%d", helpers.IPV4Localhost, ri.NodePort)
}

// FillInEmptyValues sets every necessary value not already set to a reasonable default
func (ri *RegistryInfo) FillInEmptyValues() error {
	var err error
	// Set default NodePort if none was provided and the registry is internal
	if ri.NodePort == 0 && ri.Address == "" {
		ri.NodePort = ZarfInClusterContainerRegistryNodePort
	}

	// Set default url if an external registry was not provided
	if ri.Address == "" {
		ri.Address = fmt.Sprintf("%s:%d", helpers.IPV4Localhost, ri.NodePort)
	}

	// Generate a push-user password if not provided by init flag
	if ri.PushPassword == "" {
		if ri.PushPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
			return fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
		}
	}

	if ri.PushUsername == "" && ri.IsInternal() {
		ri.PushUsername = ZarfRegistryPushUser
	}

	// Set pull-username if not provided by init flag
	if ri.PullUsername == "" {
		if ri.IsInternal() {
			ri.PullUsername = ZarfRegistryPullUser
		} else {
			// If this is an external registry and a pull-user wasn't provided, use the same credentials as the push user
			ri.PullUsername = ri.PushUsername
		}
	}
	if ri.PullPassword == "" {
		if ri.IsInternal() {
			if ri.PullPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
				return fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		} else {
			// If this is an external registry and a pull-user wasn't provided, use the same credentials as the push user
			ri.PullPassword = ri.PushPassword
		}
	}

	if ri.Secret == "" {
		if ri.Secret, err = helpers.RandomString(ZarfGeneratedSecretLen); err != nil {
			return fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
		}
	}

	return nil
}

// Default returns a default State with default values filled in for the registry, git server, and artifact server
func Default() (*State, error) {
	state := &State{}
	err := state.GitServer.FillInEmptyValues()
	if err != nil {
		return nil, err
	}
	err = state.RegistryInfo.FillInEmptyValues()
	if err != nil {
		return nil, err
	}
	state.ArtifactServer.FillInEmptyValues()
	return state, nil
}

// MergeOptions tracks the user-defined options during cluster initialization.
// TODO(mkcp): Provide semantic doccomments for how exported fields are used.
type MergeOptions struct {
	GitServer      GitServerInfo
	RegistryInfo   RegistryInfo
	ArtifactServer ArtifactServerInfo
	Services       []string
}

// Merge merges init options for provided services into the provided state to create a new state struct
func Merge(oldState *State, opts MergeOptions) (*State, error) {
	newState := *oldState
	var err error
	if slices.Contains(opts.Services, RegistryKey) {
		// TODO: Replace use of reflections with explicit setting
		newState.RegistryInfo = helpers.MergeNonZero(newState.RegistryInfo, opts.RegistryInfo)

		// Set the new passwords if they should be autogenerated
		if newState.RegistryInfo.PushPassword == oldState.RegistryInfo.PushPassword && oldState.RegistryInfo.IsInternal() {
			if newState.RegistryInfo.PushPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
		if newState.RegistryInfo.PullPassword == oldState.RegistryInfo.PullPassword && oldState.RegistryInfo.IsInternal() {
			if newState.RegistryInfo.PullPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
	}
	if slices.Contains(opts.Services, GitKey) {
		// TODO: Replace use of reflections with explicit setting
		newState.GitServer = helpers.MergeNonZero(newState.GitServer, opts.GitServer)

		// Set the new passwords if they should be autogenerated
		if newState.GitServer.PushPassword == oldState.GitServer.PushPassword && oldState.GitServer.IsInternal() {
			if newState.GitServer.PushPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
		if newState.GitServer.PullPassword == oldState.GitServer.PullPassword && oldState.GitServer.IsInternal() {
			if newState.GitServer.PullPassword, err = helpers.RandomString(ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
	}
	if slices.Contains(opts.Services, ArtifactKey) {
		// TODO: Replace use of reflections with explicit setting
		newState.ArtifactServer = helpers.MergeNonZero(newState.ArtifactServer, opts.ArtifactServer)

		// Set an empty token if it should be autogenerated
		if newState.ArtifactServer.PushToken == oldState.ArtifactServer.PushToken && oldState.ArtifactServer.IsInternal() {
			newState.ArtifactServer.PushToken = ""
		}
	}
	if slices.Contains(opts.Services, AgentKey) {
		agentTLS, err := pki.GeneratePKI(ZarfAgentHost)
		if err != nil {
			return nil, err
		}
		newState.AgentTLS = agentTLS
	}

	return &newState, nil
}

// DebugPrint takes a State struct, sanitizes sensitive fields, and prints them.
func DebugPrint(ctx context.Context, state *State) {
	if state == nil {
		return
	}
	// this is a shallow copy, nested pointers WILL NOT be copied
	oldState := *state
	sanitized := sanitizeState(&oldState)
	logger.From(ctx).Debug("debugPrintZarfState", "state", sanitized)
}

func sanitizeState(s *State) *State {
	// Overwrite the AgentTLS information
	s.AgentTLS.CA = []byte("**sanitized**")
	s.AgentTLS.Cert = []byte("**sanitized**")
	s.AgentTLS.Key = []byte("**sanitized**")

	// Overwrite the GitServer passwords
	s.GitServer.PushPassword = "**sanitized**"
	s.GitServer.PullPassword = "**sanitized**"

	// Overwrite the RegistryInfo passwords
	s.RegistryInfo.PushPassword = "**sanitized**"
	s.RegistryInfo.PullPassword = "**sanitized**"
	s.RegistryInfo.Secret = "**sanitized**"

	// Overwrite the ArtifactServer secret
	s.ArtifactServer.PushToken = "**sanitized**"

	return s
}
