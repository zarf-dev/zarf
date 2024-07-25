// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

import (
	"fmt"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/config/lang"
)

// WebhookStatus defines the status of a Component Webhook operating on a Zarf package secret.
type WebhookStatus string

// ComponentStatus defines the deployment status of a Zarf component within a package.
type ComponentStatus string

// DefaultWebhookWaitDuration is the default amount of time Zarf will wait for a webhook to complete.
const DefaultWebhookWaitDuration = time.Minute * 5

// All the different status options for a Zarf Component or a webhook that is running for a Zarf Component deployment.
const (
	WebhookStatusSucceeded WebhookStatus = "Succeeded"
	WebhookStatusFailed    WebhookStatus = "Failed"
	WebhookStatusRunning   WebhookStatus = "Running"
	WebhookStatusRemoving  WebhookStatus = "Removing"

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
	ZarfAppliance bool         `json:"zarfAppliance" jsonschema:"description=Indicates if Zarf was initialized while deploying its own k8s cluster"`
	Distro        string       `json:"distro" jsonschema:"description=K8s distribution of the cluster Zarf was deployed to"`
	Architecture  string       `json:"architecture" jsonschema:"description=Machine architecture of the k8s node(s)"`
	StorageClass  string       `json:"storageClass" jsonschema:"Default StorageClass value Zarf uses for variable templating"`
	AgentTLS      GeneratedPKI `json:"agentTLS" jsonschema:"PKI certificate information for the agent pods Zarf manages"`

	GitServer      GitServerInfo      `json:"gitServer" jsonschema:"description=Information about the repository Zarf is configured to use"`
	RegistryInfo   RegistryInfo       `json:"registryInfo" jsonschema:"description=Information about the container registry Zarf is configured to use"`
	ArtifactServer ArtifactServerInfo `json:"artifactServer" jsonschema:"description=Information about the artifact registry Zarf is configured to use"`
}

// DeployedPackage contains information about a Zarf Package that has been deployed to a cluster
// This object is saved as the data of a k8s secret within the 'Zarf' namespace (not as part of the ZarfState secret).
type DeployedPackage struct {
	Name               string                        `json:"name"`
	Data               ZarfPackage                   `json:"data"`
	CLIVersion         string                        `json:"cliVersion"`
	Generation         int                           `json:"generation"`
	DeployedComponents []DeployedComponent           `json:"deployedComponents"`
	ComponentWebhooks  map[string]map[string]Webhook `json:"componentWebhooks,omitempty"`
	ConnectStrings     ConnectStrings                `json:"connectStrings,omitempty"`
}

// DeployedComponent contains information about a Zarf Package Component that has been deployed to a cluster.
type DeployedComponent struct {
	Name               string           `json:"name"`
	InstalledCharts    []InstalledChart `json:"installedCharts"`
	Status             ComponentStatus  `json:"status"`
	ObservedGeneration int              `json:"observedGeneration"`
}

// Webhook contains information about a Component Webhook operating on a Zarf package secret.
type Webhook struct {
	Name                string        `json:"name"`
	WaitDurationSeconds int           `json:"waitDurationSeconds,omitempty"`
	Status              WebhookStatus `json:"status"`
	ObservedGeneration  int           `json:"observedGeneration"`
}

// InstalledChart contains information about a Helm Chart that has been deployed to a cluster.
type InstalledChart struct {
	Namespace string `json:"namespace"`
	ChartName string `json:"chartName"`
}

// GitServerInfo contains information Zarf uses to communicate with a git repository to push/pull repositories to.
type GitServerInfo struct {
	PushUsername string `json:"pushUsername" jsonschema:"description=Username of a user with push access to the git repository"`
	PushPassword string `json:"pushPassword" jsonschema:"description=Password of a user with push access to the git repository"`
	PullUsername string `json:"pullUsername" jsonschema:"description=Username of a user with pull-only access to the git repository. If not provided for an external repository then the push-user is used"`
	PullPassword string `json:"pullPassword" jsonschema:"description=Password of a user with pull-only access to the git repository. If not provided for an external repository then the push-user is used"`
	Address      string `json:"address" jsonschema:"description=URL address of the git server"`
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
	PushUsername string `json:"pushUsername" jsonschema:"description=Username of a user with push access to the artifact registry"`
	PushToken    string `json:"pushPassword" jsonschema:"description=Password of a user with push access to the artifact registry"`

	Address string `json:"address" jsonschema:"description=URL address of the artifact registry"`
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
	PushUsername string `json:"pushUsername" jsonschema:"description=Username of a user with push access to the registry"`
	PushPassword string `json:"pushPassword" jsonschema:"description=Password of a user with push access to the registry"`
	PullUsername string `json:"pullUsername" jsonschema:"description=Username of a user with pull-only access to the registry. If not provided for an external registry than the push-user is used"`
	PullPassword string `json:"pullPassword" jsonschema:"description=Password of a user with pull-only access to the registry. If not provided for an external registry than the push-user is used"`
	Address      string `json:"address" jsonschema:"description=URL address of the registry"`
	NodePort     int    `json:"nodePort" jsonschema:"description=Nodeport of the registry. Only needed if the registry is running inside the kubernetes cluster"`
	Secret       string `json:"secret" jsonschema:"description=Secret value that the registry was seeded with"`
}

// IsInternal returns true if the registry URL is equivalent to the registry deployed through the default init package
func (ri RegistryInfo) IsInternal() bool {
	return ri.Address == fmt.Sprintf("%s:%d", helpers.IPV4Localhost, ri.NodePort)
}

// FillInEmptyValues sets every necessary value not already set to a reasonable default
func (ri *RegistryInfo) FillInEmptyValues() error {
	var err error
	// Set default NodePort if none was provided
	if ri.NodePort == 0 {
		ri.NodePort = ZarfInClusterContainerRegistryNodePort
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
