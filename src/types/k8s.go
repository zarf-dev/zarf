// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

import (
	"fmt"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config/lang"
)

// ComponentStatus defines the deployment status of a Zarf component within a package.
type ComponentStatus string

// All the different status options for a Zarf Component
const (
	ComponentStatusSucceeded ComponentStatus = "Succeeded"
	ComponentStatusFailed    ComponentStatus = "Failed"
	ComponentStatusDeploying ComponentStatus = "Deploying"
	ComponentStatusRemoving  ComponentStatus = "Removing"
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

	ZarfInClusterGitServiceURL      = "http://zarf-gitea-http.zarf.svc.cluster.local:3000"
	ZarfInClusterArtifactServiceURL = ZarfInClusterGitServiceURL + "/api/packages/" + ZarfGitPushUser
)

// GeneratedPKI is a struct for storing generated PKI data.
type GeneratedPKI struct {
	CA   []byte `json:"ca"`
	Cert []byte `json:"cert"`
	Key  []byte `json:"key"`
}

// ZarfState is maintained as a secret in the Zarf namespace to track Zarf init data.
type ZarfState struct {
	// Indicates if Zarf was initialized while deploying its own k8s cluster
	ZarfAppliance bool `json:"zarfAppliance"`
	// K8s distribution of the cluster Zarf was deployed to
	Distro string `json:"distro"`
	// Machine architecture of the k8s node(s)
	Architecture string `json:"architecture"`
	// Default StorageClass value Zarf uses for variable templating
	StorageClass string `json:"storageClass"`
	// PKI certificate information for the agent pods Zarf manages
	AgentTLS GeneratedPKI `json:"agentTLS"`

	// Information about the repository Zarf is configured to use
	GitServer GitServerInfo `json:"gitServer"`
	// Information about the container registry Zarf is configured to use
	RegistryInfo RegistryInfo `json:"registryInfo"`
	// Information about the artifact registry Zarf is configured to use
	ArtifactServer ArtifactServerInfo `json:"artifactServer"`
}

// DeployedPackage contains information about a Zarf Package that has been deployed to a cluster
// This object is saved as the data of a k8s secret within the 'Zarf' namespace (not as part of the ZarfState secret).
type DeployedPackage struct {
	Name               string               `json:"name"`
	Data               v1alpha1.ZarfPackage `json:"data"`
	CLIVersion         string               `json:"cliVersion"`
	Generation         int                  `json:"generation"`
	DeployedComponents []DeployedComponent  `json:"deployedComponents"`
	ConnectStrings     ConnectStrings       `json:"connectStrings,omitempty"`
}

// ConnectString contains information about a connection made with Zarf connect.
type ConnectString struct {
	// Descriptive text that explains what the resource you would be connecting to is used for
	Description string `json:"description"`
	// URL path that gets appended to the k8s port-forward result
	URL string `json:"url"`
}

// ConnectStrings is a map of connect names to connection information.
type ConnectStrings map[string]ConnectString

// DeployedComponent contains information about a Zarf Package Component that has been deployed to a cluster.
type DeployedComponent struct {
	Name               string           `json:"name"`
	InstalledCharts    []InstalledChart `json:"installedCharts"`
	Status             ComponentStatus  `json:"status"`
	ObservedGeneration int              `json:"observedGeneration"`
}

// InstalledChart contains information about a Helm Chart that has been deployed to a cluster.
type InstalledChart struct {
	Namespace      string         `json:"namespace"`
	ChartName      string         `json:"chartName"`
	ConnectStrings ConnectStrings `json:"connectStrings,omitempty"`
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
