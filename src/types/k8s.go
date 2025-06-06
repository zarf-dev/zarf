// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

import (
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
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
