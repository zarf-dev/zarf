// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

import (
	"time"

	"github.com/defenseunicorns/zarf/src/pkg/k8s"
)

// PackageStatus defines the deployment status of a Zarf package.
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

// ZarfState is maintained as a secret in the Zarf namespace to track Zarf init data.
type ZarfState struct {
	ZarfAppliance bool             `json:"zarfAppliance" jsonschema:"description=Indicates if Zarf was initialized while deploying its own k8s cluster"`
	Distro        string           `json:"distro" jsonschema:"description=K8s distribution of the cluster Zarf was deployed to"`
	Architecture  string           `json:"architecture" jsonschema:"description=Machine architecture of the k8s node(s)"`
	StorageClass  string           `json:"storageClass" jsonschema:"Default StorageClass value Zarf uses for variable templating"`
	AgentTLS      k8s.GeneratedPKI `json:"agentTLS" jsonschema:"PKI certificate information for the agent pods Zarf manages"`

	GitServer      GitServerInfo      `json:"gitServer" jsonschema:"description=Information about the repository Zarf is configured to use"`
	RegistryInfo   RegistryInfo       `json:"registryInfo" jsonschema:"description=Information about the container registry Zarf is configured to use"`
	ArtifactServer ArtifactServerInfo `json:"artifactServer" jsonschema:"description=Information about the artifact registry Zarf is configured to use"`
	LoggingSecret  string             `json:"loggingSecret" jsonschema:"description=Secret value that the internal Grafana server was seeded with"`
}

// DeployedPackage contains information about a Zarf Package that has been deployed to a cluster
// This object is saved as the data of a k8s secret within the 'Zarf' namespace (not as part of the ZarfState secret).
type DeployedPackage struct {
	Name       string      `json:"name"`
	Data       ZarfPackage `json:"data"`
	CLIVersion string      `json:"cliVersion"`
	Generation int         `json:"generation"`

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

type Webhook struct {
	Name                string `json:"name"`
	WaitDurationSeconds int    `json:"waitDurationSeconds,omitempty"`
	Status              string `json:"status"`
	ObservedGeneration  int    `json:"observedGeneration"`
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

	Address        string `json:"address" jsonschema:"description=URL address of the git server"`
	InternalServer bool   `json:"internalServer" jsonschema:"description=Indicates if we are using a git server that Zarf is directly managing"`
}

// ArtifactServerInfo contains information Zarf uses to communicate with a artifact registry to push/pull repositories to.
type ArtifactServerInfo struct {
	PushUsername string `json:"pushUsername" jsonschema:"description=Username of a user with push access to the artifact registry"`
	PushToken    string `json:"pushPassword" jsonschema:"description=Password of a user with push access to the artifact registry"`

	Address        string `json:"address" jsonschema:"description=URL address of the artifact registry"`
	InternalServer bool   `json:"internalServer" jsonschema:"description=Indicates if we are using a artifact registry that Zarf is directly managing"`
}

// RegistryInfo contains information Zarf uses to communicate with a container registry to push/pull images.
type RegistryInfo struct {
	PushUsername string `json:"pushUsername" jsonschema:"description=Username of a user with push access to the registry"`
	PushPassword string `json:"pushPassword" jsonschema:"description=Password of a user with push access to the registry"`
	PullUsername string `json:"pullUsername" jsonschema:"description=Username of a user with pull-only access to the registry. If not provided for an external registry than the push-user is used"`
	PullPassword string `json:"pullPassword" jsonschema:"description=Password of a user with pull-only access to the registry. If not provided for an external registry than the push-user is used"`

	Address          string `json:"address" jsonschema:"description=URL address of the registry"`
	NodePort         int    `json:"nodePort" jsonschema:"description=Nodeport of the registry. Only needed if the registry is running inside the kubernetes cluster"`
	InternalRegistry bool   `json:"internalRegistry" jsonschema:"description=Indicates if we are using a registry that Zarf is directly managing"`

	Secret string `json:"secret" jsonschema:"description=Secret value that the registry was seeded with"`
}
